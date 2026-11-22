package modaldeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var (
	modalURLRegex   = regexp.MustCompile(`https://[a-zA-Z0-9._-]+\.modal\.run`)
	ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	lineWrapRegex   = regexp.MustCompile(`\r?\n[ \t│]*`)
)

func extractModalURL(output string) string {
	clean := ansiEscapeRegex.ReplaceAllString(output, "")
	if m := modalURLRegex.FindString(clean); m != "" {
		return m
	}
	unwrapped := lineWrapRegex.ReplaceAllString(clean, "")
	return modalURLRegex.FindString(unwrapped)
}

type ModelFileSpec struct {
	Repo string `json:"repo,omitempty"`
	File string `json:"file"`
	Role string `json:"role,omitempty"` // main | text_encoder | text_encoder_2 | vae | unet | lora | extra
}

type DeployConfig struct {
	AppName              string
	ModelRepo            string
	ModelFile            string
	ModelFiles           []ModelFileSpec
	TaskType             string // text, image, video, audio, embeddings
	Accelerator          string
	FallbackAccelerators []string
	IdleTimeoutSeconds   int
	TokenID              string
	TokenSecret          string
	HFToken              string
	SharedVolumePath     string
}

func pyQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return `"` + s + `"`
}

func indentPythonBlock(code string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = pad + line
		} else {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// BuildAppScript generates a Modal v1.x ASGI FastAPI script that downloads and
// serves the requested Hugging Face model (including GGUF, diffusers, video, audio).
func BuildAppScript(cfg DeployConfig) string {
	task := strings.ToLower(strings.TrimSpace(cfg.TaskType))
	if task == "" {
		task = "text"
	}

	var gpus []string
	addGPU := func(g string) {
		g = strings.TrimSpace(g)
		if g == "" || strings.EqualFold(g, "CPU") {
			return
		}
		for _, existing := range gpus {
			if existing == g {
				return
			}
		}
		gpus = append(gpus, g)
	}
	addGPU(cfg.Accelerator)
	for _, fb := range cfg.FallbackAccelerators {
		addGPU(fb)
	}

	gpuArg := "None"
	if len(gpus) == 1 {
		gpuArg = pyQuote(gpus[0])
	} else if len(gpus) > 1 {
		quoted := make([]string, len(gpus))
		for i, g := range gpus {
			quoted[i] = pyQuote(g)
		}
		gpuArg = "[" + strings.Join(quoted, ", ") + "]"
	}

	scaledownLine := ""
	if cfg.IdleTimeoutSeconds > 0 {
		scaledownLine = fmt.Sprintf("\n    scaledown_window=%d,", cfg.IdleTimeoutSeconds)
	}

	volPath := strings.TrimSpace(cfg.SharedVolumePath)
	if volPath == "" {
		volPath = "/vol/models"
	}

	modelFilesJSON := "[]"
	if len(cfg.ModelFiles) > 0 {
		if b, err := common.Marshal(cfg.ModelFiles); err == nil {
			modelFilesJSON = string(b)
		}
	}

	var pipPkgs string
	var extraPipOpts string
	var aptChain string
	var taskCode string

	switch task {
	case "video":
		pipPkgs = `"fastapi[standard]", "diffusers", "transformers", "accelerate", "imageio[ffmpeg]", "torch", "huggingface_hub", "safetensors", "omegaconf"`
		taskCode = `
import json
import torch
from diffusers import DiffusionPipeline
from huggingface_hub import hf_hub_download, list_repo_files
from diffusers.utils import export_to_video

_pipe = None
def get_pipe():
    nonlocal _pipe
    if _pipe is None:
        dtype = torch.float16 if torch.cuda.is_available() else torch.float32
        entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
        main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
        explicit_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
        explicit_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
        repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
        has_index = "model_index.json" in repo_files
        if has_index and not explicit_file.lower().endswith((".safetensors", ".ckpt")):
            _pipe = DiffusionPipeline.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None, torch_dtype=dtype)
        else:
            target = explicit_file
            if not target:
                cands = [f for f in repo_files if f.lower().endswith((".safetensors", ".ckpt"))]
                if not cands:
                    raise RuntimeError(f"No model_index.json or .safetensors/.ckpt file found in {MODEL_REPO}")
                target = sorted(cands)[0]
            local_ckpt = hf_hub_download(repo_id=explicit_repo, filename=target, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            _pipe = DiffusionPipeline.from_single_file(local_ckpt, torch_dtype=dtype)
        for entry in entries:
            role = (entry.get("role") or "extra").lower()
            fn = entry.get("file") or ""
            if not fn or role == "main":
                continue
            lp = hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            if role == "lora" and hasattr(_pipe, "load_lora_weights"):
                _pipe.load_lora_weights(lp)
            elif role == "vae" and hasattr(_pipe, "vae"):
                from diffusers import AutoencoderKL
                _pipe.vae = AutoencoderKL.from_single_file(lp, torch_dtype=dtype)
        if torch.cuda.is_available():
            _pipe = _pipe.to("cuda")
    return _pipe

@web_app.post("/v1/videos/generations")
async def gen_video(req: Request):
    body = await req.json()
    prompt = body.get("prompt", "A serene mountain landscape")
    num_frames = int(body.get("num_frames", 16))
    pipe = get_pipe()
    frames = pipe(prompt=prompt, num_frames=num_frames).frames[0]
    fn = f"{uuid.uuid4().hex}.mp4"
    out = os.path.join(OUT_DIR, fn)
    export_to_video(frames, out, fps=8)
    base = str(req.base_url).rstrip("/")
    return {"data": [{"url": f"{base}/outputs/{fn}"}]}
`
	case "image":
		pipPkgs = `"fastapi[standard]", "diffusers", "transformers", "accelerate", "pillow", "torch", "huggingface_hub", "safetensors", "omegaconf"`
		taskCode = `
import json
import torch
from diffusers import AutoPipelineForText2Image
from huggingface_hub import hf_hub_download, list_repo_files

_pipe = None
def get_pipe():
    nonlocal _pipe
    if _pipe is None:
        dtype = torch.float16 if torch.cuda.is_available() else torch.float32
        entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
        main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
        explicit_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
        explicit_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
        repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
        has_index = "model_index.json" in repo_files
        if has_index and not explicit_file.lower().endswith((".safetensors", ".ckpt")):
            _pipe = AutoPipelineForText2Image.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None, torch_dtype=dtype)
        else:
            target = explicit_file
            if not target:
                cands = [f for f in repo_files if f.lower().endswith((".safetensors", ".ckpt"))]
                if not cands:
                    raise RuntimeError(f"No model_index.json or .safetensors/.ckpt file found in {MODEL_REPO}")
                target = sorted(cands)[0]
            local_ckpt = hf_hub_download(repo_id=explicit_repo, filename=target, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            from diffusers import StableDiffusionPipeline, StableDiffusionXLPipeline, DiffusionPipeline
            errs = []
            for cls in (StableDiffusionPipeline, StableDiffusionXLPipeline, DiffusionPipeline):
                try:
                    _pipe = cls.from_single_file(local_ckpt, torch_dtype=dtype)
                    break
                except Exception as exc:
                    errs.append(f"{cls.__name__}: {exc}")
            if _pipe is None:
                raise RuntimeError("Failed to load single-file image checkpoint: " + " | ".join(errs))
        for entry in entries:
            role = (entry.get("role") or "extra").lower()
            fn = entry.get("file") or ""
            if not fn or role == "main":
                continue
            lp = hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            if role == "vae" and hasattr(_pipe, "vae"):
                from diffusers import AutoencoderKL
                _pipe.vae = AutoencoderKL.from_single_file(lp, torch_dtype=dtype)
            elif role == "lora" and hasattr(_pipe, "load_lora_weights"):
                _pipe.load_lora_weights(lp)
            elif role in ("text_encoder", "text_encoder_2", "unet") and hasattr(_pipe, role):
                from safetensors.torch import load_file
                sd = load_file(lp) if lp.lower().endswith(".safetensors") else torch.load(lp, map_location="cpu")
                getattr(_pipe, role).load_state_dict(sd, strict=False)
        if torch.cuda.is_available():
            _pipe = _pipe.to("cuda")
    return _pipe

@web_app.post("/v1/images/generations")
async def gen_image(req: Request):
    body = await req.json()
    prompt = body.get("prompt", "A scenic landscape")
    img = get_pipe()(prompt=prompt).images[0]
    fn = f"{uuid.uuid4().hex}.png"
    out = os.path.join(OUT_DIR, fn)
    img.save(out)
    base = str(req.base_url).rstrip("/")
    return {"data": [{"url": f"{base}/outputs/{fn}"}]}
`
	case "audio":
		pipPkgs = `"fastapi[standard]", "transformers", "accelerate", "scipy", "soundfile", "torch", "huggingface_hub"`
		taskCode = `
import torch
import scipy.io.wavfile as wavfile
from transformers import pipeline as hf_pipeline

_pipe = None
def get_pipe():
    nonlocal _pipe
    if _pipe is None:
        _pipe = hf_pipeline("text-to-audio", model=MODEL_REPO, device=0 if torch.cuda.is_available() else -1, token=HF_TOKEN or None)
    return _pipe

@web_app.post("/v1/audio/speech")
@web_app.post("/v1/audio/generations")
async def gen_audio(req: Request):
    body = await req.json()
    text = body.get("input") or body.get("prompt") or "Hello world"
    out = get_pipe()(text)
    fn = f"{uuid.uuid4().hex}.wav"
    out_path = os.path.join(OUT_DIR, fn)
    wavfile.write(out_path, rate=int(out.get("sampling_rate", 16000)), data=out["audio"])
    base = str(req.base_url).rstrip("/")
    return {"url": f"{base}/outputs/{fn}", "data": [{"url": f"{base}/outputs/{fn}"}]}
`
	case "embeddings":
		pipPkgs = `"fastapi[standard]", "sentence-transformers", "torch", "huggingface_hub"`
		taskCode = `
from sentence_transformers import SentenceTransformer

_model = None
def get_model():
    nonlocal _model
    if _model is None:
        _model = SentenceTransformer(MODEL_REPO, cache_folder=CACHE_DIR, token=HF_TOKEN or None)
    return _model

@web_app.post("/v1/embeddings")
async def embeddings(req: Request):
    body = await req.json()
    inp = body.get("input", "")
    texts = inp if isinstance(inp, list) else [str(inp)]
    vecs = get_model().encode(texts).tolist()
    return {"object": "list", "model": MODEL_REPO, "data": [{"object": "embedding", "index": i, "embedding": v} for i, v in enumerate(vecs)]}
`
	default: // text (supports both GGUF via llama-cpp-python and standard transformers)
		isGGUF := strings.HasSuffix(strings.ToLower(strings.TrimSpace(cfg.ModelFile)), ".gguf") ||
			strings.Contains(strings.ToLower(cfg.ModelRepo), "gguf")
		if isGGUF {
			aptChain = `.apt_install("libgomp1")`
			pipPkgs = `"fastapi[standard]", "huggingface_hub", "llama-cpp-python"`
			if gpuArg == "None" {
				extraPipOpts = `, extra_index_url="https://abetlen.github.io/llama-cpp-python/whl/cpu"`
			} else {
				extraPipOpts = `, extra_options="--prefer-binary"`
			}
			taskCode = `
import json
from huggingface_hub import hf_hub_download, list_repo_files
from llama_cpp import Llama

_llm = None
def get_llm():
    nonlocal _llm
    if _llm is not None:
        return _llm
    entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
    main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
    target_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
    target_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
    for entry in entries:
        fn = entry.get("file") or ""
        if fn and fn != target_file:
            hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
    if not target_file:
        files = [f for f in list_repo_files(MODEL_REPO, token=HF_TOKEN or None) if f.lower().endswith(".gguf")]
        if not files:
            raise RuntimeError(f"No .gguf file found in {MODEL_REPO}")
        target_file = sorted(files)[0]
    local_path = hf_hub_download(repo_id=target_repo, filename=target_file, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
    n_gpu = -1 if os.path.exists("/dev/nvidia0") else 0
    _llm = Llama(model_path=local_path, n_ctx=4096, n_gpu_layers=n_gpu, verbose=False)
    return _llm

@web_app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    body = await req.json()
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    return get_llm().create_chat_completion(messages=messages, max_tokens=max_tokens)
`
		} else {
			pipPkgs = `"fastapi[standard]", "transformers", "accelerate", "torch", "huggingface_hub"`
			taskCode = `
import torch
from transformers import AutoModelForCausalLM, AutoTokenizer

_backend = None
def get_backend():
    nonlocal _backend
    if _backend is not None:
        return _backend
    tok = AutoTokenizer.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
    mdl = AutoModelForCausalLM.from_pretrained(
        MODEL_REPO,
        cache_dir=CACHE_DIR,
        token=HF_TOKEN or None,
        torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
        device_map="auto" if torch.cuda.is_available() else None,
    )
    _backend = (tok, mdl)
    return _backend

@web_app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    body = await req.json()
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    tok, mdl = get_backend()
    prompt = "\n".join(f"{m.get('role', 'user')}: {m.get('content', '')}" for m in messages) + "\nassistant:"
    inputs = tok(prompt, return_tensors="pt")
    if torch.cuda.is_available():
        inputs = {k: v.to(mdl.device) for k, v in inputs.items()}
    with torch.no_grad():
        out_ids = mdl.generate(**inputs, max_new_tokens=max_tokens)
    text = tok.decode(out_ids[0][inputs["input_ids"].shape[-1]:], skip_special_tokens=True)
    return {
        "id": "chatcmpl-modal",
        "object": "chat.completion",
        "model": MODEL_REPO,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}],
    }
`
		}
	}

	return fmt.Sprintf(`import os
import uuid
import modal

app = modal.App(%s)
image = modal.Image.debian_slim(python_version="3.11")%s.pip_install(%s%s)
volume = modal.Volume.from_name("better-new-api-models", create_if_missing=True)

MODEL_REPO = %s
MODEL_FILE = %s
MODEL_FILES_JSON = %s
HF_TOKEN = %s
CACHE_DIR = %s
OUT_DIR = "/tmp/model_outputs"

@app.function(
    image=image,
    gpu=%s,
    volumes={CACHE_DIR: volume},
    timeout=3600,%s
)
@modal.asgi_app()
def fastapi_app():
    from fastapi import FastAPI, Request
    from fastapi.responses import JSONResponse
    from fastapi.staticfiles import StaticFiles

    os.makedirs(OUT_DIR, exist_ok=True)
    web_app = FastAPI(title="Better-New-API Modal Server")
    web_app.mount("/outputs", StaticFiles(directory=OUT_DIR), name="outputs")

    @web_app.exception_handler(Exception)
    async def unhandled_exception_handler(request: Request, exc: Exception):
        return JSONResponse(status_code=500, content={"error": f"{type(exc).__name__}: {exc}"})

    @web_app.get("/health")
    def health():
        return {"status": "ok", "model": MODEL_REPO, "task": %s}
%s
    return web_app
`,
		pyQuote(cfg.AppName),
		aptChain,
		pipPkgs,
		extraPipOpts,
		pyQuote(cfg.ModelRepo),
		pyQuote(cfg.ModelFile),
		pyQuote(modelFilesJSON),
		pyQuote(cfg.HFToken),
		pyQuote(volPath),
		gpuArg,
		scaledownLine,
		pyQuote(task),
		indentPythonBlock(taskCode, 4),
	)
}

type modalCmd struct {
	bin        string
	prefixArgs []string
}

func (m modalCmd) command(ctx context.Context, args ...string) *exec.Cmd {
	fullArgs := append(append([]string{}, m.prefixArgs...), args...)
	return exec.CommandContext(ctx, m.bin, fullArgs...)
}

var modalBootstrapMu sync.Mutex

func modalVenvPythonPath() string {
	base := filepath.Join(os.TempDir(), "better-new-api-modal-venv")
	if runtime.GOOS == "windows" {
		return filepath.Join(base, "Scripts", "python.exe")
	}
	return filepath.Join(base, "bin", "python")
}

func testModalCmd(ctx context.Context, cand modalCmd) bool {
	if _, err := os.Stat(cand.bin); err != nil {
		if _, lookErr := exec.LookPath(cand.bin); lookErr != nil {
			return false
		}
	}
	return cand.command(ctx, "--version").Run() == nil
}

func commonModalCandidates() []modalCmd {
	var out []modalCmd
	out = append(out, modalCmd{bin: modalVenvPythonPath(), prefixArgs: []string{"-m", "modal"}})
	out = append(out, modalCmd{bin: "modal"})
	for _, py := range []string{"python", "python3"} {
		out = append(out, modalCmd{bin: py, prefixArgs: []string{"-m", "modal"}})
	}
	if runtime.GOOS == "windows" {
		out = append(out, modalCmd{bin: "py", prefixArgs: []string{"-m", "modal"}})
		out = append(out, modalCmd{bin: "py", prefixArgs: []string{"-3", "-m", "modal"}})
		patterns := []string{}
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			patterns = append(patterns,
				filepath.Join(localApp, "Programs", "Python", "Python3*", "Scripts", "modal.exe"),
				filepath.Join(localApp, "Programs", "Python", "Python3*", "python.exe"),
			)
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			patterns = append(patterns,
				filepath.Join(home, "AppData", "Roaming", "Python", "Python3*", "Scripts", "modal.exe"),
			)
		}
		patterns = append(patterns, `C:\Python3*\python.exe`, `C:\Python3*\Scripts\modal.exe`)
		for _, pat := range patterns {
			matches, _ := filepath.Glob(pat)
			for _, m := range matches {
				if strings.HasSuffix(strings.ToLower(m), "modal.exe") {
					out = append(out, modalCmd{bin: m})
				} else {
					out = append(out, modalCmd{bin: m, prefixArgs: []string{"-m", "modal"}})
				}
			}
		}
	} else {
		for _, p := range []string{"/usr/local/bin/modal", "/usr/bin/modal", "/opt/modal-venv/bin/modal"} {
			out = append(out, modalCmd{bin: p})
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			out = append(out, modalCmd{bin: filepath.Join(home, ".local", "bin", "modal")})
		}
	}
	return out
}

