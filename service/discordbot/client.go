package discordbot

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// localRelayMaxBody bounds how much of a relay response the bot buffers; relays
// can return large payloads (base64 images) and the bot must not depend on their
// size.
const localRelayMaxBody = 16 << 20

// postToLocalRelay posts a JSON payload to this instance's own relay so the bot
// reuses channel routing, quota accounting and request logging instead of
// bypassing the gateway.
func postToLocalRelay(path string, payload any, out any) error {
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	return doLocalRelay(http.MethodPost, path, bytes.NewReader(body), out)
}

// getFromLocalRelay reads a JSON document from this instance's own relay.
func getFromLocalRelay(path string, out any) error {
	return doLocalRelay(http.MethodGet, path, nil, out)
}

func doLocalRelay(method, path string, body io.Reader, out any) error {
	req, err := http.NewRequest(method, localRelayURL(path), body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := localRelayToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := readLimitedBody(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("relay returned HTTP %d: %s", resp.StatusCode, truncateText(string(raw), 300))
	}
	if out == nil {
		return nil
	}
	if err := common.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("cannot decode relay response: %w; body: %s", err, truncateText(string(raw), 200))
	}
	return nil
}

func localRelayURL(path string) string {
	port := os.Getenv("PORT")
	if port == "" && common.Port != nil {
		port = strconv.Itoa(*common.Port)
	}
	if port == "" {
		port = "3000"
	}
	return fmt.Sprintf("http://127.0.0.1:%s%s", port, path)
}

// localRelayToken finds a usable API key for the in-process relay, preferring
// the root account and falling back to any enabled token.
func localRelayToken() string {
	var token model.Token
	if err := model.DB.Where("user_id = 1").First(&token).Error; err == nil {
		return token.Key
	}
	if err := model.DB.Where("status = 1").First(&token).Error; err == nil {
		return token.Key
	}
	return ""
}

func readLimitedBody(reader io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, localRelayMaxBody))
}
