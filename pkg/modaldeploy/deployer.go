package modaldeploy

import (
	"bufio"
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
	CPUCores             float64
	MemoryGB             int
	IdleTimeoutSeconds   int
	TokenID              string
	TokenSecret          string
	HFToken                string
	SharedVolumePath       string
	ShutdownToken          string
	FineTunedModelID       string
	FineTunedArtifactURL   string
	FineTunedArtifactToken string
	FineTuningMethod       string
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
	resourceLines := ""
	if cfg.CPUCores > 0 {
		resourceLines += fmt.Sprintf("\n    cpu=%g,", cfg.CPUCores)
	}
	if cfg.MemoryGB > 0 {
		resourceLines += fmt.Sprintf("\n    memory=%d,", cfg.MemoryGB*1024)
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
	imageBase := `modal.Image.debian_slim(python_version="3.11")`

	switch task {
	case "video":
		pipPkgs = `"fastapi[standard]", "diffusers", "transformers", "accelerate", "imageio[ffmpeg]", "torch", "huggingface_hub", "safetensors", "omegaconf", "gguf"`
		taskCode = `
import json
import time
import torch
from diffusers import DiffusionPipeline
from huggingface_hub import hf_hub_download, list_repo_files
from diffusers.utils import export_to_video

_pipe = None
def get_pipe():
    nonlocal _pipe
    if _pipe is None:
        t0 = time.time()
        _log(f"[model] Preparing video model {MODEL_REPO} (cuda={torch.cuda.is_available()})...")
        dtype = torch.float16 if torch.cuda.is_available() else torch.float32
        entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
        main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
        explicit_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
        explicit_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
        repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
        _log(f"[model] Repo contains {len(repo_files)} files")
        has_index = "model_index.json" in repo_files
        if has_index and not explicit_file.lower().endswith((".safetensors", ".ckpt")):
            _log(f"[model] Loading diffusers pipeline from {MODEL_REPO}...")
            _pipe = DiffusionPipeline.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None, torch_dtype=dtype)
        else:
            target = explicit_file
            if not target:
                cands = [f for f in repo_files if f.lower().endswith((".safetensors", ".ckpt"))]
                if not cands:
                    raise RuntimeError(f"No model_index.json or .safetensors/.ckpt file found in {MODEL_REPO}")
                target = sorted(cands)[0]
            _log(f"[model] Downloading checkpoint {explicit_repo}/{target}...")
            local_ckpt = hf_hub_download(repo_id=explicit_repo, filename=target, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            _log(f"[model] Downloaded checkpoint ({os.path.getsize(local_ckpt) / (1024*1024):.1f} MB)")
            _pipe = None
            _v_errs = []
            for _cls_name in ("StableVideoDiffusionPipeline", "CogVideoXPipeline", "WanPipeline"):
                try:
                    import diffusers as _diff
                    _cls = getattr(_diff, _cls_name, None)
                    if _cls is not None and hasattr(_cls, "from_single_file"):
                        _log(f"[model] Trying {_cls_name}.from_single_file...")
                        _pipe = _cls.from_single_file(local_ckpt, torch_dtype=dtype)
                        break
                except Exception as _exc:
                    _v_errs.append(f"{_cls_name}: {_exc}")
            if _pipe is None:
                raise RuntimeError("Failed to load single-file video checkpoint: " + " | ".join(_v_errs))
        for entry in entries:
            role = (entry.get("role") or "extra").lower()
            fn = entry.get("file") or ""
            if not fn or role == "main":
                continue
            _log(f"[model] Downloading component ({role}): {fn}...")
            lp = hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            if role == "lora" and hasattr(_pipe, "load_lora_weights"):
                _pipe.load_lora_weights(lp)
            elif role == "vae" and hasattr(_pipe, "vae"):
                from diffusers import AutoencoderKL
                _pipe.vae = AutoencoderKL.from_single_file(lp, torch_dtype=dtype)
        if torch.cuda.is_available():
            _pipe = _pipe.to("cuda")
        _log(f"[model] Video model ready in {time.time() - t0:.1f}s")
    return _pipe

def _execute_task(body, base_url, set_progress=None):
    prompt = body.get("prompt", "A serene mountain landscape")
    num_frames = int(body.get("num_frames", 16))
    steps = int(body.get("num_inference_steps") or body.get("steps") or 20)
    if set_progress:
        set_progress(5, "Loading video model...")
    t0 = time.time()
    _log(f"[infer] Generating video (frames={num_frames})...")
    pipe = get_pipe()
    if set_progress:
        set_progress(20, "Model loaded, generating video frames...")
    def _cb(_p, step, _ts, kw):
        pct = 20 + int(((step + 1) / max(1, steps)) * 70)
        stage = f"Generating video step {step + 1}/{steps}"
        _log(f"[infer] {stage} ({min(90, pct)}%)")
        if set_progress:
            set_progress(min(90, pct), stage)
        time.sleep(0.001)
        return kw
    def _legacy_cb(step, _ts, _latents):
        pct = 20 + int(((step + 1) / max(1, steps)) * 70)
        stage = f"Generating video step {step + 1}/{steps}"
        _log(f"[infer] {stage} ({min(90, pct)}%)")
        if set_progress:
            set_progress(min(90, pct), stage)
        time.sleep(0.001)
    try:
        frames = pipe(prompt=prompt, num_frames=num_frames, num_inference_steps=steps, callback_on_step_end=_cb).frames[0]
    except TypeError:
        try:
            frames = pipe(prompt=prompt, num_frames=num_frames, num_inference_steps=steps, callback=_legacy_cb, callback_steps=1).frames[0]
        except TypeError:
            frames = pipe(prompt=prompt, num_frames=num_frames).frames[0]
    if set_progress:
        set_progress(92, "Encoding MP4 video...")
    fn = f"{uuid.uuid4().hex}.mp4"
    out = os.path.join(OUT_DIR, fn)
    export_to_video(frames, out, fps=8)
    _log(f"[infer] Video generated in {time.time() - t0:.1f}s -> {fn}")
    if set_progress:
        set_progress(100, "Completed")
    return {"data": [{"url": f"{base_url}/outputs/{fn}"}]}

@web_app.post("/v1/videos/generations")
async def gen_video(req: Request):
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
	case "image":
		isGGUFImage := strings.HasSuffix(strings.ToLower(strings.TrimSpace(cfg.ModelFile)), ".gguf") ||
			strings.Contains(strings.ToLower(cfg.ModelRepo), "gguf")
		if isGGUFImage {
			if gpuArg != "None" {
				imageBase = `modal.Image.from_registry("nvidia/cuda:12.4.1-devel-ubuntu22.04", add_python="3.11")`
				aptChain = `.apt_install("cmake", "build-essential", "libgomp1").env({"CMAKE_ARGS": "-DSD_CUDA=ON"})`
			} else {
				aptChain = `.apt_install("cmake", "build-essential", "libgomp1", "libopenblas-dev").env({"CMAKE_ARGS": "-DGGML_OPENBLAS=ON"})`
			}
			pipPkgs = `"fastapi[standard]", "pillow", "huggingface_hub", "stable-diffusion-cpp-python"`
			taskCode = `
import json
import time
from huggingface_hub import hf_hub_download, list_repo_files
from stable_diffusion_cpp import StableDiffusion

_pipe = None
def get_pipe():
    nonlocal _pipe
    if _pipe is None:
        t0 = time.time()
        _log(f"[model] Preparing GGUF image model {MODEL_REPO}...")
        entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
        main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
        explicit_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
        explicit_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
        repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
        _log(f"[model] Repo contains {len(repo_files)} files")
        target = explicit_file
        if not target:
            cands = [f for f in repo_files if f.lower().endswith(".gguf")]
            if not cands:
                raise RuntimeError(f"No .gguf file found in {MODEL_REPO}")
            target = sorted(cands)[0]
        _log(f"[model] Downloading GGUF checkpoint {explicit_repo}/{target}...")
        local_ckpt = hf_hub_download(repo_id=explicit_repo, filename=target, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
        _log(f"[model] Downloaded GGUF checkpoint ({os.path.getsize(local_ckpt) / (1024*1024):.1f} MB)")
        sd_kw = {}
        for entry in entries:
            role = (entry.get("role") or "extra").lower()
            fn = entry.get("file") or ""
            if not fn or role == "main":
                continue
            _log(f"[model] Downloading component ({role}): {fn}...")
            lp = hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            if role == "vae":
                sd_kw["vae_path"] = lp
            elif role == "text_encoder":
                sd_kw["clip_l_path"] = lp
            elif role == "text_encoder_2":
                sd_kw["t5xxl_path"] = lp
            elif role == "lora":
                sd_kw["lora_model_dir"] = os.path.dirname(lp)
        _log("[model] Initializing stable-diffusion.cpp engine...")
        try:
            sd = StableDiffusion(model_path=local_ckpt, **sd_kw)
        except Exception as exc1:
            _log(f"[model] Retrying with diffusion_model_path ({exc1})...")
            sd = StableDiffusion(diffusion_model_path=local_ckpt, **sd_kw)

        class _GGUFImageWrapper:
            def __init__(self, engine):
                self._engine = engine
            def __call__(self, prompt: str, width: int = 512, height: int = 512, num_inference_steps: int = 20, progress_callback=None):
                imgs = self._engine.generate_image(prompt=prompt, width=width, height=height, sample_steps=num_inference_steps, progress_callback=progress_callback)
                return type("_Out", (), {"images": imgs})()

        _pipe = _GGUFImageWrapper(sd)
        _log(f"[model] GGUF image model ready in {time.time() - t0:.1f}s")
    return _pipe

def _execute_task(body, base_url, set_progress=None):
    prompt = body.get("prompt", "A scenic landscape")
    width = int(body.get("width") or 512)
    height = int(body.get("height") or 512)
    steps = int(body.get("num_inference_steps") or body.get("steps") or 20)
    if set_progress:
        set_progress(5, "Loading GGUF image model...")
    t0 = time.time()
    _log("[infer] Generating image (GGUF)...")
    pipe = get_pipe()
    if set_progress:
        set_progress(20, "Model loaded, generating image...")
    def _on_step(step, total, _t):
        pct = 20 + int((step / max(1, total)) * 75)
        stage = f"Generating image step {step}/{total}"
        _log(f"[infer] {stage} ({min(95, pct)}%)")
        if set_progress:
            set_progress(min(95, pct), stage)
        time.sleep(0.001)
    img = pipe(prompt=prompt, width=width, height=height, num_inference_steps=steps, progress_callback=_on_step).images[0]
    fn = f"{uuid.uuid4().hex}.png"
    out = os.path.join(OUT_DIR, fn)
    img.save(out)
    _log(f"[infer] Image generated in {time.time() - t0:.1f}s -> {fn}")
    if set_progress:
        set_progress(100, "Completed")
    return {"data": [{"url": f"{base_url}/outputs/{fn}"}]}

@web_app.post("/v1/images/generations")
async def gen_image(req: Request):
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
		} else {
			pipPkgs = `"fastapi[standard]", "diffusers", "transformers", "accelerate", "pillow", "torch", "huggingface_hub", "safetensors", "omegaconf", "gguf"`
			taskCode = `
import json
import time
import torch
from diffusers import AutoPipelineForText2Image
from huggingface_hub import hf_hub_download, list_repo_files

_pipe = None
def get_pipe():
    nonlocal _pipe
    if _pipe is None:
        t0 = time.time()
        _log(f"[model] Preparing image model {MODEL_REPO} (cuda={torch.cuda.is_available()})...")
        dtype = torch.float16 if torch.cuda.is_available() else torch.float32
        entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
        main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
        explicit_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
        explicit_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
        repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
        _log(f"[model] Repo contains {len(repo_files)} files")
        has_index = "model_index.json" in repo_files
        if has_index and not explicit_file.lower().endswith((".safetensors", ".ckpt", ".gguf")):
            _log(f"[model] Loading AutoPipelineForText2Image from {MODEL_REPO}...")
            _pipe = AutoPipelineForText2Image.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, token=HF_TOKEN or None, torch_dtype=dtype)
        else:
            target = explicit_file
            if not target:
                cands = [f for f in repo_files if f.lower().endswith((".safetensors", ".ckpt", ".gguf"))]
                if not cands:
                    raise RuntimeError(f"No model_index.json or .safetensors/.ckpt/.gguf file found in {MODEL_REPO}")
                target = sorted(cands)[0]
            _log(f"[model] Downloading checkpoint {explicit_repo}/{target}...")
            local_ckpt = hf_hub_download(repo_id=explicit_repo, filename=target, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
            _log(f"[model] Downloaded checkpoint ({os.path.getsize(local_ckpt) / (1024*1024):.1f} MB)")
            from diffusers import AutoencoderKL, StableDiffusionPipeline, StableDiffusionXLPipeline
            from transformers import CLIPTextModel, CLIPTextModelWithProjection, CLIPTokenizer
            errs = []
            try:
                _log("[model] Trying StableDiffusionPipeline.from_single_file...")
                _pipe = StableDiffusionPipeline.from_single_file(local_ckpt, torch_dtype=dtype)
            except Exception as exc:
                msg = str(exc)
                errs.append(f"StableDiffusionPipeline: {msg}")
                if "CLIPTextModel" in msg or "text_encoder" in msg or "AutoencoderKL" in msg or "vae" in msg:
                    try:
                        kw = {}
                        if "CLIPTextModel" in msg or "text_encoder" in msg:
                            kw["text_encoder"] = CLIPTextModel.from_pretrained("openai/clip-vit-large-patch14", cache_dir=CACHE_DIR, torch_dtype=dtype)
                            kw["tokenizer"] = CLIPTokenizer.from_pretrained("openai/clip-vit-large-patch14", cache_dir=CACHE_DIR)
                        if "AutoencoderKL" in msg or "vae" in msg:
                            kw["vae"] = AutoencoderKL.from_pretrained("stabilityai/sd-vae-ft-mse", cache_dir=CACHE_DIR, torch_dtype=dtype)
                        _pipe = StableDiffusionPipeline.from_single_file(local_ckpt, torch_dtype=dtype, **kw)
                    except Exception as exc2:
                        errs.append(f"StableDiffusionPipeline+fallback: {exc2}")
            if _pipe is None:
                try:
                    _log("[model] Trying StableDiffusionXLPipeline.from_single_file...")
                    _pipe = StableDiffusionXLPipeline.from_single_file(local_ckpt, torch_dtype=dtype)
                except Exception as exc:
                    msg = str(exc)
                    errs.append(f"StableDiffusionXLPipeline: {msg}")
                    if "CLIPTextModel" in msg or "text_encoder" in msg:
                        try:
                            kw = {
                                "text_encoder": CLIPTextModel.from_pretrained("openai/clip-vit-large-patch14", cache_dir=CACHE_DIR, torch_dtype=dtype),
                                "tokenizer": CLIPTokenizer.from_pretrained("openai/clip-vit-large-patch14", cache_dir=CACHE_DIR),
                                "text_encoder_2": CLIPTextModelWithProjection.from_pretrained("laion/CLIP-ViT-bigG-14-laion2B-39B-b160k", cache_dir=CACHE_DIR, torch_dtype=dtype),
                                "tokenizer_2": CLIPTokenizer.from_pretrained("laion/CLIP-ViT-bigG-14-laion2B-39B-b160k", cache_dir=CACHE_DIR),
                            }
                            _pipe = StableDiffusionXLPipeline.from_single_file(local_ckpt, torch_dtype=dtype, **kw)
                        except Exception as exc2:
                            errs.append(f"StableDiffusionXLPipeline+fallback: {exc2}")
            if _pipe is None:
                try:
                    import diffusers as _diff
                    _flux_cls = getattr(_diff, "FluxPipeline", None)
                    if _flux_cls is not None and hasattr(_flux_cls, "from_single_file"):
                        _log("[model] Trying FluxPipeline.from_single_file...")
                        _pipe = _flux_cls.from_single_file(local_ckpt, torch_dtype=dtype)
                except Exception as exc:
                    errs.append(f"FluxPipeline: {exc}")
            if _pipe is None:
                raise RuntimeError("Failed to load single-file image checkpoint: " + " | ".join(errs))
        for entry in entries:
            role = (entry.get("role") or "extra").lower()
            fn = entry.get("file") or ""
            if not fn or role == "main":
                continue
            _log(f"[model] Downloading component ({role}): {fn}...")
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
        _log(f"[model] Image model ready in {time.time() - t0:.1f}s")
    return _pipe

def _execute_task(body, base_url, set_progress=None):
    prompt = body.get("prompt", "A scenic landscape")
    steps = int(body.get("num_inference_steps") or body.get("steps") or 20)
    if set_progress:
        set_progress(5, "Loading image model...")
    t0 = time.time()
    _log("[infer] Generating image...")
    pipe = get_pipe()
    if set_progress:
        set_progress(20, "Model loaded, generating image...")
    def _cb(_p, step, _ts, kw):
        pct = 20 + int(((step + 1) / max(1, steps)) * 75)
        stage = f"Generating image step {step + 1}/{steps}"
        _log(f"[infer] {stage} ({min(95, pct)}%)")
        if set_progress:
            set_progress(min(95, pct), stage)
        time.sleep(0.001)
        return kw
    def _legacy_cb(step, _ts, _latents):
        pct = 20 + int(((step + 1) / max(1, steps)) * 75)
        stage = f"Generating image step {step + 1}/{steps}"
        _log(f"[infer] {stage} ({min(95, pct)}%)")
        if set_progress:
            set_progress(min(95, pct), stage)
        time.sleep(0.001)
    try:
        img = pipe(prompt=prompt, num_inference_steps=steps, callback_on_step_end=_cb).images[0]
    except TypeError:
        try:
            img = pipe(prompt=prompt, num_inference_steps=steps, callback=_legacy_cb, callback_steps=1).images[0]
        except TypeError:
            img = pipe(prompt=prompt).images[0]
    fn = f"{uuid.uuid4().hex}.png"
    out = os.path.join(OUT_DIR, fn)
    img.save(out)
    _log(f"[infer] Image generated in {time.time() - t0:.1f}s -> {fn}")
    if set_progress:
        set_progress(100, "Completed")
    return {"data": [{"url": f"{base_url}/outputs/{fn}"}]}

@web_app.post("/v1/images/generations")
async def gen_image(req: Request):
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
		}
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
        t0 = time.time()
        _log(f"[model] Loading audio model {MODEL_REPO} (cuda={torch.cuda.is_available()})...")
        _pipe = hf_pipeline("text-to-audio", model=MODEL_REPO, device=0 if torch.cuda.is_available() else -1, token=HF_TOKEN or None)
        _log(f"[model] Audio model ready in {time.time() - t0:.1f}s")
    return _pipe

def _execute_task(body, base_url, set_progress=None):
    text = body.get("input") or body.get("prompt") or "Hello world"
    if set_progress:
        set_progress(5, "Loading audio model...")
    t0 = time.time()
    _log("[infer] Generating audio...")
    pipe = get_pipe()
    if set_progress:
        set_progress(40, "Synthesizing audio...")
    out = pipe(text)
    fn = f"{uuid.uuid4().hex}.wav"
    out_path = os.path.join(OUT_DIR, fn)
    wavfile.write(out_path, rate=int(out.get("sampling_rate", 16000)), data=out["audio"])
    _log(f"[infer] Audio generated in {time.time() - t0:.1f}s -> {fn}")
    if set_progress:
        set_progress(100, "Completed")
    return {"url": f"{base_url}/outputs/{fn}", "data": [{"url": f"{base_url}/outputs/{fn}"}]}

@web_app.post("/v1/audio/speech")
@web_app.post("/v1/audio/generations")
async def gen_audio(req: Request):
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
	case "embeddings":
		pipPkgs = `"fastapi[standard]", "sentence-transformers", "torch", "huggingface_hub"`
		taskCode = `
from sentence_transformers import SentenceTransformer

_model = None
def get_model():
    nonlocal _model
    if _model is None:
        t0 = time.time()
        _log(f"[model] Loading embeddings model {MODEL_REPO}...")
        _model = SentenceTransformer(MODEL_REPO, cache_folder=CACHE_DIR, token=HF_TOKEN or None)
        _log(f"[model] Embeddings model ready in {time.time() - t0:.1f}s")
    return _model

def _execute_task(body, _base_url, set_progress=None):
    inp = body.get("input", "")
    texts = inp if isinstance(inp, list) else [str(inp)]
    if set_progress:
        set_progress(5, "Loading embeddings model...")
    t0 = time.time()
    mdl = get_model()
    if set_progress:
        set_progress(40, "Encoding embeddings...")
    vecs = mdl.encode(texts).tolist()
    _log(f"[infer] Encoded {len(texts)} input(s) in {time.time() - t0:.2f}s")
    if set_progress:
        set_progress(100, "Completed")
    return {"object": "list", "model": MODEL_REPO, "data": [{"object": "embedding", "index": i, "embedding": v} for i, v in enumerate(vecs)]}

@web_app.post("/v1/embeddings")
async def embeddings(req: Request):
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
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
    t0 = time.time()
    entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
    main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
    target_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
    target_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
    for entry in entries:
        fn = entry.get("file") or ""
        if fn and fn != target_file:
            _log(f"[model] Downloading auxiliary file {fn}...")
            hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
    if not target_file:
        files = [f for f in list_repo_files(MODEL_REPO, token=HF_TOKEN or None) if f.lower().endswith(".gguf")]
        if not files:
            raise RuntimeError(f"No .gguf file found in {MODEL_REPO}")
        target_file = sorted(files)[0]
    _log(f"[model] Downloading GGUF {target_repo}/{target_file}...")
    local_path = hf_hub_download(repo_id=target_repo, filename=target_file, cache_dir=CACHE_DIR, token=HF_TOKEN or None)
    n_gpu = -1 if os.path.exists("/dev/nvidia0") else 0
    _log(f"[model] Loading GGUF ({os.path.getsize(local_path)/(1024*1024):.1f} MB, gpu_layers={n_gpu})...")
    _llm = Llama(model_path=local_path, n_ctx=4096, n_gpu_layers=n_gpu, verbose=False)
    _log(f"[model] GGUF model ready in {time.time() - t0:.1f}s")
    return _llm

def _execute_task(body, _base_url, set_progress=None):
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    if set_progress:
        set_progress(5, "Loading GGUF model...")
    t0 = time.time()
    llm = get_llm()
    if set_progress:
        set_progress(40, "Generating text response...")
    out = llm.create_chat_completion(messages=messages, max_tokens=max_tokens)
    _log(f"[infer] GGUF chat completion done in {time.time() - t0:.2f}s")
    if set_progress:
        set_progress(100, "Completed")
    return out

@web_app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
		} else {
			if strings.TrimSpace(cfg.FineTunedArtifactURL) != "" {
				pipPkgs = `"fastapi[standard]", "transformers", "peft", "safetensors", "accelerate", "torch", "huggingface_hub"`
			} else {
				pipPkgs = `"fastapi[standard]", "transformers", "accelerate", "torch", "huggingface_hub"`
			}
			taskCode = `
import urllib.request
import zipfile
import torch
from fastapi.responses import StreamingResponse
from transformers import AutoModelForCausalLM, AutoTokenizer, TextIteratorStreamer

_backend = None
_backend_lock = threading.Lock()
def _prepare_ft_weights():
    if not FT_ARTIFACT_URL:
        return None
    ft_dir = "/tmp/ft_weights"
    marker = os.path.join(ft_dir, ".ready")
    if os.path.exists(marker):
        return ft_dir
    os.makedirs(ft_dir, exist_ok=True)
    zip_path = "/tmp/ft_model.zip"
    _log("[model] Downloading fine-tuned model archive...")
    headers = {"User-Agent": "BetterNewAPI-Modal/1.0"}
    if FT_ARTIFACT_TOKEN:
        headers["Authorization"] = f"Bearer {FT_ARTIFACT_TOKEN}"
    req = urllib.request.Request(FT_ARTIFACT_URL, headers=headers)
    with urllib.request.urlopen(req, timeout=600) as resp, open(zip_path, "wb") as out_f:
        while True:
            chunk = resp.read(1024 * 1024)
            if not chunk:
                break
            out_f.write(chunk)
    with zipfile.ZipFile(zip_path, "r") as zf:
        zf.extractall(ft_dir)
    try:
        os.remove(zip_path)
    except Exception:
        pass
    with open(marker, "w", encoding="utf-8") as mf:
        mf.write("ok")
    return ft_dir

def _load_causal_fast(src, dtype, use_cuda, use_cache_dir=True, token=None):
    kw = {"trust_remote_code": True, "low_cpu_mem_usage": True, "torch_dtype": dtype}
    if use_cache_dir:
        kw["cache_dir"] = CACHE_DIR
    if token:
        kw["token"] = token
    if use_cuda and torch.cuda.device_count() == 1:
        try:
            m = AutoModelForCausalLM.from_pretrained(src, **kw)
            return m.to("cuda")
        except torch.cuda.OutOfMemoryError:
            torch.cuda.empty_cache()
        except Exception:
            pass
    if use_cuda:
        kw["device_map"] = "auto"
    return AutoModelForCausalLM.from_pretrained(src, **kw)

def get_backend():
    nonlocal _backend
    if _backend is not None:
        return _backend
    with _backend_lock:
        if _backend is not None:
            return _backend
        t0 = time.time()
        use_cuda = torch.cuda.is_available()
        if use_cuda:
            try:
                torch.backends.cuda.matmul.allow_tf32 = True
                torch.backends.cudnn.benchmark = True
            except Exception:
                pass
        dtype = torch.bfloat16 if (use_cuda and torch.cuda.is_bf16_supported()) else (torch.float16 if use_cuda else torch.float32)
        ft_dir = _prepare_ft_weights()
        if ft_dir:
            has_adapter = os.path.exists(os.path.join(ft_dir, "adapter_config.json"))
            tok_src = ft_dir if (os.path.exists(os.path.join(ft_dir, "tokenizer_config.json")) or os.path.exists(os.path.join(ft_dir, "tokenizer.json"))) else MODEL_REPO
            _log(f"[model] Loading fine-tuned model ({SERVED_MODEL_ID}, adapter={has_adapter})...")
            tok = AutoTokenizer.from_pretrained(tok_src, cache_dir=CACHE_DIR, trust_remote_code=True, token=HF_TOKEN or None)
            if has_adapter:
                from peft import PeftModel
                base_mdl = _load_causal_fast(MODEL_REPO, dtype, use_cuda, use_cache_dir=True, token=HF_TOKEN or None)
                mdl = PeftModel.from_pretrained(base_mdl, ft_dir)
                try:
                    mdl = mdl.merge_and_unload()
                except Exception:
                    pass
                if use_cuda and getattr(mdl, "device", None) is not None and mdl.device.type != "cuda":
                    try:
                        mdl = mdl.to("cuda")
                    except Exception:
                        pass
            else:
                mdl = _load_causal_fast(ft_dir, dtype, use_cuda, use_cache_dir=False, token=None)
        else:
            _log(f"[model] Loading transformers model {MODEL_REPO} (cuda={use_cuda})...")
            tok = AutoTokenizer.from_pretrained(MODEL_REPO, cache_dir=CACHE_DIR, trust_remote_code=True, token=HF_TOKEN or None)
            mdl = _load_causal_fast(MODEL_REPO, dtype, use_cuda, use_cache_dir=True, token=HF_TOKEN or None)
        if tok.pad_token is None and tok.eos_token is not None:
            tok.pad_token = tok.eos_token
        mdl.config.use_cache = True
        mdl.eval()
        try:
            _w = tok("hi", return_tensors="pt")
            if use_cuda:
                _w = {k: v.to(mdl.device) for k, v in _w.items()}
            with torch.inference_mode():
                _ = mdl.generate(**_w, max_new_tokens=1, do_sample=False, use_cache=True, pad_token_id=tok.pad_token_id or tok.eos_token_id)
            if use_cuda:
                torch.cuda.synchronize()
        except Exception:
            pass
        _backend = (tok, mdl)
        _log(f"[model] Model loaded & warmed in {time.time() - t0:.1f}s")
        return _backend

threading.Thread(target=lambda: get_backend(), daemon=True).start()

def _prepare_modal_gen(body, tok, mdl):
    messages = body.get("messages", [])
    max_tokens = max(1, min(4096, int(body.get("max_tokens") or body.get("max_completion_tokens") or 512)))
    temperature = body.get("temperature")
    top_p = body.get("top_p")
    clean_msgs = [{"role": str(m.get("role", "user")), "content": str(m.get("content", "") or "")} for m in messages if isinstance(m, dict)]
    prompt = None
    if hasattr(tok, "apply_chat_template") and getattr(tok, "chat_template", None):
        try:
            prompt = tok.apply_chat_template(clean_msgs, tokenize=False, add_generation_prompt=True)
        except Exception:
            prompt = None
    if prompt is None:
        prompt = "\n".join(f"<|{m['role']}|>\n{m['content']}" for m in clean_msgs) + "\n<|assistant|>\n"
    inputs = tok(prompt, return_tensors="pt")
    if torch.cuda.is_available():
        inputs = {k: v.to(mdl.device) for k, v in inputs.items()}
    gen_kwargs = {"max_new_tokens": max_tokens, "use_cache": True, "pad_token_id": tok.pad_token_id or tok.eos_token_id}
    if temperature is not None and float(temperature) > 0:
        gen_kwargs["do_sample"] = True
        gen_kwargs["temperature"] = max(0.01, min(2.0, float(temperature)))
        if top_p is not None:
            gen_kwargs["top_p"] = max(0.05, min(1.0, float(top_p)))
    else:
        gen_kwargs["do_sample"] = False
    return inputs, gen_kwargs

def _execute_task(body, _base_url, set_progress=None):
    if set_progress:
        set_progress(5, "Loading transformers model...")
    t0 = time.time()
    tok, mdl = get_backend()
    if set_progress:
        set_progress(40, "Generating text response...")
    inputs, gen_kwargs = _prepare_modal_gen(body, tok, mdl)
    with torch.inference_mode():
        out_ids = mdl.generate(**inputs, **gen_kwargs)
    prompt_tokens = int(inputs["input_ids"].shape[-1])
    comp_ids = out_ids[0][prompt_tokens:]
    text = tok.decode(comp_ids, skip_special_tokens=True)
    resp_model = str(body.get("model") or SERVED_MODEL_ID)
    _log(f"[infer] Chat completion ({len(comp_ids)} tok) in {time.time() - t0:.2f}s")
    if set_progress:
        set_progress(100, "Completed")
    return {
        "id": f"chatcmpl-{int(time.time() * 1000)}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": resp_model,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}],
        "usage": {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": int(len(comp_ids)),
            "total_tokens": prompt_tokens + int(len(comp_ids)),
        },
    }

@web_app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    body = await req.json()
    if body.get("stream"):
        import json as _json
        tok, mdl = get_backend()
        inputs, gen_kwargs = _prepare_modal_gen(body, tok, mdl)
        prompt_tokens = int(inputs["input_ids"].shape[-1])
        cid = f"chatcmpl-{int(time.time() * 1000)}"
        created = int(time.time())
        resp_model = str(body.get("model") or SERVED_MODEL_ID)
        streamer = TextIteratorStreamer(tok, skip_prompt=True, skip_special_tokens=True)
        gen_kwargs["streamer"] = streamer

        def _run_gen():
            try:
                with torch.inference_mode():
                    mdl.generate(**inputs, **gen_kwargs)
            except Exception:
                try:
                    streamer.end()
                except Exception:
                    pass

        threading.Thread(target=_run_gen, daemon=True).start()

        def _sse():
            c0 = {"id": cid, "object": "chat.completion.chunk", "created": created, "model": resp_model, "choices": [{"index": 0, "delta": {"role": "assistant", "content": ""}, "finish_reason": None}]}
            yield f"data: {_json.dumps(c0, ensure_ascii=False)}\n\n"
            comp_chars = 0
            for piece in streamer:
                if not piece:
                    continue
                comp_chars += len(piece)
                c1 = {"id": cid, "object": "chat.completion.chunk", "created": created, "model": resp_model, "choices": [{"index": 0, "delta": {"content": piece}, "finish_reason": None}]}
                yield f"data: {_json.dumps(c1, ensure_ascii=False)}\n\n"
            est_comp = max(1, comp_chars // 4) if comp_chars > 0 else 0
            c2 = {"id": cid, "object": "chat.completion.chunk", "created": created, "model": resp_model, "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}], "usage": {"prompt_tokens": prompt_tokens, "completion_tokens": est_comp, "total_tokens": prompt_tokens + est_comp}}
            yield f"data: {_json.dumps(c2, ensure_ascii=False)}\n\n"
            yield "data: [DONE]\n\n"

        return StreamingResponse(_sse(), media_type="text/event-stream", headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"})
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
		}
	}

	servedModelID := strings.TrimSpace(cfg.FineTunedModelID)
	if servedModelID == "" {
		servedModelID = cfg.ModelRepo
	}

	return fmt.Sprintf(`import os
import threading
import time
import uuid
import modal

app = modal.App(%s)
image = %s%s.pip_install(%s%s)
volume = modal.Volume.from_name("better-new-api-models", create_if_missing=True)

MODEL_REPO = %s
SERVED_MODEL_ID = %s
FT_ARTIFACT_URL = %s
FT_ARTIFACT_TOKEN = %s
MODEL_FILE = %s
MODEL_FILES_JSON = %s
HF_TOKEN = %s
CACHE_DIR = %s
SHUTDOWN_TOKEN = %s
OUT_DIR = "/tmp/model_outputs"

@app.function(
    image=image,
    gpu=%s,
    volumes={CACHE_DIR: volume},
    timeout=3600,%s%s
)
@modal.asgi_app()
def fastapi_app():
    from fastapi import FastAPI, HTTPException, Request
    from fastapi.responses import JSONResponse
    from fastapi.staticfiles import StaticFiles

    os.makedirs(OUT_DIR, exist_ok=True)
    web_app = FastAPI(title="Better-New-API Modal Server")
    web_app.mount("/outputs", StaticFiles(directory=OUT_DIR), name="outputs")
    _log_lines = []
    _jobs = {}
    _jobs_lock = threading.Lock()

    def _log(msg: str):
        line = f"{time.strftime('%%H:%%M:%%S')} {msg}"
        print(line, flush=True)
        _log_lines.append(line)
        if len(_log_lines) > 500:
            del _log_lines[:len(_log_lines) - 500]

    @web_app.exception_handler(Exception)
    async def unhandled_exception_handler(request: Request, exc: Exception):
        _log(f"[error] {type(exc).__name__}: {exc}")
        return JSONResponse(status_code=500, content={"error": f"{type(exc).__name__}: {exc}"})

    @web_app.get("/health")
    def health():
        return {"status": "ok", "model": SERVED_MODEL_ID, "task": %s}

    @web_app.get("/_logs")
    def get_logs(req: Request):
        tok = req.query_params.get("token") or req.headers.get("X-Shutdown-Token") or ""
        if SHUTDOWN_TOKEN and tok != SHUTDOWN_TOKEN:
            raise HTTPException(status_code=403, detail="forbidden")
        return {"logs": "\n".join(_log_lines)}
%s
    @web_app.post("/_jobs")
    async def start_job(req: Request):
        body = await req.json()
        base_url = str(req.base_url).rstrip("/")
        job_id = uuid.uuid4().hex
        now = time.time()
        with _jobs_lock:
            stale = [k for k, v in _jobs.items() if now - v.get("updated_at", now) > 3600]
            for k in stale:
                del _jobs[k]
            _jobs[job_id] = {"job_id": job_id, "status": "queued", "progress": 0, "stage": "Queued...", "result": None, "error": None, "updated_at": now}

        def _set_progress(pct: int, stage: str):
            with _jobs_lock:
                if job_id in _jobs:
                    _jobs[job_id]["status"] = "running"
                    _jobs[job_id]["progress"] = max(0, min(100, int(pct)))
                    _jobs[job_id]["stage"] = stage
                    _jobs[job_id]["updated_at"] = time.time()

        def _worker():
            try:
                _set_progress(1, "Starting...")
                res = _execute_task(body, base_url, _set_progress)
                with _jobs_lock:
                    if job_id in _jobs:
                        _jobs[job_id]["status"] = "completed"
                        _jobs[job_id]["progress"] = 100
                        _jobs[job_id]["stage"] = "Completed"
                        _jobs[job_id]["result"] = res
                        _jobs[job_id]["updated_at"] = time.time()
            except Exception as exc:
                _log(f"[job] Failed: {type(exc).__name__}: {exc}")
                with _jobs_lock:
                    if job_id in _jobs:
                        _jobs[job_id]["status"] = "failed"
                        _jobs[job_id]["stage"] = "Failed"
                        _jobs[job_id]["error"] = f"{type(exc).__name__}: {exc}"
                        _jobs[job_id]["updated_at"] = time.time()

        threading.Thread(target=_worker, daemon=True).start()
        return {"job_id": job_id, "status": "queued", "progress": 0, "stage": "Queued..."}

    @web_app.get("/_jobs/{job_id}")
    async def get_job(job_id: str):
        with _jobs_lock:
            job = _jobs.get(job_id)
            if not job:
                raise HTTPException(status_code=404, detail="job not found")
            return {k: v for k, v in job.items() if k != "updated_at"}

    return web_app
`,
		pyQuote(cfg.AppName),
		imageBase,
		aptChain,
		pipPkgs,
		extraPipOpts,
		pyQuote(cfg.ModelRepo),
		pyQuote(servedModelID),
		pyQuote(cfg.FineTunedArtifactURL),
		pyQuote(cfg.FineTunedArtifactToken),
		pyQuote(cfg.ModelFile),
		pyQuote(modelFilesJSON),
		pyQuote(cfg.HFToken),
		pyQuote(volPath),
		pyQuote(cfg.ShutdownToken),
		gpuArg,
		scaledownLine,
		resourceLines,
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
		return modalCmd{}, fmt.Errorf("failed to auto-install 'modal' Python package: %v: %s", err, msg)
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
	return DeployWithLogger(ctx, cfg, nil)
}

// DeployWithLogger deploys the Modal ASGI app using `modal deploy`, streaming
// each clean output line to onLog as it is produced, and returns the *.modal.run URL.
func DeployWithLogger(ctx context.Context, cfg DeployConfig, onLog func(string)) (string, string, error) {
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

	stdout, pipeErr := cmd.StdoutPipe()
	var outBuf strings.Builder
	var runErr error
	if pipeErr == nil {
		cmd.Stderr = cmd.Stdout
		if startErr := cmd.Start(); startErr != nil {
			runErr = startErr
		} else {
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
			for scanner.Scan() {
				rawLine := scanner.Text()
				outBuf.WriteString(rawLine)
				outBuf.WriteByte('\n')
				clean := strings.TrimSpace(ansiEscapeRegex.ReplaceAllString(rawLine, ""))
				if clean != "" && onLog != nil {
					onLog(clean)
				}
			}
			runErr = cmd.Wait()
		}
	} else {
		var outBytes []byte
		outBytes, runErr = cmd.CombinedOutput()
		outBuf.Write(outBytes)
	}

	outText := outBuf.String()
	if runErr != nil {
		msg := strings.TrimSpace(outText)
		if len(msg) > 600 {
			msg = msg[len(msg)-600:]
		}
		return "", script, fmt.Errorf("modal deploy failed: %v: %s", runErr, msg)
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
	cmd := cmdSpec.command(ctx, "app", "stop", "--yes", trimmed)
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
					stopByID := cmdSpec.command(ctx, "app", "stop", "--yes", appID)
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
	return fmt.Errorf("modal app stop failed: %v: %s", stopErr, msg)
}

// FineTuneConfig configures a one-shot real PyTorch fine-tuning job on Modal.
type FineTuneConfig struct {
	AppName              string
	Accelerator          string
	TokenID              string
	TokenSecret          string
	SharedVolumePath     string
	TrainScript          string
	DatasetPath          string
	DatasetName          string
	BaseModelPath        string // optional local uploaded weights file
	ResumeCheckpointPath string // optional local checkpoint_latest.zip to resume from
	OutputZipPath        string // local destination path where trained .zip is written
}

// RunFineTuneWithLogger executes a real GPU/CPU PyTorch fine-tuning script on Modal
// via `modal run`, streaming container logs live to onLog and saving the trained
// .zip archive directly to cfg.OutputZipPath.
func RunFineTuneWithLogger(ctx context.Context, cfg FineTuneConfig, onLog func(string)) error {
	cmdSpec, err := findOrBootstrapModalCommand(ctx)
	if err != nil {
		return err
	}
	acc := strings.TrimSpace(cfg.Accelerator)
	gpuArg := "None"
	if acc != "" && !strings.EqualFold(acc, "CPU") {
		gpuArg = pyQuote(acc)
	}
	volPath := strings.TrimSpace(cfg.SharedVolumePath)
	if volPath == "" {
		volPath = "/vol/models"
	}
	appName := strings.TrimSpace(cfg.AppName)
	if appName == "" {
		appName = "newapi-finetune"
	}

	tmpDir, err := os.MkdirTemp("", "modal-ft-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	trainPyPath := filepath.Join(tmpDir, "train_inner.py")
	if err := os.WriteFile(trainPyPath, []byte(cfg.TrainScript), 0o600); err != nil {
		return err
	}

	wrapper := fmt.Sprintf(`import os
import subprocess
import sys
from pathlib import Path
import modal

app = modal.App(%s)
vol = modal.Volume.from_name("newapi-model-cache", create_if_missing=True)

image = (
    modal.Image.debian_slim(python_version="3.11")
    .pip_install(
        "torch",
        "transformers",
        "peft",
        "accelerate",
        "safetensors",
        "datasets",
        "huggingface_hub",
        "bitsandbytes",
        "diffusers",
        "Pillow",
    )
)

@app.function(
    image=image,
    gpu=%s,
    timeout=14400,
    volumes={%s: vol},
)
def run_train_remote(train_script: str, dataset_bytes: bytes, dataset_name: str, base_model_bytes: bytes, base_model_name: str, resume_ckpt_bytes: bytes) -> bytes:
    os.environ["HF_HOME"] = %s + "/hf_cache"
    work = Path("/tmp/ft_run")
    work.mkdir(parents=True, exist_ok=True)
    os.chdir(work)
    script_file = work / "train.py"
    script_file.write_text(train_script, encoding="utf-8")

    # Patch local dataset, optional base model, and optional resume checkpoint paths
    if dataset_bytes:
        ds_local = work / (dataset_name or "dataset.jsonl")
        ds_local.write_bytes(dataset_bytes)
        os.environ["FT_LOCAL_DATASET"] = str(ds_local)
    if base_model_bytes:
        bm_local = work / (base_model_name or "base_model.safetensors")
        bm_local.write_bytes(base_model_bytes)
        os.environ["FT_LOCAL_BASE_MODEL"] = str(bm_local)
    if resume_ckpt_bytes:
        ck_local = work / "resume_checkpoint.zip"
        ck_local.write_bytes(resume_ckpt_bytes)
        os.environ["FT_LOCAL_RESUME_CHECKPOINT"] = str(ck_local)

    proc = subprocess.Popen(
        [sys.executable, "-u", str(script_file)],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )
    assert proc.stdout is not None
    for line in proc.stdout:
        print(line, end="", flush=True)
    rc = proc.wait()
    if rc != 0:
        raise RuntimeError(f"Fine-tuning process exited with code {rc}")
    out_zip = work / "output.zip"
    if not out_zip.exists():
        raise RuntimeError("Fine-tuning completed without producing output.zip")
    return out_zip.read_bytes()

@app.local_entrypoint()
def main():
    script_txt = Path(%s).read_text(encoding="utf-8")
    ds_path = %s
    ds_bytes = Path(ds_path).read_bytes() if (ds_path and os.path.exists(ds_path)) else b""
    bm_path = %s
    bm_bytes = Path(bm_path).read_bytes() if (bm_path and os.path.exists(bm_path)) else b""
    bm_name = os.path.basename(bm_path) if bm_path else ""
    ck_path = %s
    ck_bytes = Path(ck_path).read_bytes() if (ck_path and os.path.exists(ck_path)) else b""
    zip_bytes = run_train_remote.remote(script_txt, ds_bytes, %s, bm_bytes, bm_name, ck_bytes)
    out_path = Path(%s)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_bytes(zip_bytes)
    print(f"[modal-ft] Saved trained archive ({len(zip_bytes)} bytes)", flush=True)
`,
		pyQuote(appName),
		gpuArg,
		pyQuote(volPath),
		pyQuote(volPath),
		pyQuote(trainPyPath),
		pyQuote(cfg.DatasetPath),
		pyQuote(cfg.BaseModelPath),
		pyQuote(cfg.ResumeCheckpointPath),
		pyQuote(cfg.DatasetName),
		pyQuote(cfg.OutputZipPath),
	)

	wrapperPath := filepath.Join(tmpDir, "modal_ft_app.py")
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o600); err != nil {
		return err
	}

	cmd := cmdSpec.command(ctx, "run", wrapperPath)
	cmd.Env = append(os.Environ(),
		"MODAL_TOKEN_ID="+strings.TrimSpace(cfg.TokenID),
		"MODAL_TOKEN_SECRET="+strings.TrimSpace(cfg.TokenSecret),
		"PYTHONIOENCODING=utf-8",
		"COLUMNS=500",
		"NO_COLOR=1",
		"FORCE_COLOR=0",
		"TERM=dumb",
	)

	stdout, pipeErr := cmd.StdoutPipe()
	var outBuf strings.Builder
	var runErr error
	if pipeErr == nil {
		cmd.Stderr = cmd.Stdout
		if startErr := cmd.Start(); startErr != nil {
			runErr = startErr
		} else {
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
			for scanner.Scan() {
				rawLine := scanner.Text()
				outBuf.WriteString(rawLine)
				outBuf.WriteByte('\n')
				clean := strings.TrimSpace(ansiEscapeRegex.ReplaceAllString(rawLine, ""))
				if clean != "" && onLog != nil {
					onLog(clean)
				}
			}
			runErr = cmd.Wait()
		}
	} else {
		var outBytes []byte
		outBytes, runErr = cmd.CombinedOutput()
		outBuf.Write(outBytes)
	}

	if runErr != nil {
		msg := strings.TrimSpace(outBuf.String())
		if len(msg) > 600 {
			msg = msg[len(msg)-600:]
		}
		return fmt.Errorf("modal fine-tuning failed: %v: %s", runErr, msg)
	}
	return nil
}