func findBasePython(ctx context.Context) (string, []string, bool) {
	type pyTry struct {
		bin  string
		args []string
	}
	tries := []pyTry{
		{bin: "python3"},
		{bin: "python"},
	}
	if runtime.GOOS == "windows" {
		tries = append(tries, pyTry{bin: "py", args: []string{"-3"}}, pyTry{bin: "py"})
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			matches, _ := filepath.Glob(filepath.Join(localApp, "Programs", "Python", "Python3*", "python.exe"))
			for _, m := range matches {
				tries = append(tries, pyTry{bin: m})
			}
		}
	}
	for _, t := range tries {
		path, err := exec.LookPath(t.bin)
		if err != nil {
			if _, statErr := os.Stat(t.bin); statErr != nil {
				continue
			}
			path = t.bin
		}
		checkArgs := append(append([]string{}, t.args...), "--version")
		if exec.CommandContext(ctx, path, checkArgs...).Run() == nil {
			return path, t.args, true
		}
	}
	return "", nil, false
}

func findOrBootstrapModalCommand(ctx context.Context) (modalCmd, error) {
	for _, cand := range commonModalCandidates() {
		if testModalCmd(ctx, cand) {
			return cand, nil
		}
	}

	modalBootstrapMu.Lock()
	defer modalBootstrapMu.Unlock()

	for _, cand := range commonModalCandidates() {
		if testModalCmd(ctx, cand) {
			return cand, nil
		}
	}

	pyBin, pyArgs, ok := findBasePython(ctx)
	if !ok {
		return modalCmd{}, fmt.Errorf("python with 'modal' package not found in PATH (install Python 3 and run: pip install modal)")
	}

	venvDir := filepath.Join(os.TempDir(), "better-new-api-modal-venv")
	venvPy := modalVenvPythonPath()
	venvArgs := append(append([]string{}, pyArgs...), "-m", "venv", venvDir)
	if err := exec.CommandContext(ctx, pyBin, venvArgs...).Run(); err == nil {
		if out, pipErr := exec.CommandContext(ctx, venvPy, "-m", "pip", "install", "--quiet", "modal").CombinedOutput(); pipErr == nil {
			cand := modalCmd{bin: venvPy, prefixArgs: []string{"-m", "modal"}}
			if testModalCmd(ctx, cand) {
				return cand, nil
			}
		} else {
			_ = out
		}
	}

	// Fallback: pip install --user modal on the discovered base Python
	userInstallArgs := append(append([]string{}, pyArgs...), "-m", "pip", "install", "--user", "--quiet", "modal")
	if out, err := exec.CommandContext(ctx, pyBin, userInstallArgs...).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return modalCmd{}, fmt.Errorf("failed to auto-install 'modal' Python package: %v — %s", err, msg)
	}
	cand := modalCmd{bin: pyBin, prefixArgs: append(append([]string{}, pyArgs...), "-m", "modal")}
	if testModalCmd(ctx, cand) {
		return cand, nil
	}
	return modalCmd{}, fmt.Errorf("python with 'modal' package not found in PATH (install via: pip install modal)")
}

