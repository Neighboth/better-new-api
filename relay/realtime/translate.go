package realtime

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// sessionState keeps the values of the session of the conversation.
//
// The session is sent by the client with the protocol of OpenAI, the values of
// it are needed to build the setup of Gemini.
type sessionState struct {
	instructions       string
	voice              string
	modalities         []string
	tools              []dto.RealTimeTool
	temperature        *float64
	turnDetection      map[string]any
	turnDetectionKnown bool
	inputAudioFormat   string
	outputAudioFormat  string
	// setupSent tells whether the setup of Gemini was sent to the upstream.
	// The setup must be the first message of the connection of Gemini.
	setupSent bool
	// pendingDone is the end of the turn of the upstream that waits for the
	// usage of it.
	pendingDone []byte
}

// bridge translates the messages between the client and the upstream.
//
// The protocol of OpenAI is the common language of the two sides, a message is
// translated with it when the client and the upstream do not speak the same
// protocol.
type bridge struct {
	mutex    sync.Mutex
	client   Dialect
	upstream Dialect
	model    string
	state    *sessionState
	// pending are the messages of the upstream that wait for the protocol of
	// the client.
	pending [][]byte
}

// clientDialect returns the protocol of the client.
func (b *bridge) clientDialect() Dialect {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.client
}

// setClientDialect keeps the protocol of the client and returns the messages
// of the upstream that waited for it.
func (b *bridge) setClientDialect(dialect Dialect) [][]byte {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.client != DialectUnknown {
		return nil
	}
	b.client = dialect
	pending := b.pending
	b.pending = nil
	return pending
}

// holdMessage keeps a message of the upstream until the protocol of the client
// is known.
func (b *bridge) holdMessage(message []byte) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.client != DialectUnknown {
		return false
	}
	b.pending = append(b.pending, message)
	return true
}

// clientToUpstream converts a message of the client into the messages of the
// upstream.
//
// The first value is the form of the protocol of OpenAI, it is used to count
// the tokens of the message. The second value is the message of the upstream.
func (b *bridge) clientToUpstream(message []byte) ([][]byte, [][]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.client == b.upstream {
		return [][]byte{message}, [][]byte{message}, nil
	}
	if b.client == DialectOpenAI {
		events := [][]byte{message}
		if b.upstream != DialectGemini {
			return events, events, nil
		}
		messages, err := b.openAIToGemini(message)
		if err != nil {
			return nil, nil, err
		}
		return events, messages, nil
	}
	// the client speaks the protocol of Gemini
	events, err := geminiToOpenAI(message, b.state)
	if err != nil {
		return nil, nil, err
	}
	if b.upstream == DialectGemini {
		return events, [][]byte{message}, nil
	}
	return events, events, nil
}

// upstreamToClient converts a message of the upstream into the messages of the
// client.
//
// The first value is the form of the protocol of OpenAI, it is used to count
// the tokens of the message. The second value is the message of the client.
func (b *bridge) upstreamToClient(message []byte) ([][]byte, [][]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.upstream == b.client {
		return [][]byte{message}, [][]byte{message}, nil
	}
	if b.upstream == DialectGemini {
		events, err := geminiToOpenAI(message, b.state)
		if err != nil {
			return nil, nil, err
		}
		if b.client == DialectGemini {
			return events, [][]byte{message}, nil
		}
		return events, b.queueTurnEnd(events), nil
	}
	// the upstream speaks the protocol of OpenAI
	events := [][]byte{message}
	if b.client != DialectGemini {
		return events, events, nil
	}
	messages, err := serverEventToGemini(message)
	if err != nil {
		return nil, nil, err
	}
	return events, messages, nil
}

// queueTurnEnd keeps the end of the turn of the upstream until the usage of it
// comes.
//
// The clients of OpenAI expect the usage of the turn in the event that ends
// it, the usage of Gemini comes with a message of its own.
func (b *bridge) queueTurnEnd(events [][]byte) [][]byte {
	var result [][]byte
	for _, event := range events {
		hasUsage := eventHasUsage(event)
		if b.state.pendingDone != nil {
			if hasUsage {
				// the usage of the upstream completes the end of the turn
				result = append(result, event)
				b.state.pendingDone = nil
				continue
			}
			result = append(result, b.state.pendingDone)
			b.state.pendingDone = nil
		}
		if isTurnEnd(event) && !hasUsage {
			b.state.pendingDone = event
			continue
		}
		result = append(result, event)
	}
	return result
}

