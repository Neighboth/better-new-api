package notebookscript

import (
	"fmt"
	"strings"
)

// Config holds parameters for generating a Colab/Kaggle model serving script.
type Config struct {
	Provider             string
	DeploymentName       string
	ModelRepo            string
	ModelFile            string
	TaskType             string // text, image, video, audio, embeddings
	Accelerator          string
	FallbackAccelerators []string
	IdleTimeoutSeconds   int
	HFToken              string
	CallbackURL          string
}

// NormalizeTaskType maps arbitrary/HF pipeline_tag values into supported task types.
func NormalizeTaskType(raw string) string {
	t := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(t, "video"):
		return "video"
	case strings.Contains(t, "image") || strings.Contains(t, "diffusion"):
		return "image"
	case strings.Contains(t, "audio") || strings.Contains(t, "speech") || strings.Contains(t, "tts"):
		return "audio"
	case strings.Contains(t, "embed") || strings.Contains(t, "feature-extraction"):
		return "embeddings"
	default:
		return "text"
	}
}

func pyQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return `"` + s + `"`
}

// Generate returns a self-contained Python serving script for Colab, Kaggle, Modal, or GPU containers.
func Generate(cfg Config) string {
	task := NormalizeTaskType(cfg.TaskType)
	idle := cfg.IdleTimeoutSeconds
	if idle < 0 {
		idle = 0
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Provider), "modal") {
		return generateModalScript(cfg, task, idle)
	}

	var pipPackages string
	var extraPipArgs string
	var needsTorch bool
	var taskHandler string

	switch task {
	case "video":
		needsTorch = true
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("diffusers", "diffusers"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("imageio[ffmpeg]", "imageio"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
		taskHandler = `
import uuid
from diffusers import DiffusionPipeline
from diffusers.utils import export_to_video

pipe = DiffusionPipeline.from_pretrained(MODEL_REPO, torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32, token=HF_TOKEN or None)
if torch.cuda.is_available():
    pipe = pipe.to("cuda")

@app.post("/v1/videos/generations")
async def generate_video(req: Request):
    touch()
    body = await req.json()
    prompt = body.get("prompt", "A serene mountain landscape")
    num_frames = int(body.get("num_frames", 16))
    result = pipe(prompt=prompt, num_frames=num_frames)
    frames = result.frames[0]
    filename = f"{uuid.uuid4().hex}.mp4"
    out_path = os.path.join(OUTPUT_DIR, filename)
    export_to_video(frames, out_path, fps=8)
    base = str(req.base_url).rstrip("/")
    return {"data": [{"url": f"{base}/outputs/{filename}"}]}
`
	case "image":
		needsTorch = true
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("diffusers", "diffusers"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("pillow", "PIL"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
		taskHandler = `
import uuid
from diffusers import AutoPipelineForText2Image

pipe = AutoPipelineForText2Image.from_pretrained(MODEL_REPO, torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32, token=HF_TOKEN or None)
if torch.cuda.is_available():
    pipe = pipe.to("cuda")

@app.post("/v1/images/generations")
async def generate_image(req: Request):
    touch()
    body = await req.json()
    prompt = body.get("prompt", "A scenic landscape")
    img = pipe(prompt=prompt).images[0]
    filename = f"{uuid.uuid4().hex}.png"
    out_path = os.path.join(OUTPUT_DIR, filename)
    img.save(out_path)
    base = str(req.base_url).rstrip("/")
    return {"data": [{"url": f"{base}/outputs/{filename}"}]}
`
	case "audio":
		needsTorch = true
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("scipy", "scipy"), ("soundfile", "soundfile"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
		taskHandler = `
import uuid, scipy.io.wavfile as wavfile
from transformers import pipeline as hf_pipeline

audio_pipe = hf_pipeline("text-to-audio", model=MODEL_REPO, device=0 if torch.cuda.is_available() else -1, token=HF_TOKEN or None)

@app.post("/v1/audio/speech")
@app.post("/v1/audio/generations")
async def generate_audio(req: Request):
    touch()
    body = await req.json()
    text = body.get("input") or body.get("prompt") or "Hello world"
    out = audio_pipe(text)
    filename = f"{uuid.uuid4().hex}.wav"
    out_path = os.path.join(OUTPUT_DIR, filename)
    wavfile.write(out_path, rate=int(out.get("sampling_rate", 16000)), data=out["audio"])
    base = str(req.base_url).rstrip("/")
    return {"url": f"{base}/outputs/{filename}", "data": [{"url": f"{base}/outputs/{filename}"}]}
`
	case "embeddings":
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("sentence-transformers", "sentence_transformers"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
		taskHandler = `
from sentence_transformers import SentenceTransformer

embed_model = SentenceTransformer(MODEL_REPO, use_auth_token=HF_TOKEN or None)

@app.post("/v1/embeddings")
async def create_embeddings(req: Request):
    touch()
    body = await req.json()
    inp = body.get("input", "")
    texts = inp if isinstance(inp, list) else [str(inp)]
    vecs = embed_model.encode(texts).tolist()
    return {
        "object": "list",
        "model": MODEL_REPO,
        "data": [{"object": "embedding", "index": i, "embedding": v} for i, v in enumerate(vecs)],
    }
`
	default: // text (supports both GGUF via llama-cpp-python and standard transformers)
		isGGUF := strings.HasSuffix(strings.ToLower(strings.TrimSpace(cfg.ModelFile)), ".gguf") ||
			strings.Contains(strings.ToLower(cfg.ModelRepo), "gguf")
		if isGGUF {
			pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("huggingface_hub", "huggingface_hub"), ("llama-cpp-python", "llama_cpp")]`
			if strings.TrimSpace(cfg.Accelerator) == "" || strings.EqualFold(strings.TrimSpace(cfg.Accelerator), "CPU") {
				extraPipArgs = `["--prefer-binary", "--extra-index-url", "https://abetlen.github.io/llama-cpp-python/whl/cpu"]`
			} else {
				extraPipArgs = `["--prefer-binary"]`
			}
			taskHandler = `
from huggingface_hub import hf_hub_download, list_repo_files
from llama_cpp import Llama

_llm = None
def get_llm():
    global _llm
    if _llm is not None:
        return _llm
    target_file = MODEL_FILE
    if not target_file:
        files = [f for f in list_repo_files(MODEL_REPO, token=HF_TOKEN or None) if f.lower().endswith(".gguf")]
        if not files:
            raise RuntimeError(f"No .gguf file found in {MODEL_REPO}")
        target_file = sorted(files)[0]
    print(f"[setup] Downloading GGUF model {MODEL_REPO}/{target_file}...", flush=True)
    local_path = hf_hub_download(repo_id=MODEL_REPO, filename=target_file, token=HF_TOKEN or None)
    n_gpu = -1 if os.path.exists("/dev/nvidia0") else 0
    _llm = Llama(model_path=local_path, n_ctx=4096, n_gpu_layers=n_gpu, verbose=False)
    return _llm

@app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    touch()
    body = await req.json()
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    return get_llm().create_chat_completion(messages=messages, max_tokens=max_tokens)
`
		} else {
			needsTorch = true
			pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
			taskHandler = `
from transformers import AutoModelForCausalLM, AutoTokenizer

_backend = None
def get_backend():
    global _backend
    if _backend is not None:
        return _backend
    print(f"[setup] Loading transformers model {MODEL_REPO}...", flush=True)
    tok = AutoTokenizer.from_pretrained(MODEL_REPO, token=HF_TOKEN or None)
    mdl = AutoModelForCausalLM.from_pretrained(
        MODEL_REPO,
        torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
        device_map="auto" if torch.cuda.is_available() else None,
        token=HF_TOKEN or None,
    )
    _backend = (tok, mdl)
    return _backend

@app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    touch()
    body = await req.json()
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    tok, mdl = get_backend()
    prompt = "\n".join(f"{m.get('role', 'user')}: {m.get('content', '')}" for m in messages) + "\nassistant:"
    inputs = tok(prompt, return_tensors="pt")
    if torch.cuda.is_available():
        inputs = {k: v.to(mdl.device) for k, v in inputs.items()}
    with torch.no_grad():
        output_ids = mdl.generate(**inputs, max_new_tokens=max_tokens)
    generated = tok.decode(output_ids[0][inputs["input_ids"].shape[-1]:], skip_special_tokens=True)
    return {
        "id": "chatcmpl-notebook",
        "object": "chat.completion",
        "model": MODEL_REPO,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": generated}, "finish_reason": "stop"}],
    }
`
		}
	}

	if extraPipArgs == "" {
		extraPipArgs = "[]"
	}
	torchImport := ""
	if needsTorch {
		torchImport = "import torch\n"
	}

	return fmt.Sprintf(`# Auto-generated Better-New-API %s serving script
# Deployment: %s | Model: %s | Task: %s | Accelerator: %s
import importlib.util
import json
import os
import re
import subprocess
import sys
import threading
import time
import urllib.request

MODEL_REPO = %s
MODEL_FILE = %s
HF_TOKEN = %s
CALLBACK_URL = %s
IDLE_TIMEOUT_SECONDS = %d
OUTPUT_DIR = "/tmp/model_outputs"
os.makedirs(OUTPUT_DIR, exist_ok=True)

def _report_tunnel_url(url: str):
    print(f"[tunnel] Tunnel allocated ({url}), waiting for local server on :8000...", flush=True)
    deadline = time.time() + 600
    while time.time() < deadline:
        try:
            with urllib.request.urlopen("http://127.0.0.1:8000/health", timeout=3) as resp:
                if 200 <= resp.status < 300:
                    break
        except Exception:
            pass
        time.sleep(2)
    with open("endpoint_url.txt", "w", encoding="utf-8") as f:
        f.write(url)
    print(f"[tunnel] Ready: {url}", flush=True)
    if not CALLBACK_URL:
        return
    payload = json.dumps({"endpoint_url": url, "status": "running"}).encode("utf-8")
    for _ in range(5):
        try:
            req = urllib.request.Request(
                CALLBACK_URL,
                data=payload,
                headers={"Content-Type": "application/json"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                if 200 <= resp.status < 300:
                    return
        except Exception as exc:
            print(f"[tunnel] callback retry: {exc}", flush=True)
        time.sleep(2)

def _start_cloudflared():
    print("[tunnel] Starting cloudflared tunnel...", flush=True)
    if os.system("which cloudflared > /dev/null 2>&1") != 0:
        print("[tunnel] Downloading cloudflared binary...", flush=True)
        os.system("wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -O /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared")
    proc = subprocess.Popen(
        ["cloudflared", "tunnel", "--url", "http://127.0.0.1:8000"],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    reported = False
    if proc.stdout is not None:
        for line in proc.stdout:
            if not reported:
                m = re.search(r"https://[a-zA-Z0-9-]+\.trycloudflare\.com", line)
                if m:
                    reported = True
                    _report_tunnel_url(m.group(0))

threading.Thread(target=_start_cloudflared, daemon=True).start()

def _ensure_packages(specs, extra_args=None):
    missing = [pkg for pkg, mod in specs if importlib.util.find_spec(mod) is None]
    if not missing:
        print("[setup] All required packages already installed", flush=True)
        return
    print(f"[setup] Installing packages: {', '.join(missing)}", flush=True)
    cmd = [sys.executable, "-m", "pip", "install", "-q"] + (extra_args or []) + missing
    subprocess.check_call(cmd)

_ensure_packages(%s, %s)

%simport uvicorn
from fastapi import FastAPI, Request
from fastapi.staticfiles import StaticFiles

app = FastAPI(title="Better-New-API Notebook Model Server")
app.mount("/outputs", StaticFiles(directory=OUTPUT_DIR), name="outputs")
_last_activity = time.time()

def touch():
    global _last_activity
    _last_activity = time.time()

def _idle_watchdog():
    if IDLE_TIMEOUT_SECONDS <= 0:
        return
    while True:
        time.sleep(10)
        if time.time() - _last_activity > IDLE_TIMEOUT_SECONDS:
            print(f"[watchdog] Idle for {IDLE_TIMEOUT_SECONDS}s, shutting down...")
            os._exit(0)

threading.Thread(target=_idle_watchdog, daemon=True).start()

@app.get("/health")
def health():
    touch()
    return {"status": "ok", "model": MODEL_REPO, "task": %s}

@app.get("/v1/models")
def list_models():
    touch()
    return {"object": "list", "data": [{"id": MODEL_REPO, "object": "model"}]}
%s
if __name__ == "__main__":
    print("[server] Starting uvicorn on port 8000...", flush=True)
    uvicorn.run(app, host="0.0.0.0", port=8000)
`,
		cfg.Provider,
		cfg.DeploymentName,
		cfg.ModelRepo,
		task,
		cfg.Accelerator,
		pyQuote(cfg.ModelRepo),
		pyQuote(cfg.ModelFile),
		pyQuote(cfg.HFToken),
		pyQuote(cfg.CallbackURL),
		idle,
		pipPackages,
		extraPipArgs,
		torchImport,
		pyQuote(task),
		taskHandler,
	)
}

func generateModalScript(cfg Config, task string, idle int) string {
	gpus := []string{}
	if strings.TrimSpace(cfg.Accelerator) != "" && !strings.EqualFold(cfg.Accelerator, "CPU") {
		gpus = append(gpus, cfg.Accelerator)
	}
	for _, fb := range cfg.FallbackAccelerators {
		fb = strings.TrimSpace(fb)
		if fb != "" && !strings.EqualFold(fb, "CPU") {
			gpus = append(gpus, fb)
		}
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
	if idle > 0 {
		scaledownLine = fmt.Sprintf("\n    scaledown_window=%d,", idle)
	}
	appName := cfg.DeploymentName
	if strings.TrimSpace(appName) == "" {
		appName = "model-server"
	}
	return fmt.Sprintf(`# Auto-generated Better-New-API Modal serving script
# Model: %s | Task: %s | GPU(s): %s
import modal

app = modal.App(%s)
image = modal.Image.debian_slim().pip_install("vllm", "diffusers", "transformers", "accelerate", "huggingface_hub", "fastapi", "uvicorn")
volume = modal.Volume.from_name("model-weights", create_if_missing=True)

MODEL_REPO = %s
TASK_TYPE = %s

@app.function(
    image=image,
    gpu=%s,
    volumes={"/models": volume},
    timeout=3600,%s
)
@modal.web_server(port=8000)
def serve():
    import subprocess
    if TASK_TYPE in ("text", "embeddings"):
        subprocess.Popen([
            "python", "-m", "vllm.entrypoints.openai.api_server",
            "--model", MODEL_REPO,
            "--download-dir", "/models",
            "--host", "0.0.0.0",
            "--port", "8000",
        ])
    else:
        subprocess.Popen(["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"])
`, cfg.ModelRepo, task, gpuArg, pyQuote(appName), pyQuote(cfg.ModelRepo), pyQuote(task), gpuArg, scaledownLine)
}
