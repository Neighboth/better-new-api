package openai

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/realtime"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

// OpenaiRealtimeHandler keeps the realtime conversation with a channel that
// speaks the realtime protocol of OpenAI.
//
// The client of the conversation may speak the live protocol of Gemini, the
// messages are translated between them.
func OpenaiRealtimeHandler(c *gin.Context, info *relaycommon.RelayInfo) (*types.NewAPIError, *dto.RealtimeUsage) {
	return realtime.Relay(c, info, realtime.DialectOpenAI)
}
