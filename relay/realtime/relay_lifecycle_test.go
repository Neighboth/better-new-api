package realtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// websocketPair returns the server and client ends of a real local connection.
func websocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		accepted <- conn
		<-release
	}))
	peer, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	conn := <-accepted
	t.Cleanup(func() { _ = peer.Close(); _ = conn.Close(); close(release); server.Close() })
	return conn, peer
}

func TestRelayStopsBothReadersBeforeReturning(t *testing.T) {
	for _, closingSide := range []string{"client", "upstream"} {
		t.Run(closingSide, func(t *testing.T) {
			client, clientPeer := websocketPair(t)
			target, targetPeer := websocketPair(t)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
			info := &relaycommon.RelayInfo{ClientWs: client, TargetWs: target}
			finished := make(chan struct{})
			go func() { Relay(c, info, DialectOpenAI); close(finished) }()
			peer := clientPeer
			if closingSide == "upstream" {
				peer = targetPeer
			}
			err := peer.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(time.Second))
			require.NoError(t, err)
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("relay did not stop both readers")
			}
		})
	}
}
