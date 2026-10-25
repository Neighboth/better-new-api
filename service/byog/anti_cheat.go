package byog

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type AntiCheatManager struct{}

var GlobalAntiCheat = &AntiCheatManager{}

// Verify HWID and detect Sybil / VM / Emulator
func (ac *AntiCheatManager) ValidateHWID(hwid string, osName string, deviceType string, specs map[string]interface{}) (bool, string) {
	// 1. Check if HWID is in banned table
	if banned, banRecord := model.IsHWIDBanned(hwid); banned {
		return false, fmt.Sprintf("HWID is banned: %s (Code: %s)", banRecord.BanReason, banRecord.ViolationCode)
	}

	if !common.DebugEnabled && os.Getenv("BYOG_ALLOW_EMULATOR") != "true" {
		hwidLower := strings.ToLower(hwid)
		// 2. VM / Emulator signatures check
		vmSignatures := []string{
			"qemu", "virtualbox", "vbox", "vmware", "bluestacks", "ldplayer",
			"nox", "genymotion", "memu", "andy", "goldfish", "ranchu",
		}

		for _, sig := range vmSignatures {
			if strings.Contains(hwidLower, sig) {
				return false, fmt.Sprintf("Virtualized or emulator environment detected: %s", sig)
			}
		}

		// 3. Inspect CPU / Motherboard strings for hypervisor indicators
		if cpuModel, ok := specs["cpu_model"].(string); ok {
			cpuLower := strings.ToLower(cpuModel)
			if strings.Contains(cpuLower, "qemu") || strings.Contains(cpuLower, "kvm") || strings.Contains(cpuLower, "virtual") {
				return false, "Hypervisor or container CPU detected"
			}
		}
	}

	return true, ""
}

// Secret Onboarding Interview
func (ac *AntiCheatManager) RunOnboardingInterview(session *NodeSession, modelDef *model.ByogModel) (bool, error) {
	if modelDef.AntiCheatLogicPrompt == "" {
		return true, nil // No test configured
	}

	reqId := fmt.Sprintf("interview_%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"request_id":  reqId,
		"model_id":    modelDef.ModelId,
		"prompt":      modelDef.AntiCheatLogicPrompt,
		"temperature": modelDef.AntiCheatLogicTemp,
		"seed":        modelDef.AntiCheatLogicSeed,
		"max_tokens":  32,
		"stream":      false,
	}

	payloadBytes, _ := json.Marshal(payload)
	respChan := make(chan *TunnelFrame, 1)
	session.PendingRequests.Store(reqId, respChan)
	defer session.PendingRequests.Delete(reqId)

	err := session.WriteFrame(TunnelFrame{
		Type:    MsgTypeCanaryReq,
		Id:      reqId,
		Payload: payloadBytes,
	})
	if err != nil {
		return false, err
	}

	// Wait up to 5 seconds for onboarding response
	select {
	case frame := <-respChan:
		var resp DonePayload
		if err := json.Unmarshal(frame.Payload, &resp); err != nil {
			return false, err
		}

		// Verify expected answer
		trimmedContent := strings.TrimSpace(resp.Error)
		expected := strings.TrimSpace(modelDef.AntiCheatLogicExpected)

		if trimmedContent != expected && !strings.Contains(trimmedContent, expected) {
			ac.PenalizeNode(session, modelDef, "Onboarding logic exam failure", "#SEC-LOGIC-001", 0, 0, 0, trimmedContent)
			return false, fmt.Errorf("onboarding failed: expected '%s', got '%s'", expected, trimmedContent)
		}

		// Reward reputation
		ac.IncreaseReputation(session.NodeId, 5)
		return true, nil

	case <-time.After(5 * time.Second):
		return false, fmt.Errorf("onboarding interview timed out")
	}
}

