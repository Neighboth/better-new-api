package realtime

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Relay keeps the realtime conversation between the client and the upstream.
//
// The client and the upstream do not always speak the same protocol, the
// messages are translated between them.
func Relay(c *gin.Context, info *relaycommon.RelayInfo, upstream Dialect) (*types.NewAPIError, *dto.RealtimeUsage) {
	if info == nil || info.ClientWs == nil || info.TargetWs == nil {
		return types.NewError(fmt.Errorf("invalid websocket connection"), types.ErrorCodeBadResponse), nil
	}

	info.IsStream = true
	clientConn := info.ClientWs
	targetConn := info.TargetWs

	clientClosed := make(chan struct{})
	targetClosed := make(chan struct{})
	errChan := make(chan error, 2)

	sumUsage := &dto.RealtimeUsage{}
	turnUsage := &dto.RealtimeUsage{}
	localUsage := &dto.RealtimeUsage{}

	translator := &bridge{
		client:   DialectUnknown,
		upstream: upstream,
		model:    info.UpstreamModelName,
		state:    &sessionState{},
	}

	gopool.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in client reader: %v", r)
			}
		}()
		// The first message of the client may have been read before the channel
		// of the request was chosen.
		firstMessage := []byte{}
		if value, ok := c.Get("realtime_first_message"); ok {
			if data, ok := value.([]byte); ok {
				firstMessage = data
			}
		}
		for {
			select {
			case <-c.Done():
				return
			default:
				var message []byte
				if len(firstMessage) > 0 {
					message = firstMessage
					firstMessage = nil
				} else {
					_, data, err := clientConn.ReadMessage()
					if err != nil {
						if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
							errChan <- fmt.Errorf("error reading from client: %v", err)
						}
						close(clientClosed)
						return
					}
					message = data
				}

				// The protocol of the client is known with its first message.
				// The messages of the upstream that waited for it are sent now.
				if translator.clientDialect() == DialectUnknown {
					dialect := DetectDialect(message)
					if dialect == DialectUnknown {
						dialect = DialectOpenAI
					}
					for _, pending := range translator.setClientDialect(dialect) {
						_, clientMessages, err := translator.upstreamToClient(pending)
						if err != nil {
							errChan <- fmt.Errorf("error translating message: %v", err)
							return
						}
						for _, clientMessage := range clientMessages {
							if err := helper.WssString(c, clientConn, string(clientMessage)); err != nil {
								errChan <- fmt.Errorf("error writing to client: %v", err)
								return
							}
						}
					}
				}

				openaiEvents, upstreamMessages, err := translator.clientToUpstream(message)
				if err != nil {
					errChan <- fmt.Errorf("error translating message: %v", err)
					return
				}

				for _, event := range openaiEvents {
					realtimeEvent := &dto.RealtimeEvent{}
					if err := common.Unmarshal(event, realtimeEvent); err != nil {
						continue
					}
					if realtimeEvent.Type == dto.RealtimeEventTypeSessionUpdate &&
						realtimeEvent.Session != nil &&
						realtimeEvent.Session.Tools != nil {
						info.RealtimeTools = realtimeEvent.Session.Tools
					}
					textToken, audioToken, err := service.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
					if err != nil {
						errChan <- fmt.Errorf("error counting text token: %v", err)
						return
					}
					logger.LogInfo(c, fmt.Sprintf("type: %s, textToken: %d, audioToken: %d", realtimeEvent.Type, textToken, audioToken))
					localUsage.TotalTokens += textToken + audioToken
					localUsage.InputTokens += textToken + audioToken
					localUsage.InputTokenDetails.TextTokens += textToken
					localUsage.InputTokenDetails.AudioTokens += audioToken
				}

				for _, upstreamMessage := range upstreamMessages {
					if err := helper.WssString(c, targetConn, string(upstreamMessage)); err != nil {
						errChan <- fmt.Errorf("error writing to target: %v", err)
						return
					}
				}
			}
		}
	})

	gopool.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in target reader: %v", r)
			}
		}()
		for {
			select {
			case <-c.Done():
				return
			default:
				_, message, err := targetConn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						errChan <- fmt.Errorf("error reading from target: %v", err)
					}
					close(targetClosed)
					return
				}
				info.SetFirstResponseTime()

				// The protocol of the client is not known yet, the message waits
				// for it.
				if translator.holdMessage(message) {
					continue
				}

				openaiEvents, clientMessages, err := translator.upstreamToClient(message)
				if err != nil {
					errChan <- fmt.Errorf("error translating message: %v", err)
					return
				}

				for _, event := range openaiEvents {
					realtimeEvent := &dto.RealtimeEvent{}
					if err := common.Unmarshal(event, realtimeEvent); err != nil {
						continue
					}
					switch {
					case realtimeEvent.Type == dto.RealtimeEventTypeResponseDone &&
						realtimeEvent.Response != nil &&
						realtimeEvent.Response.Usage != nil &&
						realtimeEvent.Response.Usage.TotalTokens > 0:
						// The upstream tells the usage of the turn.
						addUsage(sumUsage, realtimeEvent.Response.Usage)
						if err := preConsumeUsage(c, info, realtimeEvent.Response.Usage, sumUsage); err != nil {
							errChan <- fmt.Errorf("error consume usage: %v", err)
							return
						}
						turnUsage = &dto.RealtimeUsage{}
					case realtimeEvent.Type == dto.RealtimeEventTypeResponseDone:
						// The turn ends without the usage of it, the usage of
						// Gemini comes with a message of its own.
						textToken, audioToken, err := service.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
						if err != nil {
							errChan <- fmt.Errorf("error counting text token: %v", err)
							return
						}
						turnUsage.TotalTokens += textToken + audioToken
						turnUsage.OutputTokens += textToken + audioToken
						turnUsage.OutputTokenDetails.TextTokens += textToken
						turnUsage.OutputTokenDetails.AudioTokens += audioToken
						if upstream == DialectOpenAI {
							addUsage(sumUsage, turnUsage)
							if err := preConsumeUsage(c, info, turnUsage, sumUsage); err != nil {
								errChan <- fmt.Errorf("error consume usage: %v", err)
								return
							}
							turnUsage = &dto.RealtimeUsage{}
						}
					case realtimeEvent.Type == dto.RealtimeEventTypeSessionUpdated,
						realtimeEvent.Type == dto.RealtimeEventTypeSessionCreated:
						// the format of the audio of the session
						if realtimeEvent.Session != nil {
							info.InputAudioFormat = common.GetStringIfEmpty(realtimeEvent.Session.InputAudioFormat, info.InputAudioFormat)
							info.OutputAudioFormat = common.GetStringIfEmpty(realtimeEvent.Session.OutputAudioFormat, info.OutputAudioFormat)
						}
					default:
						textToken, audioToken, err := service.CountTokenRealtime(info, *realtimeEvent, info.UpstreamModelName)
						if err != nil {
							errChan <- fmt.Errorf("error counting text token: %v", err)
							return
						}
						logger.LogInfo(c, fmt.Sprintf("type: %s, textToken: %d, audioToken: %d", realtimeEvent.Type, textToken, audioToken))
						turnUsage.TotalTokens += textToken + audioToken
						turnUsage.OutputTokens += textToken + audioToken
						turnUsage.OutputTokenDetails.TextTokens += textToken
						turnUsage.OutputTokenDetails.AudioTokens += audioToken
					}
				}

				for _, clientMessage := range clientMessages {
					if err := helper.WssString(c, clientConn, string(clientMessage)); err != nil {
						errChan <- fmt.Errorf("error writing to client: %v", err)
						return
					}
				}
			}
		}
	})

	select {
	case <-clientClosed:
	case <-targetClosed:
	case err := <-errChan:
		logger.LogError(c, "realtime error: "+err.Error())
	case <-c.Done():
	}

	// The end of the turn that waits for the usage of the upstream is sent now.
	if pending := translator.flushTurnEnd(); pending != nil {
		_ = helper.WssString(c, clientConn, string(pending))
	}

	if turnUsage.TotalTokens != 0 {
		addUsage(sumUsage, turnUsage)
		_ = preConsumeUsage(c, info, turnUsage, sumUsage)
	}

	if localUsage.TotalTokens != 0 {
		addUsage(sumUsage, localUsage)
		_ = preConsumeUsage(c, info, localUsage, sumUsage)
	}

	return nil, sumUsage
}

// addUsage adds the usage of a turn to the usage of the conversation.
func addUsage(total *dto.RealtimeUsage, usage *dto.RealtimeUsage) {
	if total == nil || usage == nil {
		return
	}
	total.TotalTokens += usage.TotalTokens
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.InputTokenDetails.CachedTokens += usage.InputTokenDetails.CachedTokens
	total.InputTokenDetails.TextTokens += usage.InputTokenDetails.TextTokens
	total.InputTokenDetails.AudioTokens += usage.InputTokenDetails.AudioTokens
	total.OutputTokenDetails.TextTokens += usage.OutputTokenDetails.TextTokens
	total.OutputTokenDetails.AudioTokens += usage.OutputTokenDetails.AudioTokens
}

// preConsumeUsage takes the quota of the usage of the conversation.
func preConsumeUsage(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.RealtimeUsage, totalUsage *dto.RealtimeUsage) error {
	if usage == nil || totalUsage == nil {
		return fmt.Errorf("invalid usage pointer")
	}
	return service.PreWssConsumeQuota(ctx, info, usage)
}