// Deploy deploys the Modal ASGI app using `modal deploy` and returns
// the live *.modal.run endpoint URL.
func Deploy(ctx context.Context, cfg DeployConfig) (string, string, error) {
	script := BuildAppScript(cfg)
	cmdSpec, err := findOrBootstrapModalCommand(ctx)
	if err != nil {
		return "", script, err
	}

	tmpFile, err := os.CreateTemp("", "modal-deploy-*.py")
	if err != nil {
		return "", script, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()
		return "", script, err
	}
	_ = tmpFile.Close()

	cmd := cmdSpec.command(ctx, "deploy", tmpPath)
	cmd.Env = append(os.Environ(),
		"MODAL_TOKEN_ID="+strings.TrimSpace(cfg.TokenID),
		"MODAL_TOKEN_SECRET="+strings.TrimSpace(cfg.TokenSecret),
		"PYTHONIOENCODING=utf-8",
		"COLUMNS=500",
		"NO_COLOR=1",
		"FORCE_COLOR=0",
		"TERM=dumb",
	)
	outBytes, err := cmd.CombinedOutput()
	outText := string(outBytes)
	if err != nil {
		msg := strings.TrimSpace(outText)
		if len(msg) > 600 {
			msg = msg[len(msg)-600:]
		}
		return "", script, fmt.Errorf("modal deploy failed: %v — %s", err, msg)
	}

	match := extractModalURL(outText)
	if match == "" {
		// Fallback: query Modal SDK directly for the deployed web endpoint URL.
		pySnippet := fmt.Sprintf("import modal; print(modal.Function.from_name(%s, 'fastapi_app').get_web_url() or '')", pyQuote(cfg.AppName))
		var lookupCmd *exec.Cmd
		if len(cmdSpec.prefixArgs) >= 2 && cmdSpec.prefixArgs[len(cmdSpec.prefixArgs)-2] == "-m" {
			baseArgs := append([]string{}, cmdSpec.prefixArgs[:len(cmdSpec.prefixArgs)-2]...)
			baseArgs = append(baseArgs, "-c", pySnippet)
			lookupCmd = exec.CommandContext(ctx, cmdSpec.bin, baseArgs...)
		} else if pyBin, pyArgs, ok := findBasePython(ctx); ok {
			baseArgs := append(append([]string{}, pyArgs...), "-c", pySnippet)
			lookupCmd = exec.CommandContext(ctx, pyBin, baseArgs...)
		}
		if lookupCmd != nil {
			lookupCmd.Env = cmd.Env
			if urlOut, lookupErr := lookupCmd.CombinedOutput(); lookupErr == nil {
				match = extractModalURL(string(urlOut))
			}
		}
	}
	if match == "" {
		tail := strings.TrimSpace(outText)
		if len(tail) > 400 {
			tail = tail[len(tail)-400:]
		}
		return "", script, fmt.Errorf("modal deploy succeeded but no *.modal.run URL found in output: %s", tail)
	}
	return match, script, nil
}

