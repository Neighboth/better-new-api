package modaldeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var modalURLRegex = regexp.MustCompile(`https://[a-zA-Z0-9._-]+\.modal\.run`)

type DeployConfig struct {
	AppName              string
	ModelRepo            string
	ModelFile            string
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

	var pipPkgs string
	var taskCode string

	switch task {
	case "video":
		pipPkgs = `"fastapi[standard]", "diffusers", "transformers", "accelerate", "imageio[ffmpeg]", "torch", "huggingface_hub"`
		taskCode = `
from diffusers import DiffusionPipeline
from diffusers.utils import export_to_video

_pipe = None
def get_pipe():
    global _pipe
    if _pipe is None:
        _pipe = DiffusionPipeline.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None, torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32)
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
		pipPkgs = `"fastapi[standard]", "diffusers", "transformers", "accelerate", "pillow", "torch", "huggingface_hub"`
		taskCode = `
from diffusers import AutoPipelineForText2Image

_pipe = None
def get_pipe():
    global _pipe
    if _pipe is None:
        _pipe = AutoPipelineForText2Image.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None, torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32)
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
import scipy.io.wavfile as wavfile
from transformers import pipeline as hf_pipeline

_pipe = None
def get_pipe():
    global _pipe
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
    global _model
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
		pipPkgs = `"fastapi[standard]", "transformers", "accelerate", "torch", "huggingface_hub", "llama-cpp-python"`
		taskCode = `
from huggingface_hub import hf_hub_download, list_repo_files

_backend = None
def get_backend():
    global _backend
    if _backend is not None:
        return _backend
    is_gguf = MODEL_FILE.lower().endswith(".gguf") or "gguf" in MODEL_REPO.lower()
    if is_gguf:
        from llama_cpp import Llama
        target_file = MODEL_FILE
        if not target_file:
            files = [f for f in list_repo_files(MODEL_REPO, token=HF_TOKEN or None) if f.lower().endswith(".gguf")]
            if not files:
                raise RuntimeError(f"No .gguf file found in {MODEL_REPO}")
            target_file = sorted(files)[0]
        local_path = hf_hub_download(repo_id=MODEL_REPO, filename=target_file, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
        n_gpu = -1 if torch.cuda.is_available() else 0
        llm = Llama(model_path=local_path, n_ctx=4096, n_gpu_layers=n_gpu, verbose=False)
        _backend = ("gguf", llm)
    else:
        from transformers import AutoModelForCausalLM, AutoTokenizer
        tok = AutoTokenizer.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
        mdl = AutoModelForCausalLM.from_pretrained(
            MODEL_REPO,
            cache_dir=CACHE_DIR,
            token=HF_TOKEN or None,
            torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
            device_map="auto" if torch.cuda.is_available() else None,
        )
        _backend = ("transformers", (tok, mdl))
    return _backend

@web_app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    body = await req.json()
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    kind, engine = get_backend()
    if kind == "gguf":
        res = engine.create_chat_completion(messages=messages, max_tokens=max_tokens)
        return res
    tok, mdl = engine
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

	return fmt.Sprintf(`import os
import uuid
import modal

app = modal.App(%s)
image = modal.Image.debian_slim(python_version="3.11").pip_install(%s)
volume = modal.Volume.from_name("better-new-api-models", create_if_missing=True)

MODEL_REPO = %s
MODEL_FILE = %s
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
    import torch
    from fastapi import FastAPI, Request
    from fastapi.staticfiles import StaticFiles

    os.makedirs(OUT_DIR, exist_ok=True)
    web_app = FastAPI(title="Better-New-API Modal Server")
    web_app.mount("/outputs", StaticFiles(directory=OUT_DIR), name="outputs")

    @web_app.get("/health")
    def health():
        return {"status": "ok", "model": MODEL_REPO, "task": %s}
%s
    return web_app
`,
		pyQuote(cfg.AppName),
		pipPkgs,
		pyQuote(cfg.ModelRepo),
		pyQuote(cfg.ModelFile),
		pyQuote(cfg.HFToken),
		pyQuote(volPath),
		gpuArg,
		scaledownLine,
		pyQuote(task),
		taskCode,
	)
}

func findPythonWithModal(ctx context.Context) (string, error) {
	candidates := []string{"python", "python3", "py"}
	for _, bin := range candidates {
		path, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		check := exec.CommandContext(ctx, path, "-m", "modal", "--version")
		if err := check.Run(); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python with 'modal' package not found in PATH (install via: pip install modal)")
}

// Deploy deploys the Modal ASGI app using `python -m modal deploy` and returns
// the live *.modal.run endpoint URL.
func Deploy(ctx context.Context, cfg DeployConfig) (string, string, error) {
	script := BuildAppScript(cfg)
	pyBin, err := findPythonWithModal(ctx)
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

	cmd := exec.CommandContext(ctx, pyBin, "-m", "modal", "deploy", tmpPath)
	cmd.Env = append(os.Environ(),
		"MODAL_TOKEN_ID="+strings.TrimSpace(cfg.TokenID),
		"MODAL_TOKEN_SECRET="+strings.TrimSpace(cfg.TokenSecret),
		"PYTHONIOENCODING=utf-8",
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

	match := modalURLRegex.FindString(outText)
	if match == "" {
		return "", script, fmt.Errorf("modal deploy succeeded but no *.modal.run URL found in output")
	}
	return match, script, nil
}

// StopApp stops a deployed Modal app (best effort).
func StopApp(ctx context.Context, appName, tokenID, tokenSecret string) error {
	if strings.TrimSpace(appName) == "" || strings.TrimSpace(tokenID) == "" || strings.TrimSpace(tokenSecret) == "" {
		return nil
	}
	pyBin, err := findPythonWithModal(ctx)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, pyBin, "-m", "modal", "app", "stop", appName)
	cmd.Env = append(os.Environ(),
		"MODAL_TOKEN_ID="+strings.TrimSpace(tokenID),
		"MODAL_TOKEN_SECRET="+strings.TrimSpace(tokenSecret),
	)
	return cmd.Run()
}
