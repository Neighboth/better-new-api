package deepseekweb

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed pow/sha3_wasm_bg.wasm
var wasmBytes []byte

var (
	wasmPathOnce sync.Once
	cachedWasmPath string
)

type PowChallenge struct {
	Algorithm  string `json:"algorithm"`
	Challenge  string `json:"challenge"`
	Salt       string `json:"salt"`
	Signature  string `json:"signature"`
	Difficulty int    `json:"difficulty"`
	ExpireAt   int64  `json:"expire_at"`
	TargetPath string `json:"target_path"`
}

type PowResponse struct {
	Algorithm  string `json:"algorithm"`
	Challenge  string `json:"challenge"`
	Salt       string `json:"salt"`
	Answer     int    `json:"answer"`
	Signature  string `json:"signature"`
	TargetPath string `json:"target_path"`
}

func getWasmFilePath() (string, error) {
	var err error
	wasmPathOnce.Do(func() {
		tmpDir := os.TempDir()
		target := filepath.Join(tmpDir, "deepseek_sha3_wasm_bg.wasm")
		if len(wasmBytes) > 0 {
			if writeErr := os.WriteFile(target, wasmBytes, 0644); writeErr != nil {
				err = writeErr
				return
			}
		}
		cachedWasmPath = target
	})
	if err != nil {
		return "", err
	}
	return cachedWasmPath, nil
}

const nodeSolverScript = `
const fs = require('fs');
const wasmPath = process.argv[2];
const challengeConfig = JSON.parse(process.argv[3]);

async function solve() {
    const wasmBuffer = fs.readFileSync(wasmPath);
    const wasmModule = await WebAssembly.compile(wasmBuffer);
    const instance = await WebAssembly.instantiate(wasmModule, {});
    const exports = instance.exports;

    const memory = exports.memory;
    const wasm_solve = exports.wasm_solve;
    const add_to_stack = exports.__wbindgen_add_to_stack_pointer;
    const malloc = exports.__wbindgen_export_0;
    const free = exports.__wbindgen_export_2;

    function encodeStr(str) {
        const bytes = Buffer.from(str, 'utf8');
        const ptr = malloc(bytes.length, 1);
        const memView = new Uint8Array(memory.buffer);
        memView.set(bytes, ptr);
        return { ptr, len: bytes.length };
    }

    const { challenge, salt, expire_at, difficulty } = challengeConfig;
    const prefix = salt + "_" + expire_at + "_";

    const stackPtr = add_to_stack(-16);
    const chal = encodeStr(challenge);
    const pref = encodeStr(prefix);

    wasm_solve(stackPtr, chal.ptr, chal.len, pref.ptr, pref.len, Number(difficulty));

    const view = new DataView(memory.buffer);
    const ret = view.getInt32(stackPtr, true);
    if (ret === 0) {
        throw new Error("No solution found");
    }
    const answer = view.getFloat64(stackPtr + 8, true);
    add_to_stack(16);

    try {
        free(chal.ptr, chal.len, 1);
        free(pref.ptr, pref.len, 1);
    } catch (_) {}

    return Math.round(answer);
}

solve().then(ans => {
    process.stdout.write(JSON.stringify({ answer: ans }));
}).catch(err => {
    process.stderr.write(String(err));
    process.exit(1);
});
`

func SolvePoWChallenge(challenge *PowChallenge) (string, error) {
	if challenge == nil {
		return "", errors.New("nil challenge")
	}
	wasmFile, err := getWasmFilePath()
	if err != nil {
		return "", fmt.Errorf("failed to prepare wasm: %w", err)
	}

	challengeJson, err := json.Marshal(challenge)
	if err != nil {
		return "", err
	}

	// Run node with inline script
	cmd := exec.Command("node", "-e", nodeSolverScript, wasmFile, string(challengeJson))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("node pow solver failed: %w", err)
	}

	var res struct {
		Answer int `json:"answer"`
	}
	if err := json.Unmarshal(output, &res); err != nil {
		return "", fmt.Errorf("invalid solver output: %w", err)
	}

	powResp := PowResponse{
		Algorithm:  challenge.Algorithm,
		Challenge:  challenge.Challenge,
		Salt:       challenge.Salt,
		Answer:     res.Answer,
		Signature:  challenge.Signature,
		TargetPath: challenge.TargetPath,
	}

	respBytes, err := json.Marshal(powResp)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(respBytes), nil
}
