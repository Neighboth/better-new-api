package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// ContextKeyRealtimeWs is the connection of a realtime conversation that
	// was opened before the channel of the request was chosen.
	ContextKeyRealtimeWs = "realtime_ws"
	// ContextKeyRealtimeFirstMessage is the first message of the client of a
	// realtime conversation.
	ContextKeyRealtimeFirstMessage = "realtime_first_message"
)

var realtimeUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// RealtimeModelFromSetup opens the connection of a realtime conversation and
// reads the model of it from the first message of the client.
//
// The clients of the live protocol of Gemini put the model into the setup
// message, the channel of the request cannot be chosen before that message.
// The message is kept, the conversation of it starts with it.
func RealtimeModelFromSetup() func(c *gin.Context) {
	return func(c *gin.Context) {
		ws, err := realtimeUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "the connection of the conversation could not be opened")
			return
		}
		c.Set(ContextKeyRealtimeWs, ws)

		ws.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, message, err := ws.ReadMessage()
		ws.SetReadDeadline(time.Time{})
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeBadResponse).ToOpenAIError())
			ws.Close()
			c.Abort()
			return
		}
		c.Set(ContextKeyRealtimeFirstMessage, message)

		// The model of the setup is used when the query does not have one.
		if c.Query("model") == "" {
			if model := modelOfSetup(message); model != "" {
				query := c.Request.URL.Query()
				query.Set("model", model)
				c.Request.URL.RawQuery = query.Encode()
			}
		}

		c.Next()

		// The channel of the request could not be chosen, the connection is
		// already open, the reason is sent with it.
		if c.IsAborted() {
			helper.WssError(c, ws, types.NewError(
				errors.New("the model of the conversation is not served"),
				types.ErrorCodeModelNotFound,
			).ToOpenAIError())
			ws.Close()
		}
	}
}

// modelOfSetup reads the model of the setup message of the live protocol of
// Gemini.
func modelOfSetup(message []byte) string {
	var payload struct {
		Setup *struct {
			Model string `json:"model"`
		} `json:"setup"`
	}
	if err := common.Unmarshal(message, &payload); err != nil {
		return ""
	}
	if payload.Setup == nil {
		return ""
	}
	return strings.TrimPrefix(payload.Setup.Model, "models/")
}