// Canary & 64-Bit Random Seed Audit (Section 3.2)
func (ac *AntiCheatManager) ExecuteCanaryTest(session *NodeSession, modelDef *model.ByogModel) bool {
	if modelDef.AntiCheatSigPrompt == "" {
		return true
	}

	// Generate 64-bit seed
	seedBig, _ := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	seed := seedBig.Int64()
	if modelDef.AntiCheatSigSeed != 0 {
		seed = modelDef.AntiCheatSigSeed
	}

	reqId := fmt.Sprintf("canary_%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"request_id":  reqId,
		"model_id":    modelDef.ModelId,
		"prompt":      modelDef.AntiCheatSigPrompt,
		"temperature": 0.0,
		"seed":        seed,
		"max_tokens":  5,
		"stream":      false,
	}

	payloadBytes, _ := json.Marshal(payload)
	respChan := make(chan *TunnelFrame, 1)
	session.PendingRequests.Store(reqId, respChan)
	defer session.PendingRequests.Delete(reqId)

	err := session.WriteFrame(TunnelFrame{
		Type:    MsgTypeCanaryReq,
		Id:      reqId,
		Payload: payloadBytes,
	})
	if err != nil {
		return false
	}

	select {
	case frame := <-respChan:
		var resp DonePayload
		if err := json.Unmarshal(frame.Payload, &resp); err != nil {
			return false
		}

		// Anti-Proxy Barrier Check (Section 3.2): Must provide raw tokens and logprobs
		if len(resp.RawTokens) == 0 || len(resp.Logprobs) == 0 {
			ac.PenalizeNode(session, modelDef, "Anti-Proxy violation: Missing internal raw token spectrum", "#SEC-PROXY-701", 0, 0, 0, "No raw tokens provided")
			return false
		}

		// Verify Logprob within tolerance
		actualLogprob := resp.Logprobs[0]
		expectedLogprob := modelDef.AntiCheatSigLogprob
		tolerance := modelDef.AntiCheatSigTolerance
		if tolerance <= 0 {
			tolerance = 0.0020
		}

		diff := math.Abs(actualLogprob - expectedLogprob)
		if diff > tolerance {
			ac.PenalizeNode(session, modelDef, "Deterministic logprob signature mismatch", "#SEC-LOGPROB-492", expectedLogprob, actualLogprob, tolerance, fmt.Sprintf("Diff: %f exceeds tolerance %f", diff, tolerance))
			return false
		}

		// Passed canary
		ac.IncreaseReputation(session.NodeId, 2)
		return true

	case <-time.After(4 * time.Second):
		return false
	}
}

// Random Byte Spot-Check (Section 3.3)
func (ac *AntiCheatManager) RunByteSpotCheck(session *NodeSession, modelDef *model.ByogModel) bool {
	if modelDef.FileSize <= 1024 {
		return true
	}

	// Pick random offset between 1000 and FileSize - 100
	maxOffset := modelDef.FileSize - 64
	offsetBig, _ := rand.Int(rand.Reader, big.NewInt(maxOffset))
	offset := offsetBig.Int64()

	reqId := fmt.Sprintf("probe_%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"request_id": reqId,
		"model_id":   modelDef.ModelId,
		"offset":     offset,
		"length":     64,
	}

	payloadBytes, _ := json.Marshal(payload)
	respChan := make(chan *TunnelFrame, 1)
	session.PendingRequests.Store(reqId, respChan)
	defer session.PendingRequests.Delete(reqId)

	startTime := time.Now()
	err := session.WriteFrame(TunnelFrame{
		Type:    MsgTypeByteProbeReq,
		Id:      reqId,
		Payload: payloadBytes,
	})
	if err != nil {
		return false
	}

	// Must reply within 250ms (network + 20ms local disk read)
	select {
	case frame := <-respChan:
		latency := time.Since(startTime)
		if latency > 300*time.Millisecond {
			// Suspicious disk latency
			ac.DecreaseReputation(session.NodeId, 5)
			return false
		}

		var resp struct {
			DataHex string `json:"data_hex"`
		}
		if err := json.Unmarshal(frame.Payload, &resp); err != nil {
			return false
		}

		dataBytes, err := hex.DecodeString(resp.DataHex)
		if err != nil || len(dataBytes) != 64 {
			ac.PenalizeNode(session, modelDef, "Failed byte probe integrity length", "#SEC-BYTE-002", 0, 0, 0, "Invalid length")
			return false
		}

		return true

	case <-time.After(350 * time.Millisecond):
		ac.DecreaseReputation(session.NodeId, 10)
		return false
	}
}