// flushTurnEnd returns the end of the turn that waits for the usage of the
// upstream.
func (b *bridge) flushTurnEnd() []byte {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	pending := b.state.pendingDone
	b.state.pendingDone = nil
	return pending
}

// openAIToGemini converts an event of the protocol of OpenAI into the messages
// of the live protocol of Gemini.
func (b *bridge) openAIToGemini(message []byte) ([][]byte, error) {
	var event map[string]any
	if err := common.Unmarshal(message, &event); err != nil {
		return nil, err
	}
	kind, _ := event["type"].(string)

	var messages [][]byte
	switch kind {
	case dto.RealtimeEventTypeSessionUpdate:
		b.state.applySessionUpdate(event)
		if b.state.setupSent {
			return nil, nil
		}
	case dto.RealtimeEventInputAudioBufferAppend:
		audio, _ := event["audio"].(string)
		messages = append(messages, marshalMessage(b.state.mediaChunk(audio)))
	case "input_audio_buffer.commit", dto.RealtimeEventTypeResponseCreate:
		messages = append(messages, marshalMessage(map[string]any{
			"clientContent": map[string]any{"turnComplete": true},
		}))
	case dto.RealtimeEventTypeConversationCreate:
		messages = append(messages, b.state.itemToGemini(event)...)
	default:
		// the other events of the protocol of OpenAI have no counterpart in
		// the live protocol of Gemini
		return nil, nil
	}

	if !b.state.setupSent {
		b.state.setupSent = true
		setup := b.state.buildSetup(b.model)
		messages = append([][]byte{setup}, messages...)
	}
	return messages, nil
}

// serverEventToGemini converts an event of the upstream of the protocol of
// OpenAI into the messages of the live protocol of Gemini.
func serverEventToGemini(message []byte) ([][]byte, error) {
	var event map[string]any
	if err := common.Unmarshal(message, &event); err != nil {
		return nil, err
	}
	kind, _ := event["type"].(string)
	var messages [][]byte
	switch kind {
	case dto.RealtimeEventTypeSessionCreated, dto.RealtimeEventTypeSessionUpdated:
		messages = append(messages, marshalMessage(map[string]any{"setupComplete": map[string]any{}}))
	case dto.RealtimeEventResponseAudioDelta:
		delta, _ := event["delta"].(string)
		if delta == "" {
			return nil, nil
		}
		messages = append(messages, marshalMessage(map[string]any{
			"serverContent": map[string]any{
				"modelTurn": map[string]any{
					"parts": []any{
						map[string]any{
							"inlineData": map[string]any{
								"mimeType": "audio/pcm;rate=24000",
								"data":     delta,
							},
						},
					},
				},
			},
		}))
	case dto.RealtimeEventResponseAudioTranscriptionDelta:
		delta, _ := event["delta"].(string)
		if delta == "" {
			return nil, nil
		}
		messages = append(messages, marshalMessage(map[string]any{
			"serverContent": map[string]any{
				"modelTurn": map[string]any{
					"parts": []any{
						map[string]any{"text": delta},
					},
				},
			},
		}))
	case dto.RealtimeEventResponseFunctionCallArgumentsDone:
		callID, _ := event["call_id"].(string)
		name, _ := event["name"].(string)
		arguments, _ := event["arguments"].(string)
		args := map[string]any{}
		if arguments != "" {
			_ = common.Unmarshal([]byte(arguments), &args)
		}
		messages = append(messages, marshalMessage(map[string]any{
			"toolCall": map[string]any{
				"functionCalls": []any{
					map[string]any{"id": callID, "name": name, "args": args},
				},
			},
		}))
	case dto.RealtimeEventTypeResponseDone:
		var probe struct {
			Response *struct {
				Usage *dto.RealtimeUsage `json:"usage"`
			} `json:"response"`
		}
		if err := common.Unmarshal(message, &probe); err != nil {
			return nil, nil
		}
		if probe.Response == nil || probe.Response.Usage == nil {
			return nil, nil
		}
		messages = append(messages, marshalMessage(map[string]any{
			"usageMetadata": map[string]any{
				"promptTokenCount":   probe.Response.Usage.InputTokens,
				"responseTokenCount": probe.Response.Usage.OutputTokens,
				"totalTokenCount":    probe.Response.Usage.TotalTokens,
			},
		}))
	case dto.RealtimeEventTypeError:
		errorObject, _ := event["error"].(map[string]any)
		message, _ := errorObject["message"].(string)
		messages = append(messages, marshalMessage(map[string]any{
			"error": map[string]any{
				"message": message,
				"status":  "INTERNAL_ERROR",
			},
		}))
	}
	return messages, nil
}