// StopApp stops a deployed Modal app, falling back to `modal app list --json` to resolve the app ID if needed.
func StopApp(ctx context.Context, appName, tokenID, tokenSecret string) error {
	trimmed := strings.TrimSpace(appName)
	if trimmed == "" || strings.TrimSpace(tokenID) == "" || strings.TrimSpace(tokenSecret) == "" {
		return nil
	}
	cmdSpec, err := findOrBootstrapModalCommand(ctx)
	if err != nil {
		return err
	}
	env := append(os.Environ(),
		"MODAL_TOKEN_ID="+strings.TrimSpace(tokenID),
		"MODAL_TOKEN_SECRET="+strings.TrimSpace(tokenSecret),
		"PYTHONIOENCODING=utf-8",
		"COLUMNS=500",
		"NO_COLOR=1",
		"FORCE_COLOR=0",
		"TERM=dumb",
	)
	cmd := cmdSpec.command(ctx, "app", "stop", trimmed)
	cmd.Env = env
	outBytes, stopErr := cmd.CombinedOutput()
	if stopErr == nil {
		return nil
	}

	// Fallback: inspect `modal app list --json` for matching App ID / Description and stop by App ID.
	listCmd := cmdSpec.command(ctx, "app", "list", "--json")
	listCmd.Env = env
	if listOut, listErr := listCmd.CombinedOutput(); listErr == nil {
		var apps []map[string]any
		if common.Unmarshal(listOut, &apps) == nil {
			var stoppedAny bool
			for _, app := range apps {
				desc, _ := app["Description"].(string)
				name, _ := app["Name"].(string)
				appID, _ := app["App ID"].(string)
				if appID == "" {
					appID, _ = app["app_id"].(string)
				}
				state, _ := app["State"].(string)
				if strings.EqualFold(state, "stopped") {
					continue
				}
				if appID != "" && (strings.EqualFold(desc, trimmed) || strings.EqualFold(name, trimmed) || strings.EqualFold(appID, trimmed)) {
					stopByID := cmdSpec.command(ctx, "app", "stop", appID)
					stopByID.Env = env
					if stopByID.Run() == nil {
						stoppedAny = true
					}
				}
			}
			if stoppedAny {
				return nil
			}
		}
	}

	msg := strings.TrimSpace(string(outBytes))
	if len(msg) > 400 {
		msg = msg[len(msg)-400:]
	}
	if strings.Contains(strings.ToLower(msg), "not found") || strings.Contains(strings.ToLower(msg), "no app") {
		return nil
	}
	return fmt.Errorf("modal app stop failed: %v — %s", stopErr, msg)
}
