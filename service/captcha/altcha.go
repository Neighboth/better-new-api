// Package captcha holds the self-hosted human verification providers of the
// installation: the built-in image captcha and the ALTCHA proof-of-work
// challenge, which needs no third party service at all.
package captcha

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AltchaChallenge is the JSON document the ALTCHA widget expects from its
// challenge URL. The widget brute-forces `number` until
// sha256(salt + number) equals `challenge`, which proves the client spent the
// work while `signature` proves the challenge came from this server.
type AltchaChallenge struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	MaxNumber int    `json:"maxnumber"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
}

type altchaPayload struct {
	Algorithm string          `json:"algorithm"`
	Challenge string          `json:"challenge"`
	Number    json.RawMessage `json:"number"`
	Salt      string          `json:"salt"`
	Signature string          `json:"signature"`
}

const (
	altchaAlgorithm = "SHA-256"
	altchaMaxNumber = 100000
	altchaTTL       = 10 * time.Minute
)

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func altchaSignature(key, salt string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(salt))
	return hex.EncodeToString(mac.Sum(nil))
}

func altchaDigest(salt string, number int) string {
	sum := sha256.Sum256([]byte(salt + strconv.Itoa(number)))
	return hex.EncodeToString(sum[:])
}

// GenerateAltcha builds a signed challenge. The salt carries the expiry so a
// solved payload cannot be replayed forever.
func GenerateAltcha(key string, maxNumber int, ttl time.Duration) (AltchaChallenge, error) {
	if strings.TrimSpace(key) == "" {
		return AltchaChallenge{}, errors.New("altcha hmac key is not configured")
	}
	if maxNumber <= 0 {
		maxNumber = altchaMaxNumber
	}
	if ttl <= 0 {
		ttl = altchaTTL
	}

	saltRandom, err := randomHex(12)
	if err != nil {
		return AltchaChallenge{}, err
	}
	expires := time.Now().Add(ttl).Unix()
	salt := fmt.Sprintf("%s?expires=%d", saltRandom, expires)

	// The server does not need to remember the solution: any number inside the
	// range is acceptable as long as the digest matches, so the challenge is
	// computed from a random number.
	numberRandom := make([]byte, 4)
	if _, err := rand.Read(numberRandom); err != nil {
		return AltchaChallenge{}, err
	}
	number := int(numberRandom[0])<<16 | int(numberRandom[1])<<8 | int(numberRandom[2])
	number %= maxNumber

	return AltchaChallenge{
		Algorithm: altchaAlgorithm,
		Challenge: altchaDigest(salt, number),
		MaxNumber: maxNumber,
		Salt:      salt,
		Signature: altchaSignature(key, salt),
	}, nil
}

func altchaNumber(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, errors.New("missing number")
	}
	var asNumber int
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return asNumber, nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strconv.Atoi(asString)
	}
	return 0, errors.New("invalid number")
}

// VerifyAltcha validates the base64 payload produced by the widget.
func VerifyAltcha(payload, key string) bool {
	if strings.TrimSpace(payload) == "" || strings.TrimSpace(key) == "" {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		// The widget sometimes strips the padding.
		decoded, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return false
		}
	}

	var parsed altchaPayload
	if err := json.Unmarshal(decoded, &parsed); err != nil {
		return false
	}
	if parsed.Salt == "" || parsed.Challenge == "" || parsed.Signature == "" {
		return false
	}
	if !hmac.Equal([]byte(parsed.Signature), []byte(altchaSignature(key, parsed.Salt))) {
		return false
	}

	if idx := strings.Index(parsed.Salt, "?expires="); idx >= 0 {
		raw := parsed.Salt[idx+len("?expires="):]
		if amp := strings.Index(raw, "&"); amp >= 0 {
			raw = raw[:amp]
		}
		expires, convErr := strconv.ParseInt(raw, 10, 64)
		if convErr != nil || time.Now().Unix() > expires {
			return false
		}
	}

	number, err := altchaNumber(parsed.Number)
	if err != nil || number < 0 || number > altchaMaxNumber {
		return false
	}

	return hmac.Equal([]byte(parsed.Challenge), []byte(altchaDigest(parsed.Salt, number)))
}
