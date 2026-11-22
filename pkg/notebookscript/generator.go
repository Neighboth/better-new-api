package notebookscript

import (
	"fmt"
	"strings"
)

// Config holds parameters for generating a Colab/Kaggle model serving script.
type Config struct {
	Provider           string
	DeploymentName     string
	ModelRepo          string
	ModelFile          string
	TaskType           string // text, image, video, audio, embeddings
	Accelerator        string
	IdleTimeoutSeconds int
	HFToken            string
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

// Generate returns a self-contained Python serving script for Colab or Kaggle.
func Generate(cfg Config) string {
	task := NormalizeTaskType(cfg.TaskType)
	idle := cfg.IdleTimeoutSeconds
	if idle < 0 {
		idle = 0
	}

	var pipPackages string
	var taskHandler string

	switch task {
	case "video":
		pipPackages = `"fastapi", "uvicorn", "diffusers", "transformers", "accelerate", "imageio[ffmpeg]", "torch", "huggingface_hub"`
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
		pipPackages = `"fastapi", "uvicorn", "diffusers", "transformers", "accelerate", "pillow", "torch", "huggingface_hub"`
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
		pipPackages = `"fastapi", "uvicorn", "transformers", "accelerate", "scipy", "soundfile", "torch", "huggingface_hub"`
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
		pipPackages = `"fastapi", "uvicorn", "sentence-transformers", "torch", "huggingface_hub"`
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
	default: // text
		pipPackages = `"fastapi", "uvicorn", "transformers", "accelerate", "torch", "huggingface_hub"`
		taskHandler = `
from transformers import AutoModelForCausalLM, AutoTokenizer

tokenizer = AutoTokenizer.from_pretrained(MODEL_REPO, token=HF_TOKEN or None)
model = AutoModelForCausalLM.from_pretrained(
    MODEL_REPO,
    torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
    device_map="auto" if torch.cuda.is_available() else None,
    token=HF_TOKEN or None,
)

@app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    touch()
    body = await req.json()
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    prompt = "\n".join(f"{m.get('role', 'user')}: {m.get('content', '')}" for m in messages) + "\nassistant:"
    inputs = tokenizer(prompt, return_tensors="pt")
    if torch.cuda.is_available():
        inputs = {k: v.to(model.device) for k, v in inputs.items()}
    with torch.no_grad():
        output_ids = model.generate(**inputs, max_new_tokens=max_tokens)
    generated = tokenizer.decode(output_ids[0][inputs["input_ids"].shape[-1]:], skip_special_tokens=True)
    return {
        "id": "chatcmpl-notebook",
        "object": "chat.completion",
        "model": MODEL_REPO,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": generated}, "finish_reason": "stop"}],
    }
`
	}

	return fmt.Sprintf(`# Auto-generated Better-New-API %s serving script
# Deployment: %s | Model: %s | Task: %s | Accelerator: %s
import os
import subprocess
import sys
import threading
import time

subprocess.check_call([sys.executable, "-m", "pip", "install", "-q", %s])

import torch
import uvicorn
from fastapi import FastAPI, Request
from fastapi.staticfiles import StaticFiles

MODEL_REPO = %s
MODEL_FILE = %s
HF_TOKEN = %s
IDLE_TIMEOUT_SECONDS = %d
OUTPUT_DIR = "/tmp/model_outputs"
os.makedirs(OUTPUT_DIR, exist_ok=True)

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
    # Start cloudflared tunnel if available so external requests can reach port 8000
    if os.system("which cloudflared > /dev/null 2>&1") != 0:
        os.system("wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -O /usr/local/bin/cloudflared && chmod +x /usr/local/bin/cloudflared")
    subprocess.Popen(["cloudflared", "tunnel", "--url", "http://127.0.0.1:8000"])
    uvicorn.run(app, host="0.0.0.0", port=8000)
`,
		cfg.Provider,
		cfg.DeploymentName,
		cfg.ModelRepo,
		task,
		cfg.Accelerator,
		pipPackages,
		pyQuote(cfg.ModelRepo),
		pyQuote(cfg.ModelFile),
		pyQuote(cfg.HFToken),
		idle,
		pyQuote(task),
		taskHandler,
	)
}