// Physical Latency VRAM Pinning Verification (Section 3.4)
func (ac *AntiCheatManager) VerifyVramPinning(session *NodeSession, modelDef *model.ByogModel) bool {
	reqId := fmt.Sprintf("vram_pin_%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"request_id":  reqId,
		"model_id":    modelDef.ModelId,
		"prompt":      "Ping",
		"temperature": 0.0,
		"max_tokens":  1,
		"stream":      false,
	}

	payloadBytes, _ := json.Marshal(payload)
	respChan := make(chan *TunnelFrame, 1)
	session.PendingRequests.Store(reqId, respChan)
	defer session.PendingRequests.Delete(reqId)

	startTime := time.Now()
	_ = session.WriteFrame(TunnelFrame{
		Type:    MsgTypeInferenceReq,
		Id:      reqId,
		Payload: payloadBytes,
	})

	select {
	case frame := <-respChan:
		var resp DonePayload
		_ = json.Unmarshal(frame.Payload, &resp)
		ttft := int(time.Since(startTime).Milliseconds())

		// If TTFT >= 500ms, model was unloaded from VRAM!
		if ttft >= 500 || resp.TtftMs >= 500 {
			// Deduct bonus, minor reputation drop, NO BAN (as specified in 3.4)
			ac.DecreaseReputation(session.NodeId, 3)
			_ = model.DB.Model(&model.ByogNode{}).
				Where("node_id = ?", session.NodeId).
				Update("current_ttft_ms", ttft)
			return false
		}
		return true

	case <-time.After(3 * time.Second):
		return false
	}
}

// Penalize Node, Slash Balance & Ban
func (ac *AntiCheatManager) PenalizeNode(session *NodeSession, modelDef *model.ByogModel, reason string, violationCode string, expected float64, actual float64, tol float64, logs string) {
	node, err := model.GetByogNodeByNodeId(session.NodeId)
	if err != nil || node == nil {
		return
	}

	// 1. Create Ban Record
	ban := &model.ByogNodeBan{
		NodeId:          session.NodeId,
		UserId:          session.UserId,
		HWID:            node.HWID,
		BanReason:       reason,
		ViolationCode:   violationCode,
		TestId:          modelDef.ModelId,
		TestPrompt:      modelDef.AntiCheatSigPrompt,
		ExpectedLogprob: expected,
		ActualLogprob:   actual,
		Tolerance:       tol,
		DetailedLogs:    logs,
		IsActive:        true,
		BannedAt:        time.Now().Unix(),
	}
	_ = model.DB.Create(ban)

	// 2. Mark node as banned
	node.Status = "banned"
	node.ReputationScore = 0
	_ = model.SaveByogNode(node)

	// 3. Automated Slashing (Section 3.5): Reset earned credits
	_ = model.DB.Model(&model.ByogNode{}).Where("node_id = ?", session.NodeId).Update("total_earned_credits", 0)

	// 4. Close session
	GlobalHub.Unregister(session.NodeId)
	common.SysLog(fmt.Sprintf("[BYOG ANTI-CHEAT] Banned node %s: %s (%s)", session.NodeId, reason, violationCode))
}

func (ac *AntiCheatManager) IncreaseReputation(nodeId string, delta int) {
	_ = model.DB.Model(&model.ByogNode{}).
		Where("node_id = ?", nodeId).
		Update("reputation_score", gormExpr("CASE WHEN reputation_score + ? > 100 THEN 100 ELSE reputation_score + ? END", delta, delta))
}

func (ac *AntiCheatManager) DecreaseReputation(nodeId string, delta int) {
	_ = model.DB.Model(&model.ByogNode{}).
		Where("node_id = ?", nodeId).
		Update("reputation_score", gormExpr("CASE WHEN reputation_score - ? < 0 THEN 0 ELSE reputation_score - ? END", delta, delta))
}

// gormExpr builds an expression of the database.
//
// The value has to be an expression of GORM, a string would be written into the
// column as a value and the column could not be read any more.
func gormExpr(expr string, args ...interface{}) interface{} {
	return gorm.Expr(expr, args...)
}
