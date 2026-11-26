package finetunescript

import (
	"fmt"
	"strings"
)

// Config defines parameters for generating a real PyTorch + Transformers + PEFT /
// Diffusers fine-tuning Python script that runs on Modal, Google Colab, Kaggle, or RunPod.
type Config struct {
	JobID               string
	FineTunedModel      string
	BaseModel           string // HF repo ID, HF repo/subfile, direct URL, or filename when BaseModelURL is provided
	BaseModelURL        string // Optional authenticated server download URL for locally uploaded base weights
	ModelSource         string // huggingface | url | upload
	DatasetURL          string // Authenticated server download URL for training dataset
	DatasetName         string
	TaskType            string // text | image | audio | vision
	Method              string // lora | qlora | sft | dpo | dreambooth | textual_inversion | full
	Epochs              int
	Accelerator         string
	CallbackURL         string // POST JSON status/progress/log_line updates
	CheckpointURL       string // Legacy single-shot checkpoint URL (fallback)
	UploadChunkURL      string // POST raw 16 MB streaming chunks (supports 2 TB+ through Cloudflare/Nginx)
	ResumeCheckpointURL string // GET checkpoint_latest.zip when resuming after provider interruption
	ArtifactURL         string // Legacy single-shot artifact URL (fallback)
	CallbackToken       string
	HFToken             string
	OutputZipPath       string // Local path where output.zip is saved (default: "output.zip")
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
// image tasks), continuously saves & uploads resumable checkpoints via 16 MB
// streaming chunks (capable of multi-TB uploads without 413 errors or RAM bloat),
// writes trained weights + checkpoints + training_details.json + training.log
// into output.zip, streams progress callbacks, and uploads the ZIP.
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
"""Real Fine-Tuning Worker Script (PyTorch + Transformers + PEFT / Diffusers) with 16MB chunked 2TB+ uploads & resume."""
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
import urllib.error
import urllib.parse
import urllib.request
import uuid
import zipfile
from pathlib import Path

os.environ["TQDM_DISABLE"] = "1"
os.environ["HF_HUB_DISABLE_PROGRESS_BARS"] = "1"
os.environ["TOKENIZERS_PARALLELISM"] = "false"

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
CHECKPOINT_URL = %s
UPLOAD_CHUNK_URL = %s
RESUME_CHECKPOINT_URL = %s
ARTIFACT_URL = %s
CALLBACK_TOKEN = %s
HF_TOKEN = %s
OUTPUT_ZIP_PATH = %s

CHUNK_BYTES = 16 * 1024 * 1024  # 16 MB chunks: bypasses Cloudflare/Nginx 100MB caps & handles 2TB+ with constant RAM
WORK_DIR = Path("ft_work_" + JOB_ID)
WORK_DIR.mkdir(parents=True, exist_ok=True)
LOG_FILE = WORK_DIR / "training.log"
CHECKPOINTS_DIR = WORK_DIR / "checkpoints"
CHECKPOINTS_DIR.mkdir(parents=True, exist_ok=True)
OUTPUT_DIR = WORK_DIR / "model_out"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
LATEST_CKPT_ZIP = Path("checkpoint_latest.zip")
LATEST_CKPT_META = Path("checkpoint_meta.json")


def split_hf_repo_and_file(ref):
    """Split 'owner/repo/path/file.safetensors' -> ('owner/repo', 'path/file.safetensors')."""
    s = str(ref or "").strip()
    if not s or os.path.exists(s) or s.startswith(("http://", "https://", "/", ".")):
        return s, ""
    parts = [p for p in s.split("/") if p]
    if len(parts) >= 3:
        return f"{parts[0]}/{parts[1]}", "/".join(parts[2:])
    return s, ""


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


def download_file(url, dst_path, label, allow_404=False):
    if not url:
        return None
    log("download", f"Downloading {label}...")
    headers = {"User-Agent": "new-api-finetune-worker/1.0"}
    if CALLBACK_TOKEN and ("/deployments/fine_tuning/" in url):
        headers["Authorization"] = f"Bearer {CALLBACK_TOKEN}"
    elif HF_TOKEN and "huggingface.co" in url:
        headers["Authorization"] = f"Bearer {HF_TOKEN}"
    req = urllib.request.Request(url, headers=headers)
    try:
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
    except urllib.error.HTTPError as he:
        if allow_404 and he.code == 404:
            return None
        raise


def _zip_compression_for(file_path):
    """Store already high-entropy binary weights uncompressed (ZIP_STORED) for 30x faster packaging."""
    low = str(file_path).lower()
    if low.endswith((".safetensors", ".bin", ".pt", ".pth", ".ckpt", ".zip", ".gz", ".png", ".jpg", ".webp")):
        return zipfile.ZIP_STORED
    return zipfile.ZIP_DEFLATED


def _upload_chunked(kind, file_path, extra_params=None, log_progress=False):
    """Stream any size file (up to 2 TB+) in 16 MB raw binary chunks with per-chunk retries."""
    if not UPLOAD_CHUNK_URL or not os.path.exists(file_path):
        return False
    total_size = os.path.getsize(file_path)
    if total_size <= 0:
        return False

    upload_id = uuid.uuid4().hex[:12]
    sep = "&" if "?" in UPLOAD_CHUNK_URL else "?"
    offset = 0
    last_logged_bucket = 0

    with open(file_path, "rb") as f:
        while offset < total_size:
            chunk = f.read(CHUNK_BYTES)
            if not chunk:
                break
            is_final = (offset + len(chunk)) >= total_size
            params = {
                "kind": kind,
                "upload_id": upload_id,
                "offset": str(offset),
                "total_size": str(total_size),
                "final": "1" if is_final else "0",
            }
            if is_final and extra_params:
                for k, v in extra_params.items():
                    params[k] = str(v)

            target_url = UPLOAD_CHUNK_URL + sep + urllib.parse.urlencode(params)
            last_err = None
            for attempt in range(6):
                try:
                    req = urllib.request.Request(
                        target_url,
                        data=chunk,
                        headers={
                            "Content-Type": "application/octet-stream",
                            "Content-Length": str(len(chunk)),
                            "Authorization": f"Bearer {CALLBACK_TOKEN}",
                        },
                        method="POST",
                    )
                    with urllib.request.urlopen(req, timeout=180) as resp:
                        resp.read()
                    last_err = None
                    break
                except Exception as exc:
                    last_err = exc
                    time.sleep(min(20, 2 * (attempt + 1)))
            if last_err is not None:
                raise last_err

            offset += len(chunk)
            if log_progress and total_size >= 64 * 1024 * 1024:
                pct_done = int((offset * 100) / total_size)
                bucket = (pct_done // 25) * 25
                if bucket >= 25 and bucket > last_logged_bucket and bucket < 100:
                    last_logged_bucket = bucket
                    log("upload", f"Uploaded {offset / (1024 * 1024):.1f} / {total_size / (1024 * 1024):.1f} MB ({bucket}%%)", progress=98)
    return True


def restore_resume_checkpoint():
    """Locate and extract any saved resume checkpoint (from env, local runtime, or server URL)."""
    resume_zip = None
    env_ck = os.environ.get("FT_LOCAL_RESUME_CHECKPOINT", "").strip()
    if env_ck and os.path.exists(env_ck) and os.path.getsize(env_ck) > 0:
        resume_zip = Path(env_ck)
    elif Path("resume_checkpoint.zip").exists() and Path("resume_checkpoint.zip").stat().st_size > 0:
        resume_zip = Path("resume_checkpoint.zip")
    elif RESUME_CHECKPOINT_URL:
        dl_dst = WORK_DIR / "resume_checkpoint.zip"
        try:
            got = download_file(RESUME_CHECKPOINT_URL, dl_dst, "latest saved checkpoint", allow_404=True)
            if got and os.path.exists(got) and os.path.getsize(got) > 0:
                resume_zip = Path(got)
        except Exception as exc:
            log("resume", f"No remote checkpoint retrieved ({exc}); starting fresh")

    if not resume_zip:
        return None

    extract_dir = WORK_DIR / "resumed_ckpt"
    if extract_dir.exists():
        shutil.rmtree(extract_dir, ignore_errors=True)
    extract_dir.mkdir(parents=True, exist_ok=True)
    try:
        with zipfile.ZipFile(resume_zip, "r") as zf:
            zf.extractall(extract_dir)
        state_file = extract_dir / "checkpoint_state.json"
        if not state_file.exists():
            return None
        state = json.loads(state_file.read_text(encoding="utf-8"))
        state["_dir"] = str(extract_dir)
        log(
            "resume",
            f"Restored saved checkpoint: completed_epoch={state.get('completed_epoch', 0)}, global_step={state.get('global_step', 0)}, trained_tokens={state.get('trained_tokens', 0):,}",
            progress=max(12, int(state.get("progress", 15))),
            trained_tokens=int(state.get("trained_tokens", 0)),
        )
        return state
    except Exception as exc:
        log("resume", f"Could not restore checkpoint archive ({exc}); starting fresh")
        return None


def save_and_upload_checkpoint(model, optimizer, completed_epoch, current_epoch, global_step, trained_tokens, loss_history, progress, is_epoch_end=False):
    """Persist resumable weights + optimizer state + progress into checkpoint_latest.zip and stream-sync in 16MB chunks."""
    import torch

    stage_dir = WORK_DIR / "ckpt_staging"
    if stage_dir.exists():
        shutil.rmtree(stage_dir, ignore_errors=True)
    model_dir = stage_dir / "checkpoint_model"
    model_dir.mkdir(parents=True, exist_ok=True)

    model.save_pretrained(str(model_dir), safe_serialization=True)
    try:
        torch.save(optimizer.state_dict(), str(stage_dir / "optimizer.pt"))
    except Exception:
        pass

    state = {
        "job_id": JOB_ID,
        "completed_epoch": int(completed_epoch),
        "current_epoch": int(current_epoch),
        "global_step": int(global_step),
        "trained_tokens": int(trained_tokens),
        "loss_history": loss_history,
        "progress": int(progress),
        "saved_at": int(time.time()),
    }
    (stage_dir / "checkpoint_state.json").write_text(json.dumps(state, indent=2), encoding="utf-8")

    tmp_zip = Path(str(LATEST_CKPT_ZIP) + ".tmp")
    with zipfile.ZipFile(tmp_zip, "w", allowZip64=True) as zf:
        for f in stage_dir.rglob("*"):
            if f.is_file():
                zf.write(f, arcname=str(f.relative_to(stage_dir)).replace("\\", "/"), compress_type=_zip_compression_for(f))
        if LOG_FILE.exists():
            zf.write(LOG_FILE, arcname="training.log", compress_type=zipfile.ZIP_DEFLATED)
    os.replace(tmp_zip, LATEST_CKPT_ZIP)

    if is_epoch_end and completed_epoch > 0:
        ep_dir = CHECKPOINTS_DIR / f"checkpoint-epoch-{completed_epoch}"
        if ep_dir.exists():
            shutil.rmtree(ep_dir, ignore_errors=True)
        shutil.copytree(model_dir, ep_dir)
        (ep_dir / "trainer_state.json").write_text(json.dumps(state, indent=2), encoding="utf-8")

    size_bytes = os.path.getsize(LATEST_CKPT_ZIP)
    state["size_bytes"] = size_bytes
    uploaded = False
    if UPLOAD_CHUNK_URL:
        try:
            uploaded = _upload_chunked(
                "checkpoint",
                str(LATEST_CKPT_ZIP),
                {
                    "epoch": completed_epoch,
                    "step": global_step,
                    "trained_tokens": trained_tokens,
                    "progress": progress,
                },
                log_progress=False,
            )
        except Exception as exc:
            log("checkpoint", f"Checkpoint saved locally (remote sync deferred: {exc})")
    state["uploaded"] = uploaded
    LATEST_CKPT_META.write_text(json.dumps(state), encoding="utf-8")

    size_mb = size_bytes / (1024 * 1024)
    sync_tag = "synced to server" if uploaded else "staged locally"
    log(
        "checkpoint",
        f"Saved resumable checkpoint (epoch={current_epoch}/{EPOCHS}, step={global_step}, {size_mb:.2f} MB, {sync_tag})",
        progress=progress,
        trained_tokens=trained_tokens,
    )


def format_openai_messages(messages):
    parts = []
    for m in messages:
        if not isinstance(m, dict):
            continue
        role = str(m.get("role", "user")).strip().lower()
        content = m.get("content", "")
        if isinstance(content, list):
            content = " ".join(
                str(p.get("text", "")) for p in content if isinstance(p, dict) and p.get("type") == "text"
            )
        elif content is None:
            content = ""
        if not content and m.get("tool_calls"):
            content = json.dumps(m["tool_calls"], ensure_ascii=False)
        parts.append(f"<|{role}|>\n{content}")
    return "\n".join(parts)


def load_openai_jsonl_samples(dataset_path):
    """Strictly parse OpenAI fine-tuning JSONL (Chat messages or DPO format)."""
    texts = []
    with open(dataset_path, "r", encoding="utf-8-sig", errors="strict") as f:
        for line_num, raw_line in enumerate(f, start=1):
            line = raw_line.strip()
            if not line:
                continue
            obj = json.loads(line)
            if not isinstance(obj, dict):
                raise RuntimeError(f"Line {line_num}: expected JSON object in OpenAI JSONL format")
            if METHOD == "dpo":
                inp = obj.get("input") or {}
                pref = obj.get("preferred_output") or []
                if not isinstance(inp, dict) or not isinstance(inp.get("messages"), list) or not isinstance(pref, list) or not pref:
                    raise RuntimeError(f"Line {line_num}: invalid OpenAI DPO record")
                texts.append(format_openai_messages(inp["messages"] + pref))
            else:
                msgs = obj.get("messages")
                if not isinstance(msgs, list) or len(msgs) < 2:
                    raise RuntimeError(f"Line {line_num}: missing valid OpenAI 'messages' array")
                texts.append(format_openai_messages(msgs))
    texts = [t for t in texts if t and t.strip()]
    if not texts:
        raise RuntimeError("Dataset contains no usable OpenAI training examples")
    return texts


def train_text_or_vision(dataset_path, base_model_ref, resume_state=None):
    import torch
    from torch.utils.data import Dataset, DataLoader
    from transformers import AutoTokenizer, AutoModelForCausalLM, get_cosine_schedule_with_warmup
    from peft import LoraConfig, PeftModel, get_peft_model, prepare_model_for_kbit_training

    samples = load_openai_jsonl_samples(dataset_path)
    log("dataset", f"Parsed {len(samples)} OpenAI training examples from {DATASET_NAME}", progress=16)

    repo_id, subfile = split_hf_repo_and_file(base_model_ref)
    if subfile:
        log("model", f"Resolved Hugging Face repository {repo_id} (weight file hint: {subfile})", progress=18)

    tok_kwargs = {"trust_remote_code": True}
    if HF_TOKEN:
        tok_kwargs["token"] = HF_TOKEN
    log("model", f"Loading tokenizer & base model: {repo_id}", progress=20)
    tokenizer = AutoTokenizer.from_pretrained(repo_id, **tok_kwargs)
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
        model_kwargs["dtype"] = dtype
        if use_cuda:
            model_kwargs["device_map"] = "auto"

    def _load_causal_lm(source_ref):
        try:
            return AutoModelForCausalLM.from_pretrained(source_ref, **model_kwargs)
        except TypeError:
            if "dtype" in model_kwargs:
                fb = dict(model_kwargs)
                fb["torch_dtype"] = fb.pop("dtype")
                return AutoModelForCausalLM.from_pretrained(source_ref, **fb)
            raise

    resumed_model_dir = Path(resume_state["_dir"]) / "checkpoint_model" if resume_state else None
    if METHOD == "full" and resumed_model_dir and resumed_model_dir.exists():
        log("resume", "Loading full fine-tuned weights from saved checkpoint...", progress=24)
        model = _load_causal_lm(str(resumed_model_dir))
    else:
        model = _load_causal_lm(repo_id)
    model.config.use_cache = False

    if METHOD in ("lora", "qlora", "sft", "dpo"):
        if METHOD == "qlora" and use_cuda:
            model = prepare_model_for_kbit_training(model)
        if resumed_model_dir and (resumed_model_dir / "adapter_config.json").exists():
            log("resume", "Re-attaching saved PEFT adapter weights from checkpoint...", progress=26)
            model = PeftModel.from_pretrained(model, str(resumed_model_dir), is_trainable=True)
        else:
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
        log("model", f"PEFT {METHOD.upper()} adapter ready: trainable={trainable:,} / {total_p:,} ({100.0 * trainable / max(1, total_p):.2f}%%)", progress=28)
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
    gen = torch.Generator()
    gen.manual_seed(42)
    loader = DataLoader(SimpleTextDataset(encoded), batch_size=batch_size, shuffle=True, generator=gen)

    device = next(model.parameters()).device
    optimizer = torch.optim.AdamW([p for p in model.parameters() if p.requires_grad], lr=2e-4 if METHOD != "full" else 5e-5, weight_decay=0.01)
    steps_per_epoch = max(1, math.ceil(len(loader) / grad_accum))
    total_steps = max(1, steps_per_epoch * EPOCHS)
    scheduler = get_cosine_schedule_with_warmup(optimizer, num_warmup_steps=max(1, total_steps // 10), num_training_steps=total_steps)

    trained_tokens = int(resume_state.get("trained_tokens", 0)) if resume_state else 0
    loss_history = list(resume_state.get("loss_history", [])) if resume_state else []
    resumed_completed_epoch = int(resume_state.get("completed_epoch", 0)) if resume_state else 0
    resumed_global_step = int(resume_state.get("global_step", 0)) if resume_state else 0
    global_step = resumed_global_step
    last_loss = float(loss_history[-1]["loss"]) if loss_history else 0.0

    if resume_state:
        opt_file = Path(resume_state["_dir"]) / "optimizer.pt"
        if opt_file.exists():
            try:
                optimizer.load_state_dict(torch.load(str(opt_file), map_location=device))
            except Exception:
                pass
        for _ in range(resumed_global_step):
            scheduler.step()

    model.train()
    last_ckpt_time = time.time()

    for ep in range(resumed_completed_epoch + 1, EPOCHS + 1):
        ep_loss_sum = 0.0
        ep_batches = 0
        ep_step_base = (ep - 1) * steps_per_epoch
        steps_already_done_in_ep = max(0, resumed_global_step - ep_step_base) if ep == resumed_completed_epoch + 1 else 0
        skip_batches = steps_already_done_in_ep * grad_accum
        if skip_batches > 0:
            log("resume", f"Skipping {steps_already_done_in_ep} already-completed step(s) in epoch {ep}/{EPOCHS}")

        optimizer.zero_grad()
        for b_idx, batch in enumerate(loader, start=1):
            if b_idx <= skip_batches:
                continue
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
                pct = min(90, 30 + int(60.0 * global_step / total_steps))

                if global_step == 1 or global_step %% max(1, total_steps // 20) == 0 or global_step == total_steps:
                    lr = scheduler.get_last_lr()[0]
                    log("train", f"Epoch {ep}/{EPOCHS} | step {global_step}/{total_steps} | loss={raw_loss:.4f} | lr={lr:.2e} | tokens={trained_tokens:,}", progress=pct, trained_tokens=trained_tokens)

                if (global_step %% 25 == 0 or (time.time() - last_ckpt_time) >= 180) and global_step < total_steps:
                    save_and_upload_checkpoint(
                        model, optimizer, ep - 1, ep, global_step, trained_tokens, loss_history, pct, is_epoch_end=False
                    )
                    last_ckpt_time = time.time()

        avg_ep_loss = ep_loss_sum / max(1, ep_batches)
        loss_history.append({"epoch": ep, "loss": round(avg_ep_loss, 6), "trained_tokens": trained_tokens})
        pct = min(91, 30 + int(60.0 * global_step / total_steps))
        save_and_upload_checkpoint(
            model, optimizer, ep, ep, global_step, trained_tokens, loss_history, pct, is_epoch_end=True
        )
        last_ckpt_time = time.time()

    log("export", "Saving final trained weights (.safetensors) & tokenizer...", progress=93, trained_tokens=trained_tokens)
    model.save_pretrained(str(OUTPUT_DIR), safe_serialization=True)
    tokenizer.save_pretrained(str(OUTPUT_DIR))
    return {
        "trained_tokens": trained_tokens,
        "sample_count": len(samples),
        "final_loss": round(last_loss, 6),
        "loss_history": loss_history,
    }


def train_image_diffusion(dataset_path, base_model_ref, resume_state=None):
    import torch
    import torch.nn.functional as F
    from PIL import Image
    from diffusers import StableDiffusionPipeline, DDPMScheduler
    from peft import LoraConfig, PeftModel, get_peft_model

    img_dir = WORK_DIR / "images"
    img_dir.mkdir(parents=True, exist_ok=True)
    if str(dataset_path).lower().endswith(".zip"):
        with zipfile.ZipFile(dataset_path, "r") as zf:
            zf.extractall(img_dir)
    else:
        shutil.copy2(dataset_path, img_dir / Path(dataset_path).name)

    image_paths = sorted(p for p in img_dir.rglob("*") if p.suffix.lower() in (".png", ".jpg", ".jpeg", ".webp"))
    if not image_paths:
        raise RuntimeError("No training images (.png/.jpg/.webp) found in image dataset archive")
    log("dataset", f"Found {len(image_paths)} training images", progress=16)

    use_cuda = torch.cuda.is_available()
    dtype = torch.float16 if use_cuda else torch.float32
    pipe_kwargs = {"torch_dtype": dtype, "safety_checker": None}
    if HF_TOKEN:
        pipe_kwargs["token"] = HF_TOKEN

    repo_id, subfile = split_hf_repo_and_file(base_model_ref)
    resolved_ref = base_model_ref
    if subfile and subfile.lower().endswith((".safetensors", ".ckpt")):
        from huggingface_hub import hf_hub_download
        log("model", f"Downloading single diffusion weight file {repo_id}/{subfile}...", progress=20)
        resolved_ref = hf_hub_download(repo_id=repo_id, filename=subfile, token=HF_TOKEN or None)

    log("model", f"Loading diffusion pipeline: {resolved_ref}", progress=22)
    if str(resolved_ref).lower().endswith((".safetensors", ".ckpt")) and os.path.exists(str(resolved_ref)):
        pipe = StableDiffusionPipeline.from_single_file(str(resolved_ref), **pipe_kwargs)
    else:
        pipe = StableDiffusionPipeline.from_pretrained(repo_id, **pipe_kwargs)

    device = torch.device("cuda" if use_cuda else "cpu")
    vae = pipe.vae.to(device).eval()
    text_encoder = pipe.text_encoder.to(device).eval()
    unet = pipe.unet.to(device)
    tokenizer = pipe.tokenizer
    noise_scheduler = DDPMScheduler.from_config(pipe.scheduler.config)

    vae.requires_grad_(False)
    text_encoder.requires_grad_(False)
    resumed_model_dir = Path(resume_state["_dir"]) / "checkpoint_model" if resume_state else None
    if METHOD != "full":
        if resumed_model_dir and (resumed_model_dir / "adapter_config.json").exists():
            log("resume", "Re-attaching saved diffusion LoRA checkpoint...", progress=26)
            unet = PeftModel.from_pretrained(unet, str(resumed_model_dir), is_trainable=True)
        else:
            lora_cfg = LoraConfig(r=8, lora_alpha=16, target_modules=["to_q", "to_k", "to_v", "to_out.0"])
            unet = get_peft_model(unet, lora_cfg)
    elif resumed_model_dir and resumed_model_dir.exists():
        from diffusers import UNet2DConditionModel
        log("resume", "Loading saved full UNet checkpoint...", progress=26)
        unet = UNet2DConditionModel.from_pretrained(str(resumed_model_dir), torch_dtype=dtype).to(device)
    unet.train()

    optimizer = torch.optim.AdamW([p for p in unet.parameters() if p.requires_grad], lr=1e-4)
    total_steps = max(1, len(image_paths) * EPOCHS)
    resumed_completed_epoch = int(resume_state.get("completed_epoch", 0)) if resume_state else 0
    resumed_step = int(resume_state.get("global_step", 0)) if resume_state else 0
    step = resumed_step
    trained_tokens = int(resume_state.get("trained_tokens", 0)) if resume_state else 0
    loss_history = list(resume_state.get("loss_history", [])) if resume_state else []
    last_loss = float(loss_history[-1]["loss"]) if loss_history else 0.0

    if resume_state:
        opt_file = Path(resume_state["_dir"]) / "optimizer.pt"
        if opt_file.exists():
            try:
                optimizer.load_state_dict(torch.load(str(opt_file), map_location=device))
            except Exception:
                pass

    last_ckpt_time = time.time()
    for ep in range(resumed_completed_epoch + 1, EPOCHS + 1):
        ep_loss = 0.0
        ep_count = 0
        ep_step_base = (ep - 1) * len(image_paths)
        skip_imgs = max(0, resumed_step - ep_step_base) if ep == resumed_completed_epoch + 1 else 0
        for idx, img_path in enumerate(image_paths, start=1):
            if idx <= skip_imgs:
                continue
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
            ep_count += 1
            trained_tokens += 256
            pct = min(90, 30 + int(60.0 * step / total_steps))
            if step == 1 or step %% max(1, total_steps // 15) == 0 or step == total_steps:
                log("train", f"Epoch {ep}/{EPOCHS} | step {step}/{total_steps} | mse_loss={raw_loss:.4f}", progress=pct, trained_tokens=trained_tokens)
            if (step %% 25 == 0 or (time.time() - last_ckpt_time) >= 180) and step < total_steps:
                save_and_upload_checkpoint(unet, optimizer, ep - 1, ep, step, trained_tokens, loss_history, pct, is_epoch_end=False)
                last_ckpt_time = time.time()

        avg_loss = ep_loss / max(1, ep_count)
        loss_history.append({"epoch": ep, "loss": round(avg_loss, 6), "trained_tokens": trained_tokens})
        pct = min(91, 30 + int(60.0 * step / total_steps))
        save_and_upload_checkpoint(unet, optimizer, ep, ep, step, trained_tokens, loss_history, pct, is_epoch_end=True)
        last_ckpt_time = time.time()

    log("export", "Saving trained diffusion LoRA / UNet weights (.safetensors)...", progress=93, trained_tokens=trained_tokens)
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

    with zipfile.ZipFile(OUTPUT_ZIP_PATH, "w", allowZip64=True) as zf:
        for f in OUTPUT_DIR.rglob("*"):
            if f.is_file():
                zf.write(f, arcname=str(f.relative_to(OUTPUT_DIR)).replace("\\", "/"), compress_type=_zip_compression_for(f))
        for f in CHECKPOINTS_DIR.rglob("*"):
            if f.is_file():
                zf.write(f, arcname="checkpoints/" + str(f.relative_to(CHECKPOINTS_DIR)).replace("\\", "/"), compress_type=_zip_compression_for(f))
        zf.write(details_path, arcname="training_details.json", compress_type=zipfile.ZIP_DEFLATED)
        if LOG_FILE.exists():
            zf.write(LOG_FILE, arcname="training.log", compress_type=zipfile.ZIP_DEFLATED)

    size_bytes = os.path.getsize(OUTPUT_ZIP_PATH)
    log("export", f"Packaged real fine-tuned archive ({size_bytes / (1024 * 1024):.2f} MB)", progress=96, trained_tokens=metrics["trained_tokens"])
    return size_bytes


def upload_artifact_zip(metrics):
    if not UPLOAD_CHUNK_URL:
        return False
    log("upload", "Uploading trained model archive (.zip) via 16 MB streaming chunks...", progress=97, trained_tokens=metrics["trained_tokens"])
    ok = _upload_chunked(
        "artifact",
        OUTPUT_ZIP_PATH,
        {"trained_tokens": metrics["trained_tokens"]},
        log_progress=True,
    )
    if ok:
        log("upload", "Trained model archive (.zip) uploaded to platform storage", progress=99, trained_tokens=metrics["trained_tokens"])
    return ok


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

        resume_state = restore_resume_checkpoint()

        if TASK_TYPE == "image":
            metrics = train_image_diffusion(local_ds, base_ref, resume_state=resume_state)
        else:
            metrics = train_text_or_vision(local_ds, base_ref, resume_state=resume_state)

        zip_bytes = package_zip(metrics, started_at)
        uploaded = False
        try:
            uploaded = upload_artifact_zip(metrics)
        except Exception as up_exc:
            log("error", f"Artifact chunked upload error: {up_exc}")
            # If no proxy fallback is possible, surface upload error clearly without pretending training failed
            if not Path(OUTPUT_ZIP_PATH).exists():
                raise
            Path("ft_done.json").write_text(
                json.dumps({"status": "upload_retry", "uploaded": False, "output_zip_bytes": zip_bytes, "upload_error": str(up_exc), **metrics}),
                encoding="utf-8",
            )
            raise RuntimeError(f"artifact upload failed after training completed: {up_exc}")

        Path("ft_done.json").write_text(
            json.dumps({"status": "succeeded", "uploaded": uploaded, "output_zip_bytes": zip_bytes, **metrics}),
            encoding="utf-8",
        )
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
		pyStr(cfg.CheckpointURL),
		pyStr(cfg.UploadChunkURL),
		pyStr(cfg.ResumeCheckpointURL),
		pyStr(cfg.ArtifactURL),
		pyStr(cfg.CallbackToken),
		pyStr(cfg.HFToken),
		pyStr(outZip),
	)
}