// geminiToOpenAI converts a message of the live protocol of Gemini into the
// events of the realtime protocol of OpenAI.
func geminiToOpenAI(message []byte, state *sessionState) ([][]byte, error) {
	var payload geminiMessage
	if err := common.Unmarshal(message, &payload); err != nil {
		return nil, err
	}

	var events [][]byte
	switch {
	case payload.Setup != nil:
		events = append(events, setupToSessionUpdate(payload.Setup))
	case payload.RealtimeInput != nil:
		for _, chunk := range payload.RealtimeInput.MediaChunks {
			events = append(events, marshalEvent(map[string]any{
				"type":  dto.RealtimeEventInputAudioBufferAppend,
				"audio": resampleAudioToOpenAI(chunk.Data),
			}))
		}
	case payload.ClientContent != nil:
		text := ""
		for _, part := range payload.ClientContent.Parts {
			text += part.Text
		}
		if text != "" {
			events = append(events, marshalEvent(map[string]any{
				"type": dto.RealtimeEventTypeConversationCreate,
				"item": map[string]any{
					"type": "message",
					"role": "user",
					"content": []any{
						map[string]any{"type": "input_text", "text": text},
					},
				},
			}))
		}
		if payload.ClientContent.TurnComplete {
			events = append(events, marshalEvent(map[string]any{
				"type": dto.RealtimeEventTypeResponseCreate,
			}))
		}
	case payload.ToolResponse != nil:
		for _, response := range payload.ToolResponse.FunctionResponses {
			events = append(events, marshalEvent(map[string]any{
				"type": dto.RealtimeEventTypeConversationCreate,
				"item": map[string]any{
					"type":    "function_call_output",
					"call_id": response.ID,
					"output":  marshalAny(response.Response),
				},
			}))
		}
	case payload.SetupComplete != nil:
		events = append(events, marshalEvent(map[string]any{
			"type":    dto.RealtimeEventTypeSessionCreated,
			"session": state.session(),
		}))
	case payload.ServerContent != nil:
		events = append(events, serverContentToOpenAI(payload.ServerContent)...)
	case payload.ToolCall != nil:
		for _, call := range payload.ToolCall.FunctionCalls {
			events = append(events, marshalEvent(map[string]any{
				"type":      dto.RealtimeEventResponseFunctionCallArgumentsDone,
				"call_id":   call.ID,
				"name":      call.Name,
				"arguments": marshalAny(call.Args),
			}))
			events = append(events, marshalEvent(map[string]any{
				"type": dto.RealtimeEventConversationItemCreated,
				"item": map[string]any{
					"type":      "function_call",
					"call_id":   call.ID,
					"name":      call.Name,
					"arguments": marshalAny(call.Args),
				},
			}))
		}
	case payload.UsageMetadata != nil:
		events = append(events, marshalEvent(map[string]any{
			"type": dto.RealtimeEventTypeResponseDone,
			"response": map[string]any{
				"status": "completed",
				"usage":  usageOfGemini(payload.UsageMetadata),
			},
		}))
	case payload.LiveDisconnect != nil, payload.GoAway != nil:
		// the conversation of the upstream ends
	case payload.Error != nil:
		events = append(events, marshalEvent(map[string]any{
			"type": dto.RealtimeEventTypeError,
			"error": map[string]any{
				"message": payload.Error.Message,
				"type":    payload.Error.Status,
				"code":    payload.Error.Code,
			},
		}))
	}
	return events, nil
}

