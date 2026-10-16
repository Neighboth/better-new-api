package deepseekweb

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeepSeekHashV1MatchesOfficialWasm pins the pure Go DeepSeekHashV1 against
// digests captured by executing DeepSeek's own sha3_wasm_bg.wasm
// (wasm_deepseek_hash_v1) under a real WebAssembly runtime. Any change to the
// sponge or to the round schedule must fail here, because a single wrong digest
// makes every relayed request fail upstream.
func TestDeepSeekHashV1MatchesOfficialWasm(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "e594808bc5b7151ac160c6d39a02e0a8e261ed588578403099e3561dc40c26b3"},
		{"single byte", "a", "910e595ccfd4af1264d04251f7273f7c28badc11e7de64c0e960419f416ed236"},
		{"abc", "abc", "f841106c601ce9be9bc38525e90d4178d47f21dd8eb9f238fc55ffaa4ca94506"},
		{"hello", "hello", "50605e468e6d6ead913d7d7ccc4687b83ded157cf0a0c5e011eefece12712fa5"},
		{"pangram", "The quick brown fox jumps over the lazy dog", "9a7f4e87d535e6fff80182224c6c4ddf5ab4042314bd07714b56ff5c55384811"},
		{"hex alphabet", "0123456789abcdef", "4d5193bce10a7fac4da83fddac0bcfd4b681ccfc78ae2cfe4a6aa4f8c674ed39"},
		{"pow prefix small nonce", "testsalt_1700000000_42", "d4a2ea58c89e40887c933484868380c6f803eaa8dc53a3b9df8e431b921a4f09"},
		{"pow prefix large nonce", "testsalt_1700000000_100000", "abea2f35796b65486e9be1b36f7878c66cab021e96faa473fdf4decd31f9ba30"},
		{"pow prefix alnum salt", "abc123salt_1700000000_12345", "74b3b7452745b70e85eb32ee7f0a9ec0381d42dd5137b695da915e104fc390e1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			digest := DeepSeekHashV1([]byte(tc.in))
			assert.Equal(t, tc.want, hex.EncodeToString(digest[:]))
		})
	}
}

// TestDeepSeekHashV1AbsorbsMultipleBlocks covers inputs longer than the 136 byte
// SHA3-256 rate, where the sponge has to permute intermediate blocks.
func TestDeepSeekHashV1AbsorbsMultipleBlocks(t *testing.T) {
	short := DeepSeekHashV1([]byte("x"))
	long := DeepSeekHashV1([]byte(string(repeatByte('x', 300))))
	longer := DeepSeekHashV1(repeatByte('x', 301))

	assert.NotEqual(t, short, long)
	assert.NotEqual(t, long, longer)

	// Same input must always produce the same digest.
	assert.Equal(t, long, DeepSeekHashV1(repeatByte('x', 300)))
}

func digestHex(in string) string {
	digest := DeepSeekHashV1([]byte(in))
	return hex.EncodeToString(digest[:])
}

func repeatByte(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// TestSolvePoWChallengeSyntheticChallenge builds a challenge the way DeepSeek
// does â€” challenge = DeepSeekHashV1(prefix + nonce) â€” and checks that the solver
// recovers the nonce and emits the documented header payload.
func TestSolvePoWChallengeSyntheticChallenge(t *testing.T) {
	for _, tc := range []struct {
		name       string
		salt       string
		expireAt   int64
		answer     int64
		difficulty int
	}{
		{"small difficulty", "testsalt", 1700000000, 42, 1000},
		{"mid difficulty", "testsalt", 1700000000, 500, 2000},
		{"large nonce", "abc123salt", 1700000000, 12345, 20000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			digest := DeepSeekHashV1([]byte(buildPowPrefix(tc.salt, tc.expireAt) + strconv.FormatInt(tc.answer, 10)))
			challenge := &PowChallenge{
				Algorithm:  "DeepSeekHashV1",
				Challenge:  hex.EncodeToString(digest[:]),
				Salt:       tc.salt,
				Signature:  "testsignature",
				Difficulty: tc.difficulty,
				ExpireAt:   tc.expireAt,
				TargetPath: "/api/v0/chat/completion",
			}

			header, err := SolvePoWChallenge(context.Background(), challenge)
			require.NoError(t, err)

			raw, err := base64.StdEncoding.DecodeString(header)
			require.NoError(t, err)

			var decoded struct {
				Algorithm  string `json:"algorithm"`
				Challenge  string `json:"challenge"`
				Salt       string `json:"salt"`
				Answer     int64  `json:"answer"`
				Signature  string `json:"signature"`
				TargetPath string `json:"target_path"`
			}
			require.NoError(t, json.Unmarshal(raw, &decoded))

			assert.Equal(t, tc.answer, decoded.Answer)
			assert.Equal(t, "DeepSeekHashV1", decoded.Algorithm)
			assert.Equal(t, challenge.Challenge, decoded.Challenge)
			assert.Equal(t, tc.salt, decoded.Salt)
			assert.Equal(t, "testsignature", decoded.Signature)
			assert.Equal(t, "/api/v0/chat/completion", decoded.TargetPath)
		})
	}
}

