package service

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// FineTuningDatasetValidationResult summarizes strict OpenAI format verification.
type FineTuningDatasetValidationResult struct {
	Valid       bool   `json:"valid"`
	Format      string `json:"format"`
	SampleCount int64  `json:"sample_count"`
	HasImages   bool   `json:"has_images"`
}

var validOpenAIMessageRoles = map[string]bool{
	"system":    true,
	"developer": true,
	"user":      true,
	"assistant": true,
	"tool":      true,
	"function":  true,
}

var allowedOpenAIChatTopKeys = map[string]bool{
	"messages":            true,
	"tools":               true,
	"parallel_tool_calls": true,
	"weight":              true,
	"metadata":            true,
}

var allowedOpenAIDPOTopKeys = map[string]bool{
	"input":                true,
	"preferred_output":     true,
	"non_preferred_output": true,
	"metadata":             true,
}

func validateOpenAIMessageObject(msgRaw any, lineNum int, msgIdx int) (role string, hasNonEmptyContent bool, hasImages bool, err error) {
	msg, ok := msgRaw.(map[string]any)
	if !ok {
		return "", false, false, fmt.Errorf("line %d, message #%d: each entry in \"messages\" must be a JSON object", lineNum, msgIdx+1)
	}
	rawRole, _ := msg["role"].(string)
	role = strings.ToLower(strings.TrimSpace(rawRole))
	if !validOpenAIMessageRoles[role] {
		return "", false, false, fmt.Errorf("line %d, message #%d: invalid role %q (OpenAI format allows only system, developer, user, assistant, tool, function)", lineNum, msgIdx+1, rawRole)
	}

	contentVal, hasContentKey := msg["content"]
	_, hasToolCalls := msg["tool_calls"]
	_, hasFuncCall := msg["function_call"]

	if !hasContentKey || contentVal == nil {
		if role == "assistant" && (hasToolCalls || hasFuncCall) {
			return role, true, false, nil
		}
		return "", false, false, fmt.Errorf("line %d, message #%d (%s): missing \"content\" field", lineNum, msgIdx+1, role)
	}

	switch cv := contentVal.(type) {
	case string:
		hasNonEmptyContent = strings.TrimSpace(cv) != ""
		if !hasNonEmptyContent && role == "assistant" && (hasToolCalls || hasFuncCall) {
			hasNonEmptyContent = true
		}
		return role, hasNonEmptyContent, false, nil
	case []any:
		if len(cv) == 0 {
			return "", false, false, fmt.Errorf("line %d, message #%d (%s): \"content\" array cannot be empty", lineNum, msgIdx+1, role)
		}
		for pIdx, partRaw := range cv {
			part, ok := partRaw.(map[string]any)
			if !ok {
				return "", false, false, fmt.Errorf("line %d, message #%d, part #%d: content part must be an object", lineNum, msgIdx+1, pIdx+1)
			}
			pType, _ := part["type"].(string)
			switch strings.TrimSpace(pType) {
			case "text":
				txt, ok := part["text"].(string)
				if !ok {
					return "", false, false, fmt.Errorf("line %d, message #%d, part #%d: text part must contain string \"text\"", lineNum, msgIdx+1, pIdx+1)
				}
				if strings.TrimSpace(txt) != "" {
					hasNonEmptyContent = true
				}
			case "image_url":
				imgObj, ok := part["image_url"].(map[string]any)
				if !ok {
					return "", false, false, fmt.Errorf("line %d, message #%d, part #%d: image_url part must contain an object {\"url\": \"...\"}", lineNum, msgIdx+1, pIdx+1)
				}
				u, _ := imgObj["url"].(string)
				if strings.TrimSpace(u) == "" {
					return "", false, false, fmt.Errorf("line %d, message #%d, part #%d: image_url.url cannot be empty", lineNum, msgIdx+1, pIdx+1)
				}
				hasImages = true
				hasNonEmptyContent = true
			case "input_audio":
				hasNonEmptyContent = true
			default:
				return "", false, false, fmt.Errorf("line %d, message #%d, part #%d: unsupported OpenAI content part type %q (expected \"text\" or \"image_url\")", lineNum, msgIdx+1, pIdx+1, pType)
			}
		}
		return role, hasNonEmptyContent, hasImages, nil
	default:
		return "", false, false, fmt.Errorf("line %d, message #%d (%s): \"content\" must be a string or OpenAI content-part array", lineNum, msgIdx+1, role)
	}
}