// serverContentToOpenAI converts the content of the server of Gemini into the
// events of the protocol of OpenAI.
func serverContentToOpenAI(content *geminiServerContent) [][]byte {
	var events [][]byte
	if content.ModelTurn != nil {
		for _, part := range content.ModelTurn.Parts {
			if part.Text != "" {
				events = append(events, marshalEvent(map[string]any{
					"type":  dto.RealtimeEventResponseAudioTranscriptionDelta,
					"delta": part.Text,
				}))
			}
			if part.InlineData != nil && part.InlineData.Data != "" {
				events = append(events, marshalEvent(map[string]any{
					"type":  dto.RealtimeEventResponseAudioDelta,
					"delta": part.InlineData.Data,
				}))
			}
		}
	}
	if content.Interrupted {
		events = append(events, marshalEvent(map[string]any{
			"type":     dto.RealtimeEventTypeResponseDone,
			"response": map[string]any{"status": "cancelled"},
		}))
	}
	if content.TurnComplete {
		events = append(events, marshalEvent(map[string]any{
			"type":     dto.RealtimeEventTypeResponseDone,
			"response": map[string]any{"status": "completed"},
		}))
	}
	return events
}

// setupToSessionUpdate converts the setup of Gemini into the update of the
// session of OpenAI.
func setupToSessionUpdate(setup *geminiSetup) []byte {
	session := &dto.RealtimeSession{
		Modalities:        []string{},
		InputAudioFormat:  "pcm16",
		OutputAudioFormat: "pcm16",
	}
	if setup.GenerationConfig != nil {
		for _, modality := range setup.GenerationConfig.ResponseModalities {
			switch modality {
			case "TEXT":
				session.Modalities = append(session.Modalities, "text")
			case "AUDIO":
				session.Modalities = append(session.Modalities, "text", "audio")
			}
		}
		if setup.GenerationConfig.Temperature != nil {
			session.Temperature = *setup.GenerationConfig.Temperature
		}
		if setup.GenerationConfig.SpeechConfig != nil &&
			setup.GenerationConfig.SpeechConfig.VoiceConfig != nil &&
			setup.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig != nil {
			session.Voice = setup.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName
		}
	}
	if len(session.Modalities) == 0 {
		session.Modalities = []string{"text", "audio"}
	}
	if setup.SystemInstruction != nil {
		for _, part := range setup.SystemInstruction.Parts {
			session.Instructions += part.Text
		}
	}
	for _, tool := range setup.Tools {
		for _, declaration := range tool.FunctionDeclarations {
			session.Tools = append(session.Tools, dto.RealTimeTool{
				Type:        "function",
				Name:        declaration.Name,
				Description: declaration.Description,
				Parameters:  declaration.Parameters,
			})
		}
	}
	if setup.RealtimeInputConfig != nil && setup.RealtimeInputConfig.AutomaticActivityDetection != nil {
		if setup.RealtimeInputConfig.AutomaticActivityDetection.Disabled {
			session.TurnDetection = nil
		} else {
			session.TurnDetection = map[string]any{"type": "server_vad"}
		}
	}
	return marshalEvent(map[string]any{
		"type":    dto.RealtimeEventTypeSessionUpdate,
		"session": session,
	})
}

// applySessionUpdate keeps the values of the session of the client.
func (s *sessionState) applySessionUpdate(event map[string]any) {
	raw, ok := event["session"].(map[string]any)
	if !ok {
		return
	}
	if value, ok := raw["instructions"].(string); ok {
		s.instructions = value
	}
	if value, ok := raw["voice"].(string); ok {
		s.voice = value
	}
	if value, ok := raw["modalities"].([]any); ok {
		s.modalities = stringSlice(value)
	}
	if value, ok := raw["input_audio_format"].(string); ok {
		s.inputAudioFormat = value
	}
	if value, ok := raw["output_audio_format"].(string); ok {
		s.outputAudioFormat = value
	}
	if value, ok := raw["temperature"].(float64); ok {
		s.temperature = &value
	}
	if value, ok := raw["turn_detection"].(map[string]any); ok {
		s.turnDetection = value
		s.turnDetectionKnown = true
	} else if _, ok := raw["turn_detection"]; ok {
		s.turnDetection = nil
		s.turnDetectionKnown = true
	}
	if value, ok := raw["tools"]; ok {
		if tools, err := decodeTools(value); err == nil {
			s.tools = tools
		}
	}
}