func TestSolvePoWChallengeRejectsBadInput(t *testing.T) {
	t.Run("nil challenge", func(t *testing.T) {
		_, err := SolvePoWChallenge(context.Background(), nil)
		require.Error(t, err)
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		_, err := SolvePoWChallenge(context.Background(), &PowChallenge{
			Algorithm:  "Md5V2",
			Challenge:  digestHex("x"),
			Salt:       "salt",
			Difficulty: 10,
			ExpireAt:   1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported pow algorithm")
	})

	t.Run("malformed challenge digest", func(t *testing.T) {
		_, err := SolvePoWChallenge(context.Background(), &PowChallenge{
			Algorithm:  "DeepSeekHashV1",
			Challenge:  "not-hex",
			Salt:       "salt",
			Difficulty: 10,
			ExpireAt:   1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed pow challenge")
	})

	t.Run("difficulty above the supported maximum", func(t *testing.T) {
		_, err := SolvePoWChallenge(context.Background(), &PowChallenge{
			Algorithm:  "DeepSeekHashV1",
			Challenge:  digestHex("x"),
			Salt:       "salt",
			Difficulty: maxPowDifficulty + 1,
			ExpireAt:   1,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds the supported maximum")
	})

	t.Run("no answer within difficulty", func(t *testing.T) {
		digest := DeepSeekHashV1([]byte("testsalt_1700000000_999999"))
		_, err := SolvePoWChallenge(context.Background(), &PowChallenge{
			Algorithm:  "DeepSeekHashV1",
			Challenge:  hex.EncodeToString(digest[:]),
			Salt:       "testsalt",
			Difficulty: 16,
			ExpireAt:   1700000000,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no pow answer found")
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		digest := DeepSeekHashV1([]byte("testsalt_1700000000_999999"))
		_, err := SolvePoWChallenge(ctx, &PowChallenge{
			Algorithm:  "DeepSeekHashV1",
			Challenge:  hex.EncodeToString(digest[:]),
			Salt:       "testsalt",
			Difficulty: maxPowDifficulty,
			ExpireAt:   1700000000,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "aborted")
	})
}

// TestFetchAndSolveLiveChallenge exercises the real DeepSeek endpoint with a real
// user token and confirms the pure Go solver answers a live challenge. It is
// skipped unless DEEPSEEK_USER_TOKEN is set, so the normal test run stays
// hermetic.
func TestFetchAndSolveLiveChallenge(t *testing.T) {
	token := deepSeekLiveToken(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := &webClient{baseURL: defaultBaseURL, token: token, http: &http.Client{Timeout: 60 * time.Second}}

	challenge := fetchLiveChallenge(ctx, t, client)
	require.Equal(t, "DeepSeekHashV1", challenge.Algorithm)
	require.Positive(t, challenge.Difficulty)

	start := time.Now()
	header, err := SolvePoWChallenge(ctx, challenge)
	require.NoError(t, err)
	t.Logf("solved live challenge (difficulty %d) in %s", challenge.Difficulty, time.Since(start))

	raw, err := base64.StdEncoding.DecodeString(header)
	require.NoError(t, err)
	var decoded struct {
		Answer int64 `json:"answer"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.GreaterOrEqual(t, decoded.Answer, int64(0))
	assert.Less(t, decoded.Answer, int64(challenge.Difficulty))
}

// deepSeekLiveToken returns the live user token or skips the test.
func deepSeekLiveToken(t *testing.T) string {
	t.Helper()
	token := os.Getenv("DEEPSEEK_USER_TOKEN")
	if token == "" {
		t.Skip("DEEPSEEK_USER_TOKEN is not set; skipping live DeepSeek Web round trip")
	}
	return token
}

func fetchLiveChallenge(ctx context.Context, t *testing.T, client *webClient) *PowChallenge {
	t.Helper()

	var out struct {
		Challenge PowChallenge `json:"challenge"`
	}
	require.NoError(t, client.doJSON(ctx, http.MethodPost, challengePath,
		map[string]string{"target_path": completionPath}, &out))
	require.NotEmpty(t, out.Challenge.Challenge)
	return &out.Challenge
}