func validateOpenAIMessagesArray(arr []any, lineNum int, requireAssistant bool) (hasImages bool, err error) {
	if len(arr) == 0 {
		return false, fmt.Errorf("line %d: \"messages\" array cannot be empty", lineNum)
	}
	if requireAssistant && len(arr) < 2 {
		return false, fmt.Errorf("line %d: \"messages\" must contain at least 2 messages (e.g. user + assistant)", lineNum)
	}
	sawAssistant := false
	sawNonAssistant := false
	for i, item := range arr {
		role, nonEmpty, img, mErr := validateOpenAIMessageObject(item, lineNum, i)
		if mErr != nil {
			return false, mErr
		}
		if img {
			hasImages = true
		}
		if role == "assistant" {
			if nonEmpty {
				sawAssistant = true
			}
		} else {
			sawNonAssistant = true
		}
	}
	if requireAssistant {
		if !sawAssistant {
			return false, fmt.Errorf("line %d: OpenAI fine-tuning example must contain at least one assistant message with non-empty content", lineNum)
		}
		if !sawNonAssistant {
			return false, fmt.Errorf("line %d: OpenAI fine-tuning example must contain at least one user or system message alongside the assistant reply", lineNum)
		}
	}
	return hasImages, nil
}

// ValidateOpenAIDatasetContent strictly verifies that an uploaded training
// dataset conforms to OpenAI's official fine-tuning format (JSONL chat messages
// or OpenAI DPO JSONL for text/vision, and verified ZIP archives for image/audio).
func ValidateOpenAIDatasetContent(taskType string, method string, filename string, reader io.Reader) (*FineTuningDatasetValidationResult, error) {
	task := strings.ToLower(strings.TrimSpace(taskType))
	if task == "" {
		task = "text"
	}
	meth := strings.ToLower(strings.TrimSpace(method))
	if meth == "" {
		meth = "lora"
	}
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))

	// For Text/LLM and Vision/Multimodal: STRICTLY OpenAI JSONL format only!
	if task == "text" || task == "vision" {
		if ext != ".jsonl" {
			return nil, fmt.Errorf("only OpenAI JSONL (.jsonl) datasets are accepted (got %q). Each line must be a valid JSON object in OpenAI fine-tuning format", ext)
		}
		br := bufio.NewReaderSize(reader, 256*1024)
		peek, _ := br.Peek(512)
		if len(peek) == 0 {
			return nil, fmt.Errorf("dataset file %q is empty", filepath.Base(filename))
		}
		if err := ValidateFineTuningDatasetFile(task, filename, peek); err != nil {
			return nil, err
		}

		// Strip UTF-8 BOM if present
		if bytes.HasPrefix(peek, []byte{0xEF, 0xBB, 0xBF}) {
			_, _ = br.Discard(3)
		}

		scanner := bufio.NewScanner(br)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

		var (
			lineNum   int
			samples   int64
			hasImages bool
		)
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var obj map[string]any
			if err := common.Unmarshal([]byte(line), &obj); err != nil || obj == nil {
				return nil, fmt.Errorf("line %d: invalid JSON object (every non-empty line in an OpenAI .jsonl dataset must be a single JSON object)", lineNum)
			}

			if meth == "dpo" {
				for k := range obj {
					if !allowedOpenAIDPOTopKeys[k] {
						return nil, fmt.Errorf("line %d: unexpected key %q — OpenAI DPO format requires {\"input\": {\"messages\": [...]}, \"preferred_output\": [...], \"non_preferred_output\": [...]}", lineNum, k)
					}
				}
				inp, ok := obj["input"].(map[string]any)
				if !ok {
					return nil, fmt.Errorf("line %d: OpenAI DPO format requires an \"input\" object with \"messages\"", lineNum)
				}
				inpMsgs, ok := inp["messages"].([]any)
				if !ok {
					return nil, fmt.Errorf("line %d: OpenAI DPO \"input.messages\" must be a non-empty array", lineNum)
				}
				img, err := validateOpenAIMessagesArray(inpMsgs, lineNum, false)
				if err != nil {
					return nil, err
				}
				if img {
					hasImages = true
				}
				for _, outKey := range []string{"preferred_output", "non_preferred_output"} {
					outArr, ok := obj[outKey].([]any)
					if !ok || len(outArr) == 0 {
						return nil, fmt.Errorf("line %d: OpenAI DPO format requires a non-empty %q array with an assistant message", lineNum, outKey)
					}
					role, nonEmpty, imgOut, mErr := validateOpenAIMessageObject(outArr[0], lineNum, 0)
					if mErr != nil {
						return nil, mErr
					}
					if role != "assistant" || !nonEmpty {
						return nil, fmt.Errorf("line %d: %q[0] must be a non-empty assistant message", lineNum, outKey)
					}
					if imgOut {
						hasImages = true
					}
				}
			} else {
				for k := range obj {
					if !allowedOpenAIChatTopKeys[k] {
						return nil, fmt.Errorf("line %d: unsupported key %q — dataset must strictly follow OpenAI chat format {\"messages\": [{\"role\": \"user\", \"content\": \"...\"}, {\"role\": \"assistant\", \"content\": \"...\"}]}", lineNum, k)
					}
				}
				msgsRaw, ok := obj["messages"]
				if !ok {
					return nil, fmt.Errorf("line %d: missing required \"messages\" array (dataset must be in OpenAI fine-tuning format)", lineNum)
				}
				msgsArr, ok := msgsRaw.([]any)
				if !ok {
					return nil, fmt.Errorf("line %d: \"messages\" must be a JSON array", lineNum)
				}
				img, err := validateOpenAIMessagesArray(msgsArr, lineNum, true)
				if err != nil {
					return nil, err
				}
				if img {
					hasImages = true
				}
			}
			samples++
		}
		if scanErr := scanner.Err(); scanErr != nil {
			return nil, fmt.Errorf("failed reading dataset file: %v", scanErr)
		}
		if samples == 0 {
			return nil, fmt.Errorf("dataset file %q contains no OpenAI fine-tuning examples", filepath.Base(filename))
		}
		if task == "text" && hasImages {
			return nil, fmt.Errorf("dataset contains image_url multimodal parts; select Vision / Multimodal task type and a vision-capable model")
		}
		return &FineTuningDatasetValidationResult{
			Valid:       true,
			Format:      "openai_jsonl",
			SampleCount: samples,
			HasImages:   hasImages,
		}, nil
	}

	// Image / Audio: verify extension + leading header
	head := make([]byte, 512)
	n, _ := io.ReadFull(reader, head)
	if n == 0 {
		return nil, fmt.Errorf("dataset file %q is empty", filepath.Base(filename))
	}
	if err := ValidateFineTuningDatasetFile(task, filename, head[:n]); err != nil {
		return nil, err
	}
	return &FineTuningDatasetValidationResult{
		Valid:  true,
		Format: strings.TrimPrefix(ext, "."),
	}, nil
}