// session returns the values of the session for the events of the protocol of
// OpenAI.
func (s *sessionState) session() map[string]any {
	modalities := s.modalities
	if len(modalities) == 0 {
		modalities = []string{"text", "audio"}
	}
	session := map[string]any{
		"modalities":          modalities,
		"instructions":        s.instructions,
		"voice":               s.voice,
		"input_audio_format":  s.inputAudioFormat,
		"output_audio_format": s.outputAudioFormat,
		"tools":               s.tools,
	}
	if s.turnDetectionKnown {
		session["turn_detection"] = s.turnDetection
	}
	return session
}

// buildSetup builds the setup of Gemini from the session of the client.
func (s *sessionState) buildSetup(model string) []byte {
	setup := &geminiSetup{Model: "models/" + model}
	config := &geminiGenerationConfig{}
	for _, modality := range s.modalities {
		switch modality {
		case "text":
			config.ResponseModalities = append(config.ResponseModalities, "TEXT")
		case "audio":
			config.ResponseModalities = append(config.ResponseModalities, "AUDIO")
		}
	}
	if len(config.ResponseModalities) == 0 {
		config.ResponseModalities = []string{"TEXT"}
	}
	if s.voice != "" {
		config.SpeechConfig = &geminiSpeechConfig{
			VoiceConfig: &geminiVoiceConfig{
				PrebuiltVoiceConfig: &geminiPrebuiltVoiceConfig{VoiceName: s.voice},
			},
		}
	}
	if s.temperature != nil {
		config.Temperature = s.temperature
	}
	setup.GenerationConfig = config
	if s.instructions != "" {
		setup.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: s.instructions}}}
	}
	if len(s.tools) > 0 {
		declarations := make([]geminiFunctionDeclaration, 0, len(s.tools))
		for _, tool := range s.tools {
			declarations = append(declarations, geminiFunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			})
		}
		setup.Tools = []geminiTool{{FunctionDeclarations: declarations}}
	}
	detection := &geminiAutomaticActivityDetection{StartOfSpeech: true, EndOfSpeech: true}
	if s.turnDetectionKnown && s.turnDetection == nil {
		detection.Disabled = true
	}
	setup.RealtimeInputConfig = &geminiRealtimeInputConfig{
		AutomaticActivityDetection: detection,
		ActivityHandling:           "INTERRUPT",
	}
	return marshalMessage(map[string]any{"setup": setup})
}

// mediaChunk builds the piece of audio of the input of Gemini.
func (s *sessionState) mediaChunk(audio string) map[string]any {
	mime := geminiInputMime(s.inputAudioFormat)
	data := audio
	if s.inputAudioFormat == "" || s.inputAudioFormat == "pcm16" {
		data = resampleBase64(audio, 24000, 16000)
	}
	return map[string]any{
		"realtimeInput": map[string]any{
			"mediaChunks": []any{
				map[string]any{"data": data, "mimeType": mime},
			},
		},
	}
}

