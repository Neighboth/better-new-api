package notebookscript

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ModelFileSpec describes an individual weight/component file for multi-file models.
type ModelFileSpec struct {
	Repo string `json:"repo,omitempty"`
	File string `json:"file"`
	Role string `json:"role,omitempty"` // main | text_encoder | text_encoder_2 | vae | unet | lora | extra
}

// Config holds parameters for generating a Colab/Kaggle model serving script.
type Config struct {
	Provider             string
	DeploymentName       string
	ModelRepo            string
	ModelFile            string
	ModelFiles           []ModelFileSpec
	TaskType             string // text, image, video, audio, embeddings
	Accelerator          string
	FallbackAccelerators []string
	IdleTimeoutSeconds   int
	HFToken              string
	CallbackURL          string
	ShutdownToken        string
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

	modelFilesJSON := "[]"
	if len(cfg.ModelFiles) > 0 {
		if b, err := common.Marshal(cfg.ModelFiles); err == nil {
			modelFilesJSON = string(b)
		}
	}

	var pipPackages string
	var extraPipArgs string
	var needsTorch bool
	var isGGUF bool
	var taskHandler string

	switch task {
	case "video":
		needsTorch = true
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("diffusers", "diffusers"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("imageio[ffmpeg]", "imageio"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub"), ("safetensors", "safetensors"), ("omegaconf", "omegaconf"), ("gguf", "gguf")]`
		taskHandler = `
import uuid
from diffusers import DiffusionPipeline
from huggingface_hub import hf_hub_download, list_repo_files
from diffusers.utils import export_to_video

_pipe = None
def get_pipe():
    global _pipe
    if _pipe is not None:
        return _pipe
    _t0 = time.time()
    _log(f"[model] Preparing video model {MODEL_REPO} (cuda={torch.cuda.is_available()})...", push_callback=True)
    _dtype = torch.float16 if torch.cuda.is_available() else torch.float32
    _entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
    _main_entry = next((e for e in _entries if (e.get("role") or "main") == "main" and e.get("file")), None)
    _explicit_file = (_main_entry.get("file") if _main_entry else "") or MODEL_FILE
    _explicit_repo = (_main_entry.get("repo") if _main_entry else "") or MODEL_REPO
    _repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
    _log(f"[model] Repo contains {len(_repo_files)} files")
    if "model_index.json" in _repo_files and not _explicit_file.lower().endswith((".safetensors", ".ckpt")):
        _log(f"[model] Loading DiffusionPipeline from {MODEL_REPO}...")
        _pipe = DiffusionPipeline.from_pretrained(MODEL_REPO, torch_dtype=_dtype, token=HF_TOKEN or None)
    else:
        _target = _explicit_file or next((f for f in sorted(_repo_files) if f.lower().endswith((".safetensors", ".ckpt"))), "")
        if not _target:
            raise RuntimeError(f"No model_index.json or .safetensors/.ckpt file found in {MODEL_REPO}")
        _log(f"[model] Downloading checkpoint {_explicit_repo}/{_target}...", push_callback=True)
        _ckpt = hf_hub_download(repo_id=_explicit_repo, filename=_target, token=HF_TOKEN or None)
        _log(f"[model] Downloaded checkpoint ({os.path.getsize(_ckpt)/(1024*1024):.1f} MB)")
        import diffusers as _diff
        _v_errs = []
        for _cls_name in ("StableVideoDiffusionPipeline", "CogVideoXPipeline", "WanPipeline"):
            try:
                _cls = getattr(_diff, _cls_name, None)
                if _cls is not None and hasattr(_cls, "from_single_file"):
                    _log(f"[model] Trying {_cls_name}.from_single_file...")
                    _pipe = _cls.from_single_file(_ckpt, torch_dtype=_dtype)
                    break
            except Exception as _exc:
                _v_errs.append(f"{_cls_name}: {_exc}")
        if _pipe is None:
            raise RuntimeError("Failed to load single-file video checkpoint: " + " | ".join(_v_errs))
    for _entry in _entries:
        _role = (_entry.get("role") or "extra").lower()
        _fn = _entry.get("file") or ""
        if not _fn or _role == "main":
            continue
        _log(f"[model] Downloading component ({_role}): {_fn}...")
        _lp = hf_hub_download(repo_id=_entry.get("repo") or MODEL_REPO, filename=_fn, token=HF_TOKEN or None)
        if _role == "lora" and hasattr(_pipe, "load_lora_weights"):
            _pipe.load_lora_weights(_lp)
        elif _role == "vae" and hasattr(_pipe, "vae"):
            from diffusers import AutoencoderKL
            _pipe.vae = AutoencoderKL.from_single_file(_lp, torch_dtype=_dtype)
    if torch.cuda.is_available():
        _pipe = _pipe.to("cuda")
    _log(f"[model] Video model ready in {time.time() - _t0:.1f}s", push_callback=True)
    return _pipe

def _execute_task(body, base_url, set_progress=None):
    prompt = body.get("prompt", "A serene mountain landscape")
    num_frames = int(body.get("num_frames", 16))
    steps = int(body.get("num_inference_steps") or body.get("steps") or 20)
    if set_progress:
        set_progress(5, "Loading video model...")
    _t0 = time.time()
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
        result = pipe(prompt=prompt, num_frames=num_frames, num_inference_steps=steps, callback_on_step_end=_cb)
    except TypeError:
        try:
            result = pipe(prompt=prompt, num_frames=num_frames, num_inference_steps=steps, callback=_legacy_cb, callback_steps=1)
        except TypeError:
            result = pipe(prompt=prompt, num_frames=num_frames)
    frames = result.frames[0]
    if set_progress:
        set_progress(92, "Encoding MP4 video...")
    filename = f"{uuid.uuid4().hex}.mp4"
    out_path = os.path.join(OUTPUT_DIR, filename)
    export_to_video(frames, out_path, fps=8)
    _log(f"[infer] Video generated in {time.time() - _t0:.1f}s -> {filename}")
    if set_progress:
        set_progress(100, "Completed")
    return {"data": [{"url": f"{base_url}/outputs/{filename}"}]}

@app.post("/v1/videos/generations")
async def generate_video(req: Request):
    touch()
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
	case "image":
		needsTorch = true
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("diffusers", "diffusers"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("pillow", "PIL"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub"), ("safetensors", "safetensors"), ("omegaconf", "omegaconf"), ("gguf", "gguf")]`
		taskHandler = `
import gc
import uuid
from diffusers import AutoPipelineForText2Image
from huggingface_hub import hf_hub_download, list_repo_files

def _ensure_sd_cpp():
    if importlib.util.find_spec("stable_diffusion_cpp") is not None:
        return
    _has_cuda = bool(shutil.which("nvcc") and os.path.exists("/dev/nvidia0"))
    _log(f"[setup] Installing stable-diffusion-cpp-python (cuda={_has_cuda})...", push_callback=True)
    _env = os.environ.copy()
    if _has_cuda:
        _env["CMAKE_ARGS"] = "-DSD_CUDA=ON"
    _cmd = [sys.executable, "-m", "pip", "install", "--progress-bar", "off", "stable-diffusion-cpp-python"]
    _proc = subprocess.Popen(_cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, env=_env)
    if _proc.stdout is not None:
        for _ln in _proc.stdout:
            _s = _ln.rstrip()
            if _s:
                _log(f"[pip] {_s}", push_callback=any(k in _s for k in ("Collecting", "Downloading", "Building", "Successfully", "ERROR")))
    _rc = _proc.wait()
    if _rc != 0 and _has_cuda:
        _log("[setup] CUDA build failed for stable-diffusion-cpp-python, retrying CPU build...", push_callback=True)
        _env.pop("CMAKE_ARGS", None)
        _proc2 = subprocess.Popen(_cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, env=_env)
        if _proc2.stdout is not None:
            for _ln in _proc2.stdout:
                _s = _ln.rstrip()
                if _s:
                    _log(f"[pip] {_s}", push_callback=any(k in _s for k in ("Collecting", "Downloading", "Building", "Successfully", "ERROR")))
        _rc = _proc2.wait()
    if _rc != 0:
        raise RuntimeError(f"Failed to install stable-diffusion-cpp-python (exit {_rc})")
    _log("[setup] stable-diffusion-cpp-python ready", push_callback=True)

_pipe = None
def get_pipe():
    global _pipe
    if _pipe is not None:
        return _pipe
    _t0 = time.time()
    _log(f"[model] Preparing image model {MODEL_REPO} (cuda={torch.cuda.is_available()})...", push_callback=True)
    _dtype = torch.float16 if torch.cuda.is_available() else torch.float32
    _entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
    _main_entry = next((e for e in _entries if (e.get("role") or "main") == "main" and e.get("file")), None)
    _explicit_file = (_main_entry.get("file") if _main_entry else "") or MODEL_FILE
    _explicit_repo = (_main_entry.get("repo") if _main_entry else "") or MODEL_REPO
    _repo_files = list_repo_files(MODEL_REPO, token=HF_TOKEN or None)
    _log(f"[model] Repo contains {len(_repo_files)} files")
    if "model_index.json" in _repo_files and not _explicit_file.lower().endswith((".safetensors", ".ckpt", ".gguf")):
        _log(f"[model] Loading AutoPipelineForText2Image from {MODEL_REPO}...")
        _pipe = AutoPipelineForText2Image.from_pretrained(MODEL_REPO, torch_dtype=_dtype, low_cpu_mem_usage=True, token=HF_TOKEN or None)
    else:
        _target = _explicit_file or next((f for f in sorted(_repo_files) if f.lower().endswith((".safetensors", ".ckpt", ".gguf"))), "")
        if not _target:
            raise RuntimeError(f"No model_index.json or .safetensors/.ckpt/.gguf file found in {MODEL_REPO}")
        _log(f"[model] Downloading checkpoint {_explicit_repo}/{_target}...", push_callback=True)
        _ckpt = hf_hub_download(repo_id=_explicit_repo, filename=_target, token=HF_TOKEN or None)
        _log(f"[model] Downloaded checkpoint ({os.path.getsize(_ckpt)/(1024*1024):.1f} MB)")
        if _target.lower().endswith(".gguf"):
            _ensure_sd_cpp()
            from stable_diffusion_cpp import StableDiffusion
            _sd_kw = {}
            for _entry in _entries:
                _role = (_entry.get("role") or "extra").lower()
                _fn = _entry.get("file") or ""
                if not _fn or _role == "main":
                    continue
                _log(f"[model] Downloading GGUF component ({_role}): {_fn}...")
                _lp = hf_hub_download(repo_id=_entry.get("repo") or MODEL_REPO, filename=_fn, token=HF_TOKEN or None)
                if _role == "vae":
                    _sd_kw["vae_path"] = _lp
                elif _role == "text_encoder":
                    _sd_kw["clip_l_path"] = _lp
                elif _role == "text_encoder_2":
                    _sd_kw["t5xxl_path"] = _lp
                elif _role == "lora":
                    _sd_kw["lora_model_dir"] = os.path.dirname(_lp)
            _log("[model] Initializing stable-diffusion.cpp engine...")
            try:
                _sd = StableDiffusion(model_path=_ckpt, **_sd_kw)
            except Exception as _exc1:
                _log(f"[model] Retrying with diffusion_model_path ({_exc1})...")
                _sd = StableDiffusion(diffusion_model_path=_ckpt, **_sd_kw)

            class _GGUFImageWrapper:
                def __init__(self, engine):
                    self._engine = engine
                def __call__(self, prompt: str, width: int = 512, height: int = 512, num_inference_steps: int = 20, progress_callback=None):
                    _imgs = self._engine.generate_image(prompt=prompt, width=width, height=height, sample_steps=num_inference_steps, progress_callback=progress_callback)
                    return type("_Out", (), {"images": _imgs})()

            _pipe = _GGUFImageWrapper(_sd)
            _log(f"[model] GGUF image model ready in {time.time() - _t0:.1f}s", push_callback=True)
            return _pipe

        from diffusers import AutoencoderKL, StableDiffusionPipeline, StableDiffusionXLPipeline
        from transformers import CLIPTextModel, CLIPTextModelWithProjection, CLIPTokenizer
        _errs = []
        try:
            _log("[model] Trying StableDiffusionPipeline.from_single_file...")
            _pipe = StableDiffusionPipeline.from_single_file(_ckpt, torch_dtype=_dtype)
        except Exception as _exc:
            _msg = str(_exc)
            _errs.append(f"StableDiffusionPipeline: {_msg}")
            if "CLIPTextModel" in _msg or "text_encoder" in _msg or "AutoencoderKL" in _msg or "vae" in _msg:
                try:
                    _kw = {}
                    if "CLIPTextModel" in _msg or "text_encoder" in _msg:
                        _kw["text_encoder"] = CLIPTextModel.from_pretrained("openai/clip-vit-large-patch14", torch_dtype=_dtype)
                        _kw["tokenizer"] = CLIPTokenizer.from_pretrained("openai/clip-vit-large-patch14")
                    if "AutoencoderKL" in _msg or "vae" in _msg:
                        _kw["vae"] = AutoencoderKL.from_pretrained("stabilityai/sd-vae-ft-mse", torch_dtype=_dtype)
                    _pipe = StableDiffusionPipeline.from_single_file(_ckpt, torch_dtype=_dtype, **_kw)
                except Exception as _exc2:
                    _errs.append(f"StableDiffusionPipeline+fallback: {_exc2}")
        if _pipe is None:
            try:
                _log("[model] Trying StableDiffusionXLPipeline.from_single_file...")
                _pipe = StableDiffusionXLPipeline.from_single_file(_ckpt, torch_dtype=_dtype)
            except Exception as _exc:
                _msg = str(_exc)
                _errs.append(f"StableDiffusionXLPipeline: {_msg}")
                if "CLIPTextModel" in _msg or "text_encoder" in _msg:
                    try:
                        _kw = {
                            "text_encoder": CLIPTextModel.from_pretrained("openai/clip-vit-large-patch14", torch_dtype=_dtype),
                            "tokenizer": CLIPTokenizer.from_pretrained("openai/clip-vit-large-patch14"),
                            "text_encoder_2": CLIPTextModelWithProjection.from_pretrained("laion/CLIP-ViT-bigG-14-laion2B-39B-b160k", torch_dtype=_dtype),
                            "tokenizer_2": CLIPTokenizer.from_pretrained("laion/CLIP-ViT-bigG-14-laion2B-39B-b160k"),
                        }
                        _pipe = StableDiffusionXLPipeline.from_single_file(_ckpt, torch_dtype=_dtype, **_kw)
                    except Exception as _exc2:
                        _errs.append(f"StableDiffusionXLPipeline+fallback: {_exc2}")
        if _pipe is None:
            try:
                import diffusers as _diff
                _flux = getattr(_diff, "FluxPipeline", None)
                if _flux is not None and hasattr(_flux, "from_single_file"):
                    _log("[model] Trying FluxPipeline.from_single_file...")
                    _pipe = _flux.from_single_file(_ckpt, torch_dtype=_dtype)
            except Exception as _exc:
                _errs.append(f"FluxPipeline: {_exc}")
        if _pipe is None:
            raise RuntimeError("Failed to load single-file image checkpoint: " + " | ".join(_errs))
    for _entry in _entries:
        _role = (_entry.get("role") or "extra").lower()
        _fn = _entry.get("file") or ""
        if not _fn or _role == "main":
            continue
        _log(f"[model] Downloading component ({_role}): {_fn}...")
        _lp = hf_hub_download(repo_id=_entry.get("repo") or MODEL_REPO, filename=_fn, token=HF_TOKEN or None)
        if _role == "vae" and hasattr(_pipe, "vae"):
            from diffusers import AutoencoderKL
            _pipe.vae = AutoencoderKL.from_single_file(_lp, torch_dtype=_dtype)
        elif _role == "lora" and hasattr(_pipe, "load_lora_weights"):
            _pipe.load_lora_weights(_lp)
        elif _role in ("text_encoder", "text_encoder_2", "unet") and hasattr(_pipe, _role):
            from safetensors.torch import load_file
            _sd = load_file(_lp) if _lp.lower().endswith(".safetensors") else torch.load(_lp, map_location="cpu")
            getattr(_pipe, _role).load_state_dict(_sd, strict=False)
    if torch.cuda.is_available():
        _pipe = _pipe.to("cuda")
    if hasattr(_pipe, "enable_attention_slicing"):
        try:
            _pipe.enable_attention_slicing()
        except Exception:
            pass
    if hasattr(_pipe, "enable_vae_slicing"):
        try:
            _pipe.enable_vae_slicing()
        except Exception:
            pass
    gc.collect()
    _log(f"[model] Image model ready in {time.time() - _t0:.1f}s", push_callback=True)
    return _pipe

def _execute_task(body, base_url, set_progress=None):
    prompt = body.get("prompt", "A scenic landscape")
    width = int(body.get("width") or 512)
    height = int(body.get("height") or 512)
    steps = int(body.get("num_inference_steps") or body.get("steps") or 20)
    if set_progress:
        set_progress(5, "Loading image model...")
    _t0 = time.time()
    _log("[infer] Generating image...")
    pipe = get_pipe()
    if set_progress:
        set_progress(20, "Model loaded, generating image...")
    if hasattr(pipe, "_engine"):
        def _on_step(step, total, _t):
            pct = 20 + int((step / max(1, total)) * 75)
            stage = f"Generating image step {step}/{total}"
            _log(f"[infer] {stage} ({min(95, pct)}%)")
            if set_progress:
                set_progress(min(95, pct), stage)
            time.sleep(0.001)
        img = pipe(prompt=prompt, width=width, height=height, num_inference_steps=steps, progress_callback=_on_step).images[0]
    else:
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
        with torch.inference_mode():
            try:
                img = pipe(prompt=prompt, num_inference_steps=steps, callback_on_step_end=_cb).images[0]
            except TypeError:
                try:
                    img = pipe(prompt=prompt, num_inference_steps=steps, callback=_legacy_cb, callback_steps=1).images[0]
                except TypeError:
                    img = pipe(prompt=prompt).images[0]
    filename = f"{uuid.uuid4().hex}.png"
    out_path = os.path.join(OUTPUT_DIR, filename)
    img.save(out_path)
    gc.collect()
    _log(f"[infer] Image generated in {time.time() - _t0:.1f}s -> {filename}")
    if set_progress:
        set_progress(100, "Completed")
    return {"data": [{"url": f"{base_url}/outputs/{filename}"}]}

@app.post("/v1/images/generations")
async def generate_image(req: Request):
    touch()
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
	case "audio":
		needsTorch = true
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("transformers", "transformers"), ("accelerate", "accelerate"), ("scipy", "scipy"), ("soundfile", "soundfile"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
		taskHandler = `
import uuid, scipy.io.wavfile as wavfile
from transformers import pipeline as hf_pipeline

_audio_pipe = None
def get_audio_pipe():
    global _audio_pipe
    if _audio_pipe is None:
        _log(f"[model] Loading audio model {MODEL_REPO}...")
        _audio_pipe = hf_pipeline("text-to-audio", model=MODEL_REPO, device=0 if torch.cuda.is_available() else -1, token=HF_TOKEN or None)
        _log("[model] Audio model ready")
    return _audio_pipe

def _execute_task(body, base_url, set_progress=None):
    text = body.get("input") or body.get("prompt") or "Hello world"
    if set_progress:
        set_progress(5, "Loading audio model...")
    _t0 = time.time()
    pipe = get_audio_pipe()
    if set_progress:
        set_progress(40, "Synthesizing audio...")
    out = pipe(text)
    filename = f"{uuid.uuid4().hex}.wav"
    out_path = os.path.join(OUTPUT_DIR, filename)
    wavfile.write(out_path, rate=int(out.get("sampling_rate", 16000)), data=out["audio"])
    _log(f"[infer] Audio generated in {time.time() - _t0:.1f}s -> {filename}")
    if set_progress:
        set_progress(100, "Completed")
    return {"url": f"{base_url}/outputs/{filename}", "data": [{"url": f"{base_url}/outputs/{filename}"}]}

@app.post("/v1/audio/speech")
@app.post("/v1/audio/generations")
async def generate_audio(req: Request):
    touch()
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
	case "embeddings":
		pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("sentence-transformers", "sentence_transformers"), ("torch", "torch"), ("huggingface_hub", "huggingface_hub")]`
		taskHandler = `
from sentence_transformers import SentenceTransformer

_embed_model = None
def get_embed_model():
    global _embed_model
    if _embed_model is None:
        _log(f"[model] Loading embeddings model {MODEL_REPO}...")
        _embed_model = SentenceTransformer(MODEL_REPO, use_auth_token=HF_TOKEN or None)
        _log("[model] Embeddings model ready")
    return _embed_model

def _execute_task(body, _base_url, set_progress=None):
    inp = body.get("input", "")
    texts = inp if isinstance(inp, list) else [str(inp)]
    if set_progress:
        set_progress(5, "Loading embeddings model...")
    mdl = get_embed_model()
    if set_progress:
        set_progress(40, "Encoding embeddings...")
    vecs = mdl.encode(texts).tolist()
    if set_progress:
        set_progress(100, "Completed")
    return {
        "object": "list",
        "model": MODEL_REPO,
        "data": [{"object": "embedding", "index": i, "embedding": v} for i, v in enumerate(vecs)],
    }

@app.post("/v1/embeddings")
async def create_embeddings(req: Request):
    touch()
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
	default: // text (supports both GGUF via llama-cpp-python and standard transformers)
		isGGUF = strings.HasSuffix(strings.ToLower(strings.TrimSpace(cfg.ModelFile)), ".gguf") ||
			strings.Contains(strings.ToLower(cfg.ModelRepo), "gguf")
		if isGGUF {
			pipPackages = `[("fastapi", "fastapi"), ("uvicorn", "uvicorn"), ("huggingface_hub", "huggingface_hub"), ("llama-cpp-python", "llama_cpp")]`
			extraPipArgs = `(["--prefer-binary", "--extra-index-url", "https://abetlen.github.io/llama-cpp-python/whl/cu124"] if os.path.exists("/dev/nvidia0") else ["--prefer-binary", "--extra-index-url", "https://abetlen.github.io/llama-cpp-python/whl/cpu"])`
			taskHandler = `
from huggingface_hub import hf_hub_download, list_repo_files
from llama_cpp import Llama

_llm = None
def get_llm():
    global _llm
    if _llm is not None:
        return _llm
    entries = json.loads(MODEL_FILES_JSON) if MODEL_FILES_JSON else []
    main_entry = next((e for e in entries if (e.get("role") or "main") == "main" and e.get("file")), None)
    target_file = (main_entry.get("file") if main_entry else "") or MODEL_FILE
    target_repo = (main_entry.get("repo") if main_entry else "") or MODEL_REPO
    for entry in entries:
        fn = entry.get("file") or ""
        if fn and fn != target_file:
            hf_hub_download(repo_id=entry.get("repo") or MODEL_REPO, filename=fn, token=HF_TOKEN or None)
    if not target_file:
        files = [f for f in list_repo_files(MODEL_REPO, token=HF_TOKEN or None) if f.lower().endswith(".gguf")]
        if not files:
            raise RuntimeError(f"No .gguf file found in {MODEL_REPO}")
        target_file = sorted(files)[0]
    _log(f"[model] Downloading GGUF model {target_repo}/{target_file}...", push_callback=True)
    local_path = hf_hub_download(repo_id=target_repo, filename=target_file, token=HF_TOKEN or None)
    n_gpu = -1 if os.path.exists("/dev/nvidia0") else 0
    _log(f"[model] Loading GGUF ({os.path.getsize(local_path)/(1024*1024):.1f} MB, gpu_layers={n_gpu})...", push_callback=True)
    _llm = Llama(model_path=local_path, n_ctx=4096, n_gpu_layers=n_gpu, verbose=False)
    _log("[model] GGUF model ready", push_callback=True)
    return _llm

def _execute_task(body, _base_url, set_progress=None):
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    if set_progress:
        set_progress(5, "Loading GGUF model...")
    _t0 = time.time()
    llm = get_llm()
    if set_progress:
        set_progress(40, "Generating text response...")
    out = llm.create_chat_completion(messages=messages, max_tokens=max_tokens)
    _log(f"[infer] GGUF chat completion done in {time.time() - _t0:.2f}s")
    if set_progress:
        set_progress(100, "Completed")
    return out

@app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    touch()
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
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
    _t0 = time.time()
    _log(f"[model] Loading transformers model {MODEL_REPO} (cuda={torch.cuda.is_available()})...", push_callback=True)
    tok = AutoTokenizer.from_pretrained(MODEL_REPO, token=HF_TOKEN or None)
    mdl = AutoModelForCausalLM.from_pretrained(
        MODEL_REPO,
        torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32,
        device_map="auto" if torch.cuda.is_available() else None,
        token=HF_TOKEN or None,
    )
    _backend = (tok, mdl)
    _log(f"[model] Transformers model ready in {time.time() - _t0:.1f}s", push_callback=True)
    return _backend

def _execute_task(body, _base_url, set_progress=None):
    messages = body.get("messages", [])
    max_tokens = int(body.get("max_tokens", 256))
    if set_progress:
        set_progress(5, "Loading transformers model...")
    tok, mdl = get_backend()
    if set_progress:
        set_progress(40, "Generating text response...")
    prompt = "\n".join(f"{m.get('role', 'user')}: {m.get('content', '')}" for m in messages) + "\nassistant:"
    inputs = tok(prompt, return_tensors="pt")
    if torch.cuda.is_available():
        inputs = {k: v.to(mdl.device) for k, v in inputs.items()}
    with torch.no_grad():
        output_ids = mdl.generate(**inputs, max_new_tokens=max_tokens)
    generated = tok.decode(output_ids[0][inputs["input_ids"].shape[-1]:], skip_special_tokens=True)
    if set_progress:
        set_progress(100, "Completed")
    return {
        "id": "chatcmpl-notebook",
        "object": "chat.completion",
        "model": MODEL_REPO,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": generated}, "finish_reason": "stop"}],
    }

@app.post("/v1/chat/completions")
async def chat_completions(req: Request):
    touch()
    body = await req.json()
    return _execute_task(body, str(req.base_url).rstrip("/"))
`
		}
	}

	if extraPipArgs == "" {
		extraPipArgs = "[]"
	}
	ggufWheelHelper := ""
	if isGGUF {
		ggufWheelHelper = `
def _find_llama_cpp_wheel():
    try:
        py_tag = f"cp{sys.version_info.major}{sys.version_info.minor}-cp{sys.version_info.major}{sys.version_info.minor}-linux_x86_64.whl"
        has_gpu = os.path.exists("/dev/nvidia0")
        req = urllib.request.Request(
            "https://api.github.com/repos/JamePeng/llama-cpp-python/releases?per_page=15",
            headers={"User-Agent": "BetterNewAPI-Deployer/1.0"},
        )
        with urllib.request.urlopen(req, timeout=10) as resp:
            rels = json.loads(resp.read().decode("utf-8"))
        prefs = ("cu124", "cu126", "cu128", "cu122") if has_gpu else ("basic", "cpu")
        for pref in prefs:
            for rel in rels:
                for asset in rel.get("assets", []):
                    name = asset.get("name", "")
                    if py_tag in name and pref in name:
                        return asset.get("browser_download_url")
    except Exception:
        pass
    return None
`
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
import shutil
import subprocess
import sys
import threading
import time
import urllib.request

MODEL_REPO = %s
MODEL_FILE = %s
MODEL_FILES_JSON = %s
HF_TOKEN = %s
CALLBACK_URL = %s
SHUTDOWN_TOKEN = %s
IDLE_TIMEOUT_SECONDS = %d
OUTPUT_DIR = "/tmp/model_outputs"
LOG_FILE = "/tmp/serve_runtime.log"
os.makedirs(OUTPUT_DIR, exist_ok=True)
_log_lock = threading.Lock()
_log_lines = []

def _post_callback_log(msg: str):
    if not CALLBACK_URL:
        return
    try:
        payload = json.dumps({"log": msg}).encode("utf-8")
        req = urllib.request.Request(
            CALLBACK_URL,
            data=payload,
            headers={"Content-Type": "application/json", "User-Agent": "BetterNewAPI-Deployer/1.0"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=5):
            pass
    except Exception:
        pass

def _log(msg: str, push_callback: bool = False):
    line = f"{time.strftime('%%H:%%M:%%S')} {msg}"
    print(line, flush=True)
    with _log_lock:
        _log_lines.append(line)
        if len(_log_lines) > 500:
            del _log_lines[:len(_log_lines) - 500]
        try:
            with open(LOG_FILE, "w", encoding="utf-8") as f:
                f.write("\n".join(_log_lines) + "\n")
        except Exception:
            pass
    if push_callback:
        threading.Thread(target=_post_callback_log, args=(line,), daemon=True).start()

def _report_tunnel_url(url: str):
    _log(f"[tunnel] Tunnel allocated ({url}), waiting for local server on :8000...", push_callback=True)
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
    _log(f"[tunnel] Ready: {url}", push_callback=True)
    if not CALLBACK_URL:
        return
    payload = json.dumps({"endpoint_url": url, "status": "running"}).encode("utf-8")
    for _ in range(5):
        try:
            req = urllib.request.Request(
                CALLBACK_URL,
                data=payload,
                headers={"Content-Type": "application/json", "User-Agent": "BetterNewAPI-Deployer/1.0"},
                method="POST",
            )
            with urllib.request.urlopen(req, timeout=10) as resp:
                if 200 <= resp.status < 300:
                    return
        except Exception as exc:
            _log(f"[tunnel] callback retry: {exc}")
        time.sleep(2)

def _start_cloudflared():
    try:
        _log("[tunnel] Starting cloudflared tunnel...", push_callback=True)
        cf_bin = shutil.which("cloudflared") or ("/tmp/cloudflared" if os.path.exists("/tmp/cloudflared") else None)
        if not cf_bin:
            cf_bin = "/tmp/cloudflared"
            _log("[tunnel] Downloading cloudflared binary to /tmp/cloudflared...", push_callback=True)
            dl_req = urllib.request.Request(
                "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64",
                headers={"User-Agent": "BetterNewAPI-Deployer/1.0"},
            )
            with urllib.request.urlopen(dl_req, timeout=60) as resp, open(cf_bin, "wb") as out_f:
                total = int(resp.headers.get("Content-Length") or 0)
                done = 0
                next_pct = 25
                while True:
                    chunk = resp.read(262144)
                    if not chunk:
                        break
                    out_f.write(chunk)
                    done += len(chunk)
                    if total > 0:
                        pct = int(done * 100 / total)
                        if pct >= next_pct:
                            _log(f"[tunnel] Downloading cloudflared: {pct}%% ({done/(1024*1024):.1f}/{total/(1024*1024):.1f} MB)", push_callback=True)
                            next_pct += 25
            os.chmod(cf_bin, 0o755)
            _log("[tunnel] cloudflared binary downloaded", push_callback=True)
        proc = subprocess.Popen(
            [cf_bin, "tunnel", "--url", "http://127.0.0.1:8000"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        reported = False
        if proc.stdout is not None:
            for line in proc.stdout:
                s = line.strip()
                if s and any(k in s for k in ("ERR", "Registered tunnel connection", "trycloudflare.com", "Starting tunnel")):
                    _log(f"[tunnel] {s}")
                if not reported:
                    m = re.search(r"https://[a-zA-Z0-9-]+\.trycloudflare\.com", line)
                    if m:
                        reported = True
                        _report_tunnel_url(m.group(0))
    except Exception as exc:
        _log(f"[tunnel] ERROR: {exc}", push_callback=True)

_gpu_desc = "none"
if os.path.exists("/dev/nvidia0"):
    try:
        _gpu_desc = subprocess.check_output(["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader"], text=True, timeout=5).strip() or "nvidia"
    except Exception:
        _gpu_desc = "nvidia"
_log(f"[boot] Python {sys.version.split()[0]} | CPUs={os.cpu_count()} | GPU={_gpu_desc} | Model={MODEL_REPO}", push_callback=True)

threading.Thread(target=_start_cloudflared, daemon=True).start()

%s
def _ensure_packages(specs, extra_args=None):
    missing = [pkg for pkg, mod in specs if importlib.util.find_spec(mod) is None]
    if not missing:
        _log("[setup] All required packages already installed", push_callback=True)
        return
    if "_find_llama_cpp_wheel" in globals():
        target_pkg = next((p for p in missing if "llama" in p and "cpp" in p), None)
        if target_pkg:
            whl = _find_llama_cpp_wheel()
            if whl:
                _log(f"[setup] Installing prebuilt wheel: {whl}", push_callback=True)
                wcmd = [sys.executable, "-m", "pip", "install", "--progress-bar", "off", whl]
                wproc = subprocess.Popen(wcmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
                if wproc.stdout is not None:
                    for raw_line in wproc.stdout:
                        s = raw_line.rstrip()
                        if s:
                            _log(f"[pip] {s}", push_callback=any(k in s for k in ("Downloading", "Successfully", "ERROR")))
                if wproc.wait() == 0:
                    missing = [p for p in missing if p != target_pkg]
                    if not missing:
                        _log("[setup] Package installation complete", push_callback=True)
                        return
    _log(f"[setup] Installing packages: {', '.join(missing)}", push_callback=True)
    cmd = [sys.executable, "-m", "pip", "install", "--progress-bar", "off"] + (extra_args or []) + missing
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    if proc.stdout is not None:
        for raw_line in proc.stdout:
            s = raw_line.rstrip()
            if s:
                _log(f"[pip] {s}", push_callback=any(k in s for k in ("Collecting", "Downloading", "Building", "Successfully", "ERROR")))
    rc = proc.wait()
    if rc != 0:
        raise subprocess.CalledProcessError(rc, cmd)
    _log("[setup] Package installation complete", push_callback=True)

_ensure_packages(%s, %s)

%simport uvicorn
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles

app = FastAPI(title="Better-New-API Notebook Model Server")
app.mount("/outputs", StaticFiles(directory=OUTPUT_DIR), name="outputs")
_last_activity = time.time()

@app.exception_handler(Exception)
async def unhandled_exception_handler(request: Request, exc: Exception):
    _log(f"[error] {type(exc).__name__}: {exc}")
    return JSONResponse(status_code=500, content={"error": f"{type(exc).__name__}: {exc}"})

def touch():
    global _last_activity
    _last_activity = time.time()

def _idle_watchdog():
    if IDLE_TIMEOUT_SECONDS <= 0:
        return
    while True:
        time.sleep(10)
        with _jobs_lock:
            has_active = any(v.get("status") in ("queued", "running") for v in _jobs.values())
        if has_active:
            touch()
            continue
        if time.time() - _last_activity > IDLE_TIMEOUT_SECONDS:
            _log(f"[watchdog] Idle for {IDLE_TIMEOUT_SECONDS}s, shutting down...")
            os._exit(0)

threading.Thread(target=_idle_watchdog, daemon=True).start()

@app.get("/health")
def health():
    touch()
    return {"status": "ok", "model": MODEL_REPO, "task": %s}

@app.get("/_logs")
def get_logs(req: Request):
    tok = req.query_params.get("token") or req.headers.get("X-Shutdown-Token") or ""
    if not SHUTDOWN_TOKEN or tok != SHUTDOWN_TOKEN:
        from fastapi import HTTPException
        raise HTTPException(status_code=403, detail="forbidden")
    with _log_lock:
        return {"logs": "\n".join(_log_lines)}

@app.post("/_shutdown")
async def shutdown(req: Request):
    tok = req.query_params.get("token") or req.headers.get("X-Shutdown-Token") or ""
    if not SHUTDOWN_TOKEN or tok != SHUTDOWN_TOKEN:
        from fastapi import HTTPException
        raise HTTPException(status_code=403, detail="forbidden")
    threading.Timer(0.2, lambda: os._exit(0)).start()
    return {"status": "shutting_down"}

@app.get("/v1/models")
def list_models():
    touch()
    return {"object": "list", "data": [{"id": MODEL_REPO, "object": "model"}]}
%s
_jobs = {}
_jobs_lock = threading.Lock()

@app.post("/_jobs")
async def start_job(req: Request):
    touch()
    body = await req.json()
    base_url = str(req.base_url).rstrip("/")
    job_id = str(int(time.time() * 1000000)) + "-" + str(threading.get_ident())
    now = time.time()
    with _jobs_lock:
        stale = [k for k, v in _jobs.items() if now - v.get("updated_at", now) > 3600]
        for k in stale:
            del _jobs[k]
        _jobs[job_id] = {"job_id": job_id, "status": "queued", "progress": 0, "stage": "Queued...", "result": None, "error": None, "updated_at": now}

    def _set_progress(pct: int, stage: str):
        touch()
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

@app.get("/_jobs/{job_id}")
async def get_job(job_id: str):
    touch()
    with _jobs_lock:
        job = _jobs.get(job_id)
        if not job:
            from fastapi import HTTPException
            raise HTTPException(status_code=404, detail="job not found")
        return {k: v for k, v in job.items() if k != "updated_at"}

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
		pyQuote(modelFilesJSON),
		pyQuote(cfg.HFToken),
		pyQuote(cfg.CallbackURL),
		pyQuote(cfg.ShutdownToken),
		idle,
		ggufWheelHelper,
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