// ValidateOpenAIDatasetFile validates a dataset file already written to disk.
func ValidateOpenAIDatasetFile(taskType, method, path, origFilename string) (*FineTuningDatasetValidationResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open dataset file: %w", err)
	}
	defer f.Close()
	name := origFilename
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(path)
	}
	return ValidateOpenAIDatasetContent(taskType, method, name, f)
}

// FineTuningModelCompatResult describes whether the selected base model can
// actually be fine-tuned with our OpenAI-format dataset & chosen task/method.
type FineTuningModelCompatResult struct {
	Compatible        bool     `json:"compatible"`
	Checked           bool     `json:"checked"`
	ModelID           string   `json:"model_id"`
	Architecture      string   `json:"architecture,omitempty"`
	ModelType         string   `json:"model_type,omitempty"`
	PipelineTag       string   `json:"pipeline_tag,omitempty"`
	QuantMethod       string   `json:"quant_method,omitempty"`
	TotalWeightBytes  int64    `json:"total_weight_bytes,omitempty"`
	ParameterBillions float64  `json:"parameter_billions,omitempty"`
	SupportsVision    bool     `json:"supports_vision,omitempty"`
	Reason            string   `json:"reason,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

type cachedModelCompatEntry struct {
	res FineTuningModelCompatResult
	exp time.Time
}

var (
	ftModelCompatCacheMu sync.Mutex
	ftModelCompatCache   = map[string]cachedModelCompatEntry{}
)

func splitHFRepoRef(ref string) (repoID string, subfile string) {
	s := strings.TrimSpace(ref)
	if s == "" || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s, ""
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) >= 3 {
		return parts[0] + "/" + parts[1], strings.Join(parts[2:], "/")
	}
	return s, ""
}

func isVisionArchitecture(arch, modelType, pipelineTag string) bool {
	lowArch := strings.ToLower(arch)
	lowType := strings.ToLower(modelType)
	lowTag := strings.ToLower(pipelineTag)
	if lowTag == "image-text-to-text" || lowTag == "visual-question-answering" || lowTag == "multimodal" {
		return true
	}
	visionMarkers := []string{
		"vl", "vision", "llava", "qwen2_vl", "qwen2vl", "qwen2_5_vl", "pixtral",
		"idefics", "paligemma", "internvl", "minicpmv", "mllama", "gemma3",
		"blip", "kosmos", "fuyu", "chameleon", "molmo", "phi3v", "smolvlm",
	}
	for _, m := range visionMarkers {
		if strings.Contains(lowArch, m) || strings.Contains(lowType, m) {
			return true
		}
	}
	return false
}

func isCausalLMArchitecture(arch, modelType, pipelineTag string) bool {
	lowArch := strings.ToLower(arch)
	lowTag := strings.ToLower(pipelineTag)
	if strings.HasSuffix(lowArch, "forcausallm") ||
		strings.HasSuffix(lowArch, "lmheadmodel") ||
		strings.Contains(lowArch, "causal") ||
		lowArch == "gpt2lmheadmodel" ||
		lowArch == "rwforcaulslm" ||
		lowArch == "chatglmmodel" ||
		lowArch == "chatglmforconditionalgeneration" {
		return true
	}
	if lowTag == "text-generation" || lowTag == "conversational" {
		// Make sure it isn't explicitly an encoder/diffusion/audio architecture
		if !isNonTextArchitecture(arch, modelType, "") {
			return true
		}
	}
	return false
}

func isNonTextArchitecture(arch, modelType, pipelineTag string) bool {
	lowArch := strings.ToLower(arch)
	lowType := strings.ToLower(modelType)
	lowTag := strings.ToLower(pipelineTag)
	badTags := []string{
		"feature-extraction", "sentence-similarity", "fill-mask", "token-classification",
		"text-classification", "zero-shot-classification", "text-to-image", "image-to-image",
		"text-to-video", "image-classification", "object-detection", "image-segmentation",
		"automatic-speech-recognition", "text-to-speech", "text-to-audio", "audio-classification",
	}
	for _, bt := range badTags {
		if lowTag == bt {
			return true
		}
	}
	badArchMarkers := []string{
		"forsequenceclassification", "fortokenclassification", "formaskedlm",
		"forquestionanswering", "forimageclassification", "forobjectdetection",
		"clipmodel", "siglipmodel", "bertmodel", "robertamodel", "xlmrobertamodel",
		"mpnetmodel", "nomicbertmodel", "whisperforconditionalgeneration",
		"wav2vec2", "hubert", "barkmodel", "stablediffusion", "fluxpipeline",
		"unet2d", "autoencoderkl", "vitmodel", "swinmodel", "resnet",
	}
	for _, bm := range badArchMarkers {
		if strings.Contains(lowArch, bm) || strings.Contains(lowType, bm) {
			return true
		}
	}
	return false
}

// CheckFineTuningModelCompatibility inspects a Hugging Face model (or local
// uploaded archive/file) and verifies it is genuinely compatible with
// fine-tuning using our OpenAI-format dataset for the selected task & method.
func CheckFineTuningModelCompatibility(
	ctx context.Context,
	baseModel string,
	modelSource string,
	localFilePath string,
	taskType string,
	method string,
	datasetHasImages bool,
	hfToken string,
) FineTuningModelCompatResult {
	task := strings.ToLower(strings.TrimSpace(taskType))
	if task == "" {
		task = "text"
	}
	meth := strings.ToLower(strings.TrimSpace(method))
	if meth == "" {
		meth = "lora"
	}
	trimmed := strings.TrimSpace(baseModel)
	res := FineTuningModelCompatResult{
		ModelID: trimmed,
	}
	if trimmed == "" && localFilePath == "" {
		res.Reason = "Base model is required."
		return res
	}

	lowName := strings.ToLower(trimmed)
	for _, badExt := range []string{".gguf", ".ggml", ".onnx", ".exl2", ".mlx"} {
		if strings.HasSuffix(lowName, badExt) || (localFilePath != "" && strings.HasSuffix(strings.ToLower(localFilePath), badExt)) {
			res.Checked = true
			res.Compatible = false
			res.Reason = fmt.Sprintf("Format %s is inference-only and cannot be fine-tuned with PyTorch/PEFT. Choose a Hugging Face model repository or trainable .safetensors/.zip archive.", strings.ToUpper(strings.TrimPrefix(badExt, ".")))
			return res
		}
	}

	// Local uploaded model compatibility inspection
	if modelSource == "upload" || localFilePath != "" {
		res.Checked = true
		target := localFilePath
		if target == "" {
			res.Compatible = false
			res.Reason = "Uploaded base model file was not found."
			return res
		}
		if fi, sErr := os.Stat(target); sErr == nil {
			res.TotalWeightBytes = fi.Size()
			res.ParameterBillions = ParseModelBillions(trimmed, "", fi.Size())
		}
		lowPath := strings.ToLower(target)
		if task == "image" {
			if strings.HasSuffix(lowPath, ".safetensors") || strings.HasSuffix(lowPath, ".ckpt") || strings.HasSuffix(lowPath, ".zip") {
				res.Compatible = true
				return res
			}
			res.Compatible = false
			res.Reason = "Image / Diffusion fine-tuning requires a .safetensors/.ckpt checkpoint or Diffusers .zip archive."
			return res
		}
		// Text / Vision / Audio local upload: single bare .safetensors lacks config.json & tokenizer
		if !strings.HasSuffix(lowPath, ".zip") {
			res.Compatible = false
			res.Reason = "A standalone weight file without config.json and tokenizer files cannot be fine-tuned with OpenAI chat datasets. Upload a .zip archive containing the model directory (config.json, tokenizer, .safetensors) or select a Hugging Face repository."
			return res
		}
		return inspectLocalModelZipCompat(target, task, meth, datasetHasImages, res)
	}

	// Direct URL: allow only obvious non-inference-only weights (cannot probe arbitrary private URLs reliably)
	if modelSource == "url" || strings.HasPrefix(lowName, "http://") || strings.HasPrefix(lowName, "https://") {
		res.Checked = true
		if task == "text" && strings.HasSuffix(lowName, ".safetensors") {
			res.Compatible = false
			res.Reason = "A direct single .safetensors URL lacks config.json and tokenizer files needed for Text / LLM OpenAI fine-tuning. Provide a Hugging Face repo ID (e.g. Qwen/Qwen2.5-7B-Instruct) or a .zip model package."
			return res
		}
		res.Compatible = true
		return res
	}

	// Hugging Face repository live metadata + config.json verification
	repoID, _ := splitHFRepoRef(trimmed)
	if !strings.Contains(repoID, "/") || strings.Count(repoID, "/") != 1 {
		res.Checked = true
		res.Compatible = false
		res.Reason = fmt.Sprintf("Invalid Hugging Face repository ID %q. Use the exact format \"owner/model-name\" (for example, HuggingFaceTB/SmolLM2-135M).", trimmed)
		return res
	}
	res.ModelID = repoID

	cacheKey := fmt.Sprintf("%s|%s|%s|%v", strings.ToLower(repoID), task, meth, datasetHasImages)
	ftModelCompatCacheMu.Lock()
	if entry, ok := ftModelCompatCache[cacheKey]; ok && time.Now().Before(entry.exp) {
		ftModelCompatCacheMu.Unlock()
		return entry.res
	}
	ftModelCompatCacheMu.Unlock()

	checkedRes := fetchAndCheckHFModelCompat(ctx, repoID, task, meth, datasetHasImages, hfToken)
	if checkedRes.Checked {
		ftModelCompatCacheMu.Lock()
		if len(ftModelCompatCache) > 256 {
			ftModelCompatCache = map[string]cachedModelCompatEntry{}
		}
		ftModelCompatCache[cacheKey] = cachedModelCompatEntry{
			res: checkedRes,
			exp: time.Now().Add(5 * time.Minute),
		}
		ftModelCompatCacheMu.Unlock()
	}
	return checkedRes
}

func inspectLocalModelZipCompat(zipPath, task, method string, datasetHasImages bool, res FineTuningModelCompatResult) FineTuningModelCompatResult {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		res.Compatible = false
		res.Reason = "Uploaded model .zip archive is corrupted or unreadable."
		return res
	}
	defer zr.Close()

	var (
		hasConfig    bool
		hasTokenizer bool
		hasWeights   bool
		configBytes  []byte
	)
	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		switch {
		case base == "config.json" && !hasConfig:
			hasConfig = true
			if rc, oErr := f.Open(); oErr == nil {
				configBytes, _ = io.ReadAll(io.LimitReader(rc, 512*1024))
				_ = rc.Close()
			}
		case base == "tokenizer.json" || base == "tokenizer_config.json" || base == "tokenizer.model" || base == "vocab.json":
			hasTokenizer = true
		case strings.HasSuffix(base, ".safetensors") || strings.HasSuffix(base, ".bin") || strings.HasSuffix(base, ".pt"):
			hasWeights = true
		}
	}
	if !hasConfig || !hasWeights {
		res.Compatible = false
		res.Reason = "Uploaded model .zip archive must contain config.json and trainable weights (.safetensors or .bin)."
		return res
	}
	if (task == "text" || task == "vision") && !hasTokenizer {
		res.Compatible = false
		res.Reason = "Uploaded model .zip archive is missing tokenizer files (tokenizer.json / tokenizer_config.json) required for OpenAI chat fine-tuning."
		return res
	}
	return evaluateConfigJSONCompat(configBytes, task, method, datasetHasImages, res)
}

func evaluateConfigJSONCompat(configBytes []byte, task, method string, datasetHasImages bool, res FineTuningModelCompatResult) FineTuningModelCompatResult {
	if len(configBytes) > 0 {
		var cfg struct {
			Architectures      []string       `json:"architectures"`
			ModelType          string         `json:"model_type"`
			QuantizationConfig map[string]any `json:"quantization_config"`
		}
		if common.Unmarshal(configBytes, &cfg) == nil {
			if len(cfg.Architectures) > 0 {
				res.Architecture = cfg.Architectures[0]
			}
			res.ModelType = cfg.ModelType
			if cfg.QuantizationConfig != nil {
				if qm, _ := cfg.QuantizationConfig["quant_method"].(string); qm != "" {
					res.QuantMethod = strings.ToLower(strings.TrimSpace(qm))
				}
			}
		}
	}
	res.SupportsVision = isVisionArchitecture(res.Architecture, res.ModelType, res.PipelineTag)

	// Reject pre-quantized inference-only checkpoints on Full FT (and GPTQ/AWQ/MLX/EXL2 generally)
	if res.QuantMethod != "" {
		switch res.QuantMethod {
		case "awq", "gptq", "exl2", "marlin", "eetq", "hqq", "fp8", "fbgemm_fp8":
			if method == "full" || res.QuantMethod == "awq" || res.QuantMethod == "exl2" {
				res.Compatible = false
				res.Reason = fmt.Sprintf("Model uses pre-quantized %s weights which are incompatible with %s fine-tuning. Select the unquantized base model instead.", strings.ToUpper(res.QuantMethod), strings.ToUpper(method))
				return res
			}
		}
	}

	switch task {
	case "text":
		if isNonTextArchitecture(res.Architecture, res.ModelType, res.PipelineTag) {
			res.Compatible = false
			res.Reason = fmt.Sprintf("Model %q (architecture=%s, task=%s) is not a Causal Language Model and cannot be fine-tuned on OpenAI chat/text datasets.", res.ModelID, nonEmptyOr(res.Architecture, "unknown"), nonEmptyOr(res.PipelineTag, "unknown"))
			return res
		}
		if res.Architecture != "" && !isCausalLMArchitecture(res.Architecture, res.ModelType, res.PipelineTag) && !res.SupportsVision {
			res.Compatible = false
			res.Reason = fmt.Sprintf("Model architecture %q is not a supported Causal LM for OpenAI chat fine-tuning. Select a *ForCausalLM model (e.g. Qwen, Llama, Mistral, Gemma, SmolLM).", res.Architecture)
			return res
		}
		if datasetHasImages && !res.SupportsVision {
			res.Compatible = false
			res.Reason = fmt.Sprintf("Dataset contains multimodal images, but %q (%s) is a text-only model. Choose a Vision-Language model.", res.ModelID, nonEmptyOr(res.Architecture, "text-only"))
			return res
		}
		res.Compatible = true
		return res
	case "vision":
		if !res.SupportsVision {
			res.Compatible = false
			res.Reason = fmt.Sprintf("Model %q (architecture=%s) is not a Vision-Language model and cannot be fine-tuned on multimodal vision datasets.", res.ModelID, nonEmptyOr(res.Architecture, "unknown"))
			return res
		}
		res.Compatible = true
		return res
	case "image":
		if isCausalLMArchitecture(res.Architecture, res.ModelType, res.PipelineTag) {
			res.Compatible = false
			res.Reason = fmt.Sprintf("Model %q is a text LLM (%s), not an image diffusion model. Switch Task Type to Text / LLM or select a Stable Diffusion / Flux model.", res.ModelID, res.Architecture)
			return res
		}
		res.Compatible = true
		return res
	case "audio":
		if isCausalLMArchitecture(res.Architecture, res.ModelType, res.PipelineTag) && !strings.Contains(strings.ToLower(res.ModelType), "audio") && !strings.Contains(strings.ToLower(res.PipelineTag), "audio") {
			res.Compatible = false
			res.Reason = fmt.Sprintf("Model %q (%s) does not appear to be an audio/speech model.", res.ModelID, nonEmptyOr(res.Architecture, res.PipelineTag))
			return res
		}
		res.Compatible = true
		return res
	}
	res.Compatible = true
	return res
}

func fetchAndCheckHFModelCompat(ctx context.Context, repoID, task, method string, datasetHasImages bool, hfToken string) FineTuningModelCompatResult {
	res := FineTuningModelCompatResult{
		ModelID: repoID,
		Checked: true,
	}
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 8 * time.Second}
	doGet := func(target string) ([]byte, int, error) {
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("User-Agent", "new-api-finetune-validator/1.0")
		if hfToken != "" {
			req.Header.Set("Authorization", "Bearer "+hfToken)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		return body, resp.StatusCode, nil
	}

	// Escape each segment separately so owner/repo slash is preserved
	parts := strings.SplitN(repoID, "/", 2)
	escapedRepo := url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])

	metaBytes, statusCode, err := doGet("https://huggingface.co/api/models/" + escapedRepo + "?blobs=true")
	if err != nil {
		// Network hiccup: don't cache false negative; report clearly
		res.Checked = false
		res.Compatible = true
		res.Warnings = append(res.Warnings, "Could not reach Hugging Face API to verify model metadata right now.")
		return res
	}
	if statusCode == http.StatusNotFound {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Hugging Face repository %q was not found.", repoID)
		return res
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Hugging Face repository %q is private or gated. Configure a Hugging Face token with access to this repository.", repoID)
		return res
	}
	if statusCode >= 400 {
		res.Checked = false
		res.Compatible = true
		return res
	}

	var hfMeta struct {
		ID          string `json:"id"`
		Gated       any    `json:"gated"`
		PipelineTag string `json:"pipeline_tag"`
		LibraryName string `json:"library_name"`
		Safetensors *struct {
			Total int64 `json:"total"`
		} `json:"safetensors"`
		Siblings []struct {
			RFilename string `json:"rfilename"`
			Size      int64  `json:"size"`
			LFS       *struct {
				Size int64 `json:"size"`
			} `json:"lfs"`
		} `json:"siblings"`
	}
	if uErr := common.Unmarshal(metaBytes, &hfMeta); uErr != nil {
		res.Checked = false
		res.Compatible = true
		return res
	}
	res.PipelineTag = strings.TrimSpace(hfMeta.PipelineTag)

	if gatedBool, ok := hfMeta.Gated.(bool); ok && gatedBool && hfToken == "" {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Model %q is gated on Hugging Face and requires a Hugging Face access token configured in System Settings > Model Deployment.", repoID)
		return res
	}
	if gatedStr, ok := hfMeta.Gated.(string); ok && gatedStr != "" && gatedStr != "false" && hfToken == "" {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Model %q is gated on Hugging Face and requires a Hugging Face access token configured in System Settings > Model Deployment.", repoID)
		return res
	}

	var (
		hasConfig     bool
		hasModelIndex bool
		hasTokenizer  bool
		hasTrainable  bool
		hasGGUFOnly   bool
		hasONNXOnly   bool
		totalWeights  int64
	)
	for _, s := range hfMeta.Siblings {
		low := strings.ToLower(s.RFilename)
		base := strings.ToLower(filepath.Base(low))
		fSize := s.Size
		if fSize <= 0 && s.LFS != nil && s.LFS.Size > 0 {
			fSize = s.LFS.Size
		}
		switch {
		case base == "config.json":
			hasConfig = true
		case base == "model_index.json":
			hasModelIndex = true
		case base == "tokenizer.json" || base == "tokenizer_config.json" || base == "tokenizer.model" || base == "vocab.json" || base == "spiece.model":
			hasTokenizer = true
		case strings.HasSuffix(low, ".safetensors") || strings.HasSuffix(low, ".bin") || strings.HasSuffix(low, ".pt") || strings.HasSuffix(low, ".ckpt"):
			if !strings.Contains(low, "training_args") && !strings.Contains(low, "optimizer") {
				hasTrainable = true
				totalWeights += fSize
			}
		case strings.HasSuffix(low, ".gguf") || strings.HasSuffix(low, ".ggml"):
			hasGGUFOnly = true
		case strings.HasSuffix(low, ".onnx"):
			hasONNXOnly = true
		}
	}

	res.TotalWeightBytes = totalWeights
	if hfMeta.Safetensors != nil && hfMeta.Safetensors.Total > 0 {
		res.ParameterBillions = float64(hfMeta.Safetensors.Total) / 1_000_000_000.0
	} else {
		res.ParameterBillions = ParseModelBillions(repoID, "", totalWeights)
	}

	if !hasTrainable {
		res.Compatible = false
		if hasGGUFOnly {
			res.Reason = fmt.Sprintf("Repository %q only contains GGUF/GGML quantized weights (inference-only) and cannot be fine-tuned. Select the original non-GGUF Safetensors model repository.", repoID)
		} else if hasONNXOnly {
			res.Reason = fmt.Sprintf("Repository %q only contains ONNX inference weights and cannot be fine-tuned with PyTorch/PEFT.", repoID)
		} else {
			res.Reason = fmt.Sprintf("Repository %q does not contain trainable PyTorch/Safetensors weights.", repoID)
		}
		return res
	}

	if strings.EqualFold(hfMeta.LibraryName, "mlx") || strings.Contains(strings.ToLower(repoID), "-mlx") {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Repository %q is an Apple MLX-format model and cannot be fine-tuned on CUDA/PyTorch workers.", repoID)
		return res
	}

	if task == "image" {
		if hasModelIndex || strings.Contains(strings.ToLower(hfMeta.LibraryName), "diffusers") || res.PipelineTag == "text-to-image" || res.PipelineTag == "image-to-image" {
			res.Compatible = true
			return res
		}
	}

	if (task == "text" || task == "vision") && hasModelIndex && !hasConfig {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Repository %q is a Diffusers image generation model, not a Text / LLM model compatible with OpenAI chat JSONL datasets.", repoID)
		return res
	}

	if (task == "text" || task == "vision") && !hasTokenizer {
		res.Compatible = false
		res.Reason = fmt.Sprintf("Repository %q is missing tokenizer files (tokenizer.json / tokenizer_config.json) required for OpenAI chat fine-tuning.", repoID)
		return res
	}

	var configBytes []byte
	if hasConfig {
		cb, cCode, cErr := doGet("https://huggingface.co/" + escapedRepo + "/resolve/main/config.json")
		if cErr == nil && cCode == http.StatusOK {
			configBytes = cb
		}
	}

	return evaluateConfigJSONCompat(configBytes, task, method, datasetHasImages, res)
}