// itemToGemini converts the item of a conversation of the protocol of OpenAI
// into the messages of Gemini.
func (s *sessionState) itemToGemini(event map[string]any) [][]byte {
	item, ok := event["item"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := item["content"].([]any)
	if !ok {
		return nil
	}
	var parts []geminiPart
	for _, entry := range content {
		part, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := part["type"].(string)
		switch kind {
		case "input_text", "text", "output_text":
			if text, ok := part["text"].(string); ok && text != "" {
				parts = append(parts, geminiPart{Text: text})
			}
		case "input_audio", "audio":
			if data := audioOfPart(part); data != "" {
				parts = append(parts, geminiPart{InlineData: &geminiInlineData{
					MimeType: geminiInputMime(s.inputAudioFormat),
					Data:     data,
				}})
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return [][]byte{marshalMessage(map[string]any{
		"clientContent": map[string]any{"parts": parts},
	})}
}

// audioOfPart reads the audio of a part of an item of a conversation.
func audioOfPart(part map[string]any) string {
	switch value := part["audio"].(type) {
	case string:
		return value
	case map[string]any:
		if data, ok := value["data"].(string); ok {
			return data
		}
	}
	return ""
}

// usageOfGemini converts the usage of Gemini into the usage of the protocol of
// OpenAI.
func usageOfGemini(usage *geminiUsage) *dto.RealtimeUsage {
	total := usage.TotalTokenCount
	if total == 0 {
		total = usage.PromptTokenCount + usage.ResponseTokenCount
	}
	return &dto.RealtimeUsage{
		TotalTokens:  total,
		InputTokens:  usage.PromptTokenCount,
		OutputTokens: usage.ResponseTokenCount,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens:   usage.PromptTokenCount,
			CachedTokens: usage.CachedContentTokenCount,
		},
		OutputTokenDetails: dto.OutputTokenDetails{
			TextTokens: usage.ResponseTokenCount,
		},
	}
}

// eventHasUsage tells whether the event of the end of a turn carries the usage
// of it.
func eventHasUsage(event []byte) bool {
	var probe struct {
		Response *struct {
			Usage *dto.RealtimeUsage `json:"usage"`
		} `json:"response"`
	}
	if err := common.Unmarshal(event, &probe); err != nil {
		return false
	}
	return probe.Response != nil && probe.Response.Usage != nil
}

// isTurnEnd tells whether the event ends the turn of the upstream.
func isTurnEnd(event []byte) bool {
	var probe map[string]any
	if err := common.Unmarshal(event, &probe); err != nil {
		return false
	}
	kind, _ := probe["type"].(string)
	return kind == dto.RealtimeEventTypeResponseDone
}

func marshalEvent(event map[string]any) []byte {
	data, err := common.Marshal(event)
	if err != nil {
		return nil
	}
	return data
}

func marshalMessage(message any) []byte {
	data, err := common.Marshal(message)
	if err != nil {
		return nil
	}
	return data
}

func marshalAny(value any) string {
	data, err := common.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func stringSlice(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func decodeTools(value any) ([]dto.RealTimeTool, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return nil, err
	}
	var tools []dto.RealTimeTool
	if err := common.Unmarshal(data, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// geminiInputMime returns the type of the media of the input of Gemini.
func geminiInputMime(format string) string {
	switch strings.ToLower(format) {
	case "g711_ulaw", "g711_alaw":
		return "audio/pcm;rate=8000"
	default:
		return "audio/pcm;rate=16000"
	}
}

// resampleAudioToOpenAI changes the rate of the audio of Gemini for the input
// of OpenAI.
func resampleAudioToOpenAI(data string) string {
	return resampleBase64(data, 16000, 24000)
}

// resampleBase64 changes the rate of the audio of the format PCM16 that is
// coded with base64.
func resampleBase64(data string, fromRate, toRate int) string {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(raw) < 4 {
		return data
	}
	resampled := resamplePCM16(raw, fromRate, toRate)
	if len(resampled) == 0 {
		return data
	}
	return base64.StdEncoding.EncodeToString(resampled)
}

// resamplePCM16 changes the rate of the audio of the format PCM16.
func resamplePCM16(data []byte, fromRate, toRate int) []byte {
	if fromRate <= 0 || toRate <= 0 || fromRate == toRate {
		return data
	}
	samples := len(data) / 2
	if samples < 2 {
		return data
	}
	values := make([]int16, samples)
	for index := 0; index < samples; index++ {
		values[index] = int16(binaryUint16(data[index*2:]))
	}
	ratio := float64(fromRate) / float64(toRate)
	count := int(float64(samples) / ratio)
	if count <= 0 {
		return nil
	}
	result := make([]byte, count*2)
	for index := 0; index < count; index++ {
		position := float64(index) * ratio
		current := int(position)
		fraction := position - float64(current)
		if current+1 >= samples {
			putUint16(result[index*2:], uint16(values[samples-1]))
			continue
		}
		value := float64(values[current])*(1-fraction) + float64(values[current+1])*fraction
		putUint16(result[index*2:], uint16(int16(value)))
	}
	return result
}

// binaryUint16 reads the little endian number of two bytes.
func binaryUint16(data []byte) uint16 {
	return uint16(data[0]) | uint16(data[1])<<8
}

// putUint16 writes the little endian number of two bytes.
func putUint16(data []byte, value uint16) {
	data[0] = byte(value)
	data[1] = byte(value >> 8)
}

// jsonRaw is used to read the fields of a message that is not known.
type jsonRaw = json.RawMessage
