package finetunescript

import (
	"fmt"
	"strings"
)

// Config defines parameters for generating a real PyTorch + Transformers + PEFT /
// Diffusers fine-tuning Python script that runs on Modal, Google Colab, or Kaggle.
type Config struct {
	JobID          string
	FineTunedModel string
	BaseModel      string // HF repo ID, direct URL, or filename when BaseModelURL is provided
	BaseModelURL   string // Optional authenticated server download URL for locally uploaded base weights
	ModelSource    string // huggingface | url | upload
	DatasetURL     string // Authenticated server download URL for training dataset
	DatasetName    string
	TaskType       string // text | image | audio | vision
	Method         string // lora | qlora | sft | dpo | dreambooth | textual_inversion | full
	Epochs         int
	Accelerator    string
	CallbackURL    string // POST JSON status/progress/log_line updates
	ArtifactURL    string // POST multipart/form-data file=@output.zip
	CallbackToken  string
	HFToken        string
	OutputZipPath  string // Local path where output.zip is saved (default: "output.zip")
}

func pyStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", "")
	return `"` + s + `"`
}

// Generate produces a complete, self-contained Python script that performs REAL
// gradient-based fine-tuning with PyTorch + Transformers + PEFT (or Diffusers for
// image tasks), writes epoch checkpoints + trained adapter_model.safetensors +
// adapter_config.json + training_details.json + training.log into output.zip,
// streams progress callbacks, and uploads the resulting ZIP artifact.
func Generate(cfg Config) string {
	epochs := cfg.Epochs
	if epochs < 1 {
		epochs = 3
	}
	task := strings.ToLower(strings.TrimSpace(cfg.TaskType))
	if task == "" {
		task = "text"
	}
	method := strings.ToLower(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = "lora"
	}
	outZip := strings.TrimSpace(cfg.OutputZipPath)
	if outZip == "" {
		outZip = "output.zip"
	}

	return fmt.Sprintf(`#!/usr/bin/env python3
"""Real Fine-Tuning Worker Script (PyTorch + Transformers + PEFT / Diffusers)."""
import csv
import gc
import io
import json
import math
import os
import shutil
import subprocess
import sys
import time
import traceback
import urllib.request
import zipfile
from pathlib import Path

JOB_ID = %s
FINE_TUNED_MODEL = %s
BASE_MODEL = %s
BASE_MODEL_URL = %s
MODEL_SOURCE = %s
DATASET_URL = %s
DATASET_NAME = %s
TASK_TYPE = %s
METHOD = %s
EPOCHS = %d
ACCELERATOR = %s
CALLBACK_URL = %s
ARTIFACT_URL = %s
CALLBACK_TOKEN = %s
HF_TOKEN = %s
OUTPUT_ZIP_PATH = %s

WORK_DIR = Path("ft_work_" + JOB_ID)
WORK_DIR.mkdir(parents=True, exist_ok=True)
LOG_FILE = WORK_DIR / "training.log"
CHECKPOINTS_DIR = WORK_DIR / "checkpoints"
CHECKPOINTS_DIR.mkdir(parents=True, exist_ok=True)
OUTPUT_DIR = WORK_DIR / "model_out"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)


def _post_callback(payload):
    if not CALLBACK_URL:
        return
    try:
        body = json.dumps({**payload, "token": CALLBACK_TOKEN}).encode("utf-8")
        req = urllib.request.Request(
            CALLBACK_URL,
            data=body,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {CALLBACK_TOKEN}",
            },
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=15) as resp:
            resp.read()
    except Exception:
        pass


def log(stage, msg, progress=None, trained_tokens=None):
    ts = time.strftime("%%H:%%M:%%S", time.gmtime())
    line = f"{ts} [{stage}] {msg}"
    print(line, flush=True)
    try:
        with open(LOG_FILE, "a", encoding="utf-8") as lf:
            lf.write(line + "\n")
    except Exception:
        pass
    payload = {"status": "running", "log_line": line}
    if progress is not None:
        payload["progress"] = int(progress)
    if trained_tokens is not None:
        payload["trained_tokens"] = int(trained_tokens)
    _post_callback(payload)


def ensure_packages():
    needed = ["transformers", "peft", "accelerate", "safetensors", "datasets", "huggingface_hub"]
    if METHOD == "qlora":
        needed.append("bitsandbytes")
    if TASK_TYPE == "image":
        needed.extend(["diffusers", "Pillow"])
    elif TASK_TYPE == "audio":
        needed.extend(["librosa", "soundfile"])
    elif TASK_TYPE == "vision":
        needed.append("Pillow")

    missing = []
    import_names = {"Pillow": "PIL", "huggingface_hub": "huggingface_hub"}
    for pkg in needed:
        mod = import_names.get(pkg, pkg)
        try:
            __import__(mod)
        except ImportError:
            missing.append(pkg)

    if missing:
        log("deps", f"Installing required ML packages: {', '.join(missing)}", progress=4)
        cmd = [sys.executable, "-m", "pip", "install", "-q", "--upgrade"] + missing
        res = subprocess.run(cmd, capture_output=True, text=True)
        if res.returncode != 0:
            raise RuntimeError(f"pip install failed ({', '.join(missing)}): {res.stderr[-600:]}")
    try:
        import torch
        dev = torch.cuda.get_device_name(0) if torch.cuda.is_available() else "CPU"
        log("env", f"PyTorch {torch.__version__} ready | device={dev} | gpus={torch.cuda.device_count()}", progress=8)
    except Exception as e:
        raise RuntimeError(f"PyTorch unavailable: {e}")


def download_file(url, dst_path, label):
    if not url:
        return None
    log("download", f"Downloading {label}...", progress=10)
    headers = {"User-Agent": "new-api-finetune-worker/1.0"}
    if CALLBACK_TOKEN and ("/deployments/fine_tuning/" in url):
        headers["Authorization"] = f"Bearer {CALLBACK_TOKEN}"
    elif HF_TOKEN and "huggingface.co" in url:
        headers["Authorization"] = f"Bearer {HF_TOKEN}"
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req, timeout=600) as resp, open(dst_path, "wb") as out:
        total = 0
        while True:
            chunk = resp.read(1024 * 1024)
            if not chunk:
                break
            out.write(chunk)
            total += len(chunk)
    log("download", f"Downloaded {label} ({total / (1024 * 1024):.2f} MB)")
    return dst_path


def load_text_samples(dataset_path):
    ext = Path(dataset_path).suffix.lower()
    texts = []
    if ext == ".jsonl":
        with open(dataset_path, "r", encoding="utf-8", errors="ignore") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    obj = json.loads(line)
                    texts.append(format_text_record(obj))
                except Exception:
                    texts.append(line)
    elif ext == ".json":
        with open(dataset_path, "r", encoding="utf-8", errors="ignore") as f:
            data = json.load(f)
            if isinstance(data, list):
                for obj in data:
                    texts.append(format_text_record(obj))
            elif isinstance(data, dict):
                texts.append(format_text_record(data))
    elif ext == ".csv":
        with open(dataset_path, "r", encoding="utf-8", errors="ignore") as f:
            reader = csv.DictReader(f)
            for row in reader:
                texts.append(format_text_record(row))
    elif ext == ".parquet":
        from datasets import load_dataset
        ds = load_dataset("parquet", data_files=str(dataset_path), split="train")
        for row in ds:
            texts.append(format_text_record(row))
    else:
        with open(dataset_path, "r", encoding="utf-8", errors="ignore") as f:
            for line in f:
                s = line.strip()
                if s:
                    texts.append(s)
    texts = [t for t in texts if t and t.strip()]
    if not texts:
        raise RuntimeError("Dataset contains no usable training examples")
    return texts


def format_text_record(obj):
    if isinstance(obj, str):
        return obj.strip()
    if not isinstance(obj, dict):
        return str(obj)
    if "messages" in obj and isinstance(obj["messages"], list):
        parts = []
        for m in obj["messages"]:
            if isinstance(m, dict):
                role = m.get("role", "user")
                content = m.get("content", "")
                if isinstance(content, list):
                    content = " ".join(
                        p.get("text", "") for p in content if isinstance(p, dict)
                    )
                parts.append(f"<|{role}|>\n{content}")
        return "\n".join(parts)
    if "prompt" in obj and ("completion" in obj or "chosen" in obj):
        resp = obj.get("chosen") or obj.get("completion") or ""
        return f"<|user|>\n{obj['prompt']}\n<|assistant|>\n{resp}"
    if "instruction" in obj:
        inp = obj.get("input", "")
        out = obj.get("output", "")
        prompt = f"{obj['instruction']}\n{inp}".strip()
        return f"<|user|>\n{prompt}\n<|assistant|>\n{out}"
    if "text" in obj:
        return str(obj["text"])
    return json.dumps(obj, ensure_ascii=False)


def train_text_or_vision(dataset_path, base_model_ref):
    import torch
    from torch.utils.data import Dataset, DataLoader
    from transformers import AutoTokenizer, AutoModelForCausalLM, get_cosine_schedule_with_warmup
    from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training

    samples = load_text_samples(dataset_path)
    log("dataset", f"Parsed {len(samples)} training examples from {DATASET_NAME}", progress=16)

    tok_kwargs = {"trust_remote_code": True}
    if HF_TOKEN:
        tok_kwargs["token"] = HF_TOKEN
    log("model", f"Loading tokenizer & base model: {base_model_ref}", progress=20)
    tokenizer = AutoTokenizer.from_pretrained(base_model_ref, **tok_kwargs)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    use_cuda = torch.cuda.is_available()
    dtype = torch.bfloat16 if (use_cuda and torch.cuda.is_bf16_supported()) else (torch.float16 if use_cuda else torch.float32)

    model_kwargs = {"trust_remote_code": True, "low_cpu_mem_usage": True}
    if HF_TOKEN:
        model_kwargs["token"] = HF_TOKEN
    if METHOD == "qlora" and use_cuda:
        from transformers import BitsAndBytesConfig
        model_kwargs["quantization_config"] = BitsAndBytesConfig(
            load_in_4bit=True,
            bnb_4bit_quant_type="nf4",
            bnb_4bit_compute_dtype=dtype,
            bnb_4bit_use_double_quant=True,
        )
        model_kwargs["device_map"] = "auto"
    else:
        model_kwargs["torch_dtype"] = dtype
        if use_cuda:
            model_kwargs["device_map"] = "auto"

    model = AutoModelForCausalLM.from_pretrained(base_model_ref, **model_kwargs)
    model.config.use_cache = False

    if METHOD in ("lora", "qlora", "sft", "dpo"):
        if METHOD == "qlora" and use_cuda:
            model = prepare_model_for_kbit_training(model)
        # Discover linear projection layer names dynamically so any architecture works (Llama, Qwen, DeepSeek, GPT-2, Mistral, Gemma)
        linear_names = set()
        for name, mod in model.named_modules():
            cls_name = mod.__class__.__name__.lower()
            if "linear" in cls_name or "conv1d" in cls_name:
                leaf = name.split(".")[-1]
                if leaf not in ("lm_head", "embed_tokens", "wte", "wpe"):
                    linear_names.add(leaf)
        preferred = [n for n in ("q_proj", "k_proj", "v_proj", "o_proj", "gate_proj", "up_proj", "down_proj", "c_attn", "c_proj", "query_key_value") if n in linear_names]
        target_modules = preferred if preferred else (sorted(linear_names)[:4] if linear_names else "all-linear")
        lora_cfg = LoraConfig(
            r=16,
            lora_alpha=32,
            lora_dropout=0.05,
            bias="none",
            task_type="CAUSAL_LM",
            target_modules=target_modules,
        )
        model = get_peft_model(model, lora_cfg)
        trainable = sum(p.numel() for p in model.parameters() if p.requires_grad)
        total_p = sum(p.numel() for p in model.parameters())
        log("model", f"PEFT {METHOD.upper()} adapter attached: trainable={trainable:,} / {total_p:,} ({100.0 * trainable / max(1, total_p):.2f}%%)", progress=28)
    else:
        log("model", f"Full fine-tuning initialized on {sum(p.numel() for p in model.parameters()):,} parameters", progress=28)

    max_len = 512 if use_cuda else 256
    encoded = [
        tokenizer(t, truncation=True, max_length=max_len, padding="max_length", return_tensors="pt")
        for t in samples
    ]

    class SimpleTextDataset(Dataset):
        def __init__(self, items):
            self.items = items
        def __len__(self):
            return len(self.items)
        def __getitem__(self, idx):
            item = self.items[idx]
            ids = item["input_ids"].squeeze(0)
            mask = item["attention_mask"].squeeze(0)
            labels = ids.clone()
            labels[mask == 0] = -100
            return {"input_ids": ids, "attention_mask": mask, "labels": labels}

    batch_size = 4 if use_cuda else 1
    grad_accum = 4 if len(samples) >= 16 else 1
    loader = DataLoader(SimpleTextDataset(encoded), batch_size=batch_size, shuffle=True)

    device = next(model.parameters()).device
    optimizer = torch.optim.AdamW([p for p in model.parameters() if p.requires_grad], lr=2e-4 if METHOD != "full" else 5e-5, weight_decay=0.01)
    total_steps = max(1, math.ceil(len(loader) / grad_accum) * EPOCHS)
    scheduler = get_cosine_schedule_with_warmup(optimizer, num_warmup_steps=max(1, total_steps // 10), num_training_steps=total_steps)

    model.train()
    trained_tokens = 0
    loss_history = []
    global_step = 0
    last_loss = 0.0

    for ep in range(1, EPOCHS + 1):
        ep_loss_sum = 0.0
        ep_batches = 0
        optimizer.zero_grad()
        for b_idx, batch in enumerate(loader, start=1):
            batch = {k: v.to(device) for k, v in batch.items()}
            out = model(**batch)
            loss = out.loss / grad_accum
            loss.backward()
            raw_loss = float(out.loss.detach().cpu().item())
            last_loss = raw_loss
            ep_loss_sum += raw_loss
            ep_batches += 1
            trained_tokens += int(batch["attention_mask"].sum().item())

            if (b_idx %% grad_accum == 0) or (b_idx == len(loader)):
                torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
                optimizer.step()
                scheduler.step()
                optimizer.zero_grad()
                global_step += 1

                if global_step == 1 or global_step %% max(1, total_steps // 20) == 0 or global_step == total_steps:
                    pct = 30 + int(60.0 * global_step / total_steps)
                    lr = scheduler.get_last_lr()[0]
                    log("train", f"Epoch {ep}/{EPOCHS} | step {global_step}/{total_steps} | loss={raw_loss:.4f} | lr={lr:.2e} | tokens={trained_tokens:,}", progress=min(90, pct), trained_tokens=trained_tokens)

        avg_ep_loss = ep_loss_sum / max(1, ep_batches)
        loss_history.append({"epoch": ep, "loss": round(avg_ep_loss, 6), "trained_tokens": trained_tokens})
        ckpt_dir = CHECKPOINTS_DIR / f"checkpoint-epoch-{ep}"
        ckpt_dir.mkdir(parents=True, exist_ok=True)
        model.save_pretrained(str(ckpt_dir), safe_serialization=True)
        with open(ckpt_dir / "trainer_state.json", "w", encoding="utf-8") as sf:
            json.dump({"epoch": ep, "loss": avg_ep_loss, "trained_tokens": trained_tokens, "saved_at": int(time.time())}, sf, indent=2)
        log("checkpoint", f"Saved real PyTorch checkpoint-epoch-{ep} (avg_loss={avg_ep_loss:.4f})")

    log("export", "Saving final trained weights (.safetensors) & tokenizer...", progress=92, trained_tokens=trained_tokens)
    model.save_pretrained(str(OUTPUT_DIR), safe_serialization=True)
    tokenizer.save_pretrained(str(OUTPUT_DIR))
    return {
        "trained_tokens": trained_tokens,
        "sample_count": len(samples),
        "final_loss": round(last_loss, 6),
        "loss_history": loss_history,
    }


def train_image_diffusion(dataset_path, base_model_ref):
    import torch
    import torch.nn.functional as F
    from PIL import Image
    from diffusers import StableDiffusionPipeline, DDPMScheduler
    from peft import LoraConfig, get_peft_model

    img_dir = WORK_DIR / "images"
    img_dir.mkdir(parents=True, exist_ok=True)
    if str(dataset_path).lower().endswith(".zip"):
        with zipfile.ZipFile(dataset_path, "r") as zf:
            zf.extractall(img_dir)
    else:
        shutil.copy2(dataset_path, img_dir / Path(dataset_path).name)

    image_paths = [p for p in img_dir.rglob("*") if p.suffix.lower() in (".png", ".jpg", ".jpeg", ".webp")]
    if not image_paths:
        raise RuntimeError("No training images (.png/.jpg/.webp) found in image dataset archive")
    log("dataset", f"Found {len(image_paths)} training images", progress=16)

    use_cuda = torch.cuda.is_available()
    dtype = torch.float16 if use_cuda else torch.float32
    pipe_kwargs = {"torch_dtype": dtype, "safety_checker": None}
    if HF_TOKEN:
        pipe_kwargs["token"] = HF_TOKEN

    log("model", f"Loading diffusion pipeline: {base_model_ref}", progress=22)
    if str(base_model_ref).lower().endswith((".safetensors", ".ckpt")) and os.path.exists(str(base_model_ref)):
        pipe = StableDiffusionPipeline.from_single_file(str(base_model_ref), **pipe_kwargs)
    else:
        pipe = StableDiffusionPipeline.from_pretrained(base_model_ref, **pipe_kwargs)

    device = torch.device("cuda" if use_cuda else "cpu")
    vae = pipe.vae.to(device).eval()
    text_encoder = pipe.text_encoder.to(device).eval()
    unet = pipe.unet.to(device)
    tokenizer = pipe.tokenizer
    noise_scheduler = DDPMScheduler.from_config(pipe.scheduler.config)

    vae.requires_grad_(False)
    text_encoder.requires_grad_(False)
    if METHOD != "full":
        lora_cfg = LoraConfig(r=8, lora_alpha=16, target_modules=["to_q", "to_k", "to_v", "to_out.0"])
        unet = get_peft_model(unet, lora_cfg)
    unet.train()

    optimizer = torch.optim.AdamW([p for p in unet.parameters() if p.requires_grad], lr=1e-4)
    total_steps = max(1, len(image_paths) * EPOCHS)
    step = 0
    trained_tokens = 0
    loss_history = []
    last_loss = 0.0

    for ep in range(1, EPOCHS + 1):
        ep_loss = 0.0
        for img_path in image_paths:
            cap_path = img_path.with_suffix(".txt")
            caption = cap_path.read_text(encoding="utf-8", errors="ignore").strip() if cap_path.exists() else img_path.stem.replace("_", " ")
            img = Image.open(img_path).convert("RGB").resize((512, 512))
            arr = torch.tensor(list(img.getdata()), dtype=torch.float32).view(512, 512, 3).permute(2, 0, 1).unsqueeze(0)
            pixel_values = (arr / 127.5 - 1.0).to(device=device, dtype=dtype)

            with torch.no_grad():
                latents = vae.encode(pixel_values).latent_dist.sample() * vae.config.scaling_factor
                tok = tokenizer(caption, padding="max_length", max_length=tokenizer.model_max_length, truncation=True, return_tensors="pt").input_ids.to(device)
                encoder_hidden_states = text_encoder(tok)[0]

            noise = torch.randn_like(latents)
            timesteps = torch.randint(0, noise_scheduler.config.num_train_timesteps, (1,), device=device).long()
            noisy_latents = noise_scheduler.add_noise(latents, noise, timesteps)
            noise_pred = unet(noisy_latents, timesteps, encoder_hidden_states).sample
            loss = F.mse_loss(noise_pred.float(), noise.float(), reduction="mean")
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()

            step += 1
            raw_loss = float(loss.detach().cpu().item())
            last_loss = raw_loss
            ep_loss += raw_loss
            trained_tokens += 256
            if step == 1 or step %% max(1, total_steps // 15) == 0 or step == total_steps:
                pct = 30 + int(60.0 * step / total_steps)
                log("train", f"Epoch {ep}/{EPOCHS} | step {step}/{total_steps} | mse_loss={raw_loss:.4f}", progress=min(90, pct), trained_tokens=trained_tokens)

        avg_loss = ep_loss / max(1, len(image_paths))
        loss_history.append({"epoch": ep, "loss": round(avg_loss, 6), "trained_tokens": trained_tokens})
        ckpt_dir = CHECKPOINTS_DIR / f"checkpoint-epoch-{ep}"
        ckpt_dir.mkdir(parents=True, exist_ok=True)
        unet.save_pretrained(str(ckpt_dir), safe_serialization=True)
        log("checkpoint", f"Saved diffusion checkpoint-epoch-{ep} (avg_loss={avg_loss:.4f})")

    log("export", "Saving trained diffusion LoRA / UNet weights (.safetensors)...", progress=92, trained_tokens=trained_tokens)
    unet.save_pretrained(str(OUTPUT_DIR), safe_serialization=True)
    return {
        "trained_tokens": trained_tokens,
        "sample_count": len(image_paths),
        "final_loss": round(last_loss, 6),
        "loss_history": loss_history,
    }


def package_zip(metrics, started_at):
    finished_at = int(time.time())
    details = {
        "job_id": JOB_ID,
        "fine_tuned_model": FINE_TUNED_MODEL,
        "base_model": BASE_MODEL,
        "task_type": TASK_TYPE,
        "method": METHOD,
        "epochs": EPOCHS,
        "dataset_file": DATASET_NAME,
        "trained_tokens": metrics["trained_tokens"],
        "sample_count": metrics["sample_count"],
        "final_loss": metrics["final_loss"],
        "loss_history": metrics["loss_history"],
        "started_at": started_at,
        "finished_at": finished_at,
        "elapsed_seconds": max(1, finished_at - started_at),
        "real_training": True,
    }
    details_path = WORK_DIR / "training_details.json"
    details_path.write_text(json.dumps(details, indent=2), encoding="utf-8")

    with zipfile.ZipFile(OUTPUT_ZIP_PATH, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for f in OUTPUT_DIR.rglob("*"):
            if f.is_file():
                zf.write(f, arcname=str(f.relative_to(OUTPUT_DIR)).replace("\\", "/"))
        for f in CHECKPOINTS_DIR.rglob("*"):
            if f.is_file():
                zf.write(f, arcname="checkpoints/" + str(f.relative_to(CHECKPOINTS_DIR)).replace("\\", "/"))
        zf.write(details_path, arcname="training_details.json")
        if LOG_FILE.exists():
            zf.write(LOG_FILE, arcname="training.log")

    size_mb = os.path.getsize(OUTPUT_ZIP_PATH) / (1024 * 1024)
    log("export", f"Packaged real fine-tuned archive ({size_mb:.2f} MB)", progress=96, trained_tokens=metrics["trained_tokens"])


def upload_artifact_zip(metrics):
    if not ARTIFACT_URL:
        return
    log("upload", "Uploading trained model archive (.zip) to platform storage...", progress=98, trained_tokens=metrics["trained_tokens"])
    boundary = "----NewAPIFTBoundary" + str(int(time.time()))
    with open(OUTPUT_ZIP_PATH, "rb") as f:
        data = f.read()
    parts = [
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"token\"\r\n\r\n{CALLBACK_TOKEN}\r\n".encode(),
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"trained_tokens\"\r\n\r\n{metrics['trained_tokens']}\r\n".encode(),
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"{JOB_ID}.zip\"\r\nContent-Type: application/zip\r\n\r\n".encode(),
        data,
        f"\r\n--{boundary}--\r\n".encode(),
    ]
    body = b"".join(parts)
    req = urllib.request.Request(
        ARTIFACT_URL,
        data=body,
        headers={
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "Authorization": f"Bearer {CALLBACK_TOKEN}",
        },
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=600) as resp:
        resp.read()


def main():
    started_at = int(time.time())
    try:
        log("boot", f"Starting real fine-tuning job {JOB_ID} (task={TASK_TYPE}, method={METHOD}, epochs={EPOCHS})", progress=2)
        ensure_packages()

        ds_ext = Path(DATASET_NAME).suffix or ".jsonl"
        local_ds = WORK_DIR / ("dataset" + ds_ext)
        env_ds = os.environ.get("FT_LOCAL_DATASET", "").strip()
        if env_ds and os.path.exists(env_ds):
            local_ds = Path(env_ds)
            log("dataset", f"Using local staged dataset ({local_ds.name})", progress=12)
        elif os.path.exists("dataset" + ds_ext):
            local_ds = Path("dataset" + ds_ext)
            log("dataset", f"Using uploaded runtime dataset ({local_ds.name})", progress=12)
        else:
            download_file(DATASET_URL, local_ds, f"dataset ({DATASET_NAME})")

        base_ref = BASE_MODEL
        env_bm = os.environ.get("FT_LOCAL_BASE_MODEL", "").strip()
        if env_bm and os.path.exists(env_bm):
            local_bm = Path(env_bm)
            log("model", f"Using uploaded local base weights ({local_bm.name})", progress=14)
            if str(local_bm).lower().endswith(".zip"):
                bm_dir = WORK_DIR / "base_model_dir"
                bm_dir.mkdir(parents=True, exist_ok=True)
                with zipfile.ZipFile(local_bm, "r") as zf:
                    zf.extractall(bm_dir)
                base_ref = str(bm_dir)
            else:
                base_ref = str(local_bm)
        elif BASE_MODEL_URL:
            bm_name = Path(BASE_MODEL).name or "base_model.safetensors"
            local_bm = WORK_DIR / bm_name
            download_file(BASE_MODEL_URL, local_bm, f"base model ({bm_name})")
            if str(local_bm).lower().endswith(".zip"):
                bm_dir = WORK_DIR / "base_model_dir"
                bm_dir.mkdir(parents=True, exist_ok=True)
                with zipfile.ZipFile(local_bm, "r") as zf:
                    zf.extractall(bm_dir)
                base_ref = str(bm_dir)
            else:
                base_ref = str(local_bm)

        if TASK_TYPE == "image":
            metrics = train_image_diffusion(local_ds, base_ref)
        else:
            metrics = train_text_or_vision(local_ds, base_ref)

        package_zip(metrics, started_at)
        upload_artifact_zip(metrics)
        Path("ft_done.json").write_text(json.dumps({"status": "succeeded", **metrics}), encoding="utf-8")
        _post_callback({
            "status": "succeeded",
            "progress": 100,
            "trained_tokens": metrics["trained_tokens"],
            "fine_tuned_model": FINE_TUNED_MODEL,
        })
        log("done", f"Real fine-tuning completed (final_loss={metrics['final_loss']}, trained_tokens={metrics['trained_tokens']:,})", progress=100, trained_tokens=metrics["trained_tokens"])
    except Exception as exc:
        err_text = f"{exc}\n{traceback.format_exc()[-800:]}"
        log("error", f"Training failed: {exc}")
        Path("ft_error.txt").write_text(err_text, encoding="utf-8")
        _post_callback({"status": "failed", "error": str(exc)})
        sys.exit(1)
    finally:
        gc.collect()


if __name__ == "__main__":
    main()
`,
		pyStr(cfg.JobID),
		pyStr(cfg.FineTunedModel),
		pyStr(cfg.BaseModel),
		pyStr(cfg.BaseModelURL),
		pyStr(cfg.ModelSource),
		pyStr(cfg.DatasetURL),
		pyStr(cfg.DatasetName),
		pyStr(task),
		pyStr(method),
		epochs,
		pyStr(cfg.Accelerator),
		pyStr(cfg.CallbackURL),
		pyStr(cfg.ArtifactURL),
		pyStr(cfg.CallbackToken),
		pyStr(cfg.HFToken),
		pyStr(outZip),
	)
}
