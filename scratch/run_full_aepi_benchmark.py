import os
import sys
import json
import time
import shutil
import subprocess
import threading
import urllib.request
import urllib.error
from concurrent.futures import ThreadPoolExecutor, as_completed
from PIL import Image, ImageDraw, ImageFont

sys.stdout.reconfigure(line_buffering=True)

API_KEY = "sk_live_0ajK-kVJH-cUn0bF992kXgumUPOFU1oJaMisANV2Ei0"
BASE_URL = "https://videoapi.aerobox.eu.cc"
TESTS_DIR = r"S:\tests"
os.makedirs(TESTS_DIR, exist_ok=True)

# Ensure arial.ttf is in TESTS_DIR for ffmpeg drawtext without drive colon issues
font_src = r"C:\Windows\Fonts\arial.ttf"
font_dest = os.path.join(TESTS_DIR, "arial.ttf")
if os.path.exists(font_src) and not os.path.exists(font_dest):
    try:
        shutil.copy(font_src, font_dest)
    except Exception as e:
        print(f"Font copy notice: {e}", flush=True)

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "X-API-Key": API_KEY,
    "Content-Type": "application/json",
    "Accept": "application/json",
    "User-Agent": "curl/8.4.0",
}

results_lock = threading.Lock()
results_file = os.path.join(TESTS_DIR, "benchmark_results.json")

def load_results():
    with results_lock:
        if os.path.exists(results_file):
            try:
                with open(results_file, "r", encoding="utf-8") as f:
                    return json.load(f)
            except Exception:
                return []
        return []

def record_result(item):
    with results_lock:
        data = []
        if os.path.exists(results_file):
            try:
                with open(results_file, "r", encoding="utf-8") as f:
                    data = json.load(f)
            except Exception:
                data = []
        # Update or append
        existing_idx = next((i for i, r in enumerate(data) if r.get("key") == item.get("key")), -1)
        if existing_idx >= 0:
            data[existing_idx] = item
        else:
            data.append(item)
        with open(results_file, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2, ensure_ascii=False)

def annotate_image(image_path, model_id):
    try:
        img = Image.open(image_path).convert("RGBA")
        overlay = Image.new("RGBA", img.size, (255, 255, 255, 0))
        draw = ImageDraw.Draw(overlay)
        text = f"Model: {model_id}"
        font_size = max(18, int(img.height * 0.035))
        try:
            font = ImageFont.truetype(font_dest if os.path.exists(font_dest) else "arial.ttf", font_size)
        except Exception:
            font = ImageFont.load_default()
        bbox = draw.textbbox((0, 0), text, font=font)
        tw, th = bbox[2] - bbox[0], bbox[3] - bbox[1]
        x, y = 24, img.height - th - 24
        draw.rectangle([x - 10, y - 6, x + tw + 10, y + th + 6], fill=(0, 0, 0, 180))
        draw.text((x, y), text, font=font, fill=(255, 255, 255, 255))
        out = Image.alpha_composite(img, overlay)
        out.convert("RGB").save(image_path, quality=95)
    except Exception as e:
        print(f"Error annotating image {image_path}: {e}", flush=True)

def annotate_video(video_path, model_id):
    base_name = os.path.basename(video_path)
    tmp_name = base_name + ".annotated.mp4"
    tmp_path = os.path.join(TESTS_DIR, tmp_name)
    safe_id = model_id.replace(":", "\\:").replace("'", "\\'").replace("%", "\\%")
    vf = f"drawtext=fontfile=arial.ttf:text='Model\\: {safe_id}':fontcolor=white:fontsize=22:box=1:boxcolor=black@0.65:x=24:y=h-th-24"
    cmd = ["ffmpeg", "-y", "-i", base_name, "-vf", vf, "-codec:a", "copy", tmp_name]
    try:
        subprocess.run(cmd, cwd=TESTS_DIR, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=120, check=True)
        if os.path.exists(tmp_path) and os.path.getsize(tmp_path) > 0:
            os.replace(tmp_path, video_path)
    except Exception as e:
        if os.path.exists(tmp_path):
            try:
                os.remove(tmp_path)
            except Exception:
                pass
        print(f"Video annotation warning for {video_path}: {e}", flush=True)

def fetch_models():
    req = urllib.request.Request(f"{BASE_URL}/v1/models", headers=HEADERS)
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    return data.get("models", [])

def submit_job(payload):
    data_bytes = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(f"{BASE_URL}/v1/jobs", headers=HEADERS, data=data_bytes)
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        err_msg = e.read().decode()
        return {"error": f"HTTP {e.code}: {err_msg}"}
    except Exception as e:
        return {"error": str(e)}

def poll_job(job_id, timeout_sec=240):
    start = time.time()
    while time.time() - start < timeout_sec:
        req = urllib.request.Request(f"{BASE_URL}/v1/jobs/{job_id}", headers=HEADERS)
        try:
            with urllib.request.urlopen(req) as resp:
                data = json.loads(resp.read().decode())
                status = data.get("status")
                if status in ("done", "error"):
                    return data
        except Exception:
            pass
        time.sleep(3)
    return {"status": "timeout", "error": f"Polling timed out after {timeout_sec}s"}

def download_file(file_url, dest_path):
    if file_url.startswith("/"):
        file_url = BASE_URL + file_url
    req = urllib.request.Request(file_url, headers=HEADERS)
    with urllib.request.urlopen(req) as resp, open(dest_path, "wb") as f:
        f.write(resp.read())

def test_image_task(im):
    mid = im["id"]
    key = f"image_{mid}"
    current_results = load_results()
    if any(r.get("key") == key for r in current_results):
        print(f"[SKIP] Image model already tested: {mid}", flush=True)
        return

    print(f"[IMAGE] Starting: {mid}...", flush=True)
    t0 = time.time()
    res = submit_job({
        "operation": "image",
        "model": mid,
        "prompt": "An artistic digital masterpiece with luminous lighting and fine detail",
    })
    
    job_id = res.get("id")
    if not job_id:
        err = res.get("error", "No job ID returned")
        print(f"[IMAGE FAIL] Submit failed for {mid}: {err}", flush=True)
        record_result({
            "key": key,
            "model": mid,
            "kind": "image",
            "params": "1k auto",
            "latency_s": round(time.time() - t0, 1),
            "status": f"FAIL: {err[:60]}",
            "file": None
        })
        return

    job_data = poll_job(job_id, timeout_sec=150)
    latency = round(time.time() - t0, 1)
    status = job_data.get("status")
    output = job_data.get("output")

    if status == "done" and output and output.get("url"):
        out_file = os.path.join(TESTS_DIR, f"{mid}.jpg")
        try:
            download_file(output["url"], out_file)
            annotate_image(out_file, mid)
            print(f"[IMAGE SUCCESS] {mid} -> {out_file} ({latency}s)", flush=True)
            record_result({
                "key": key,
                "model": mid,
                "kind": "image",
                "params": f"{output.get('width', '')}x{output.get('height', '')}",
                "latency_s": latency,
                "status": "SUCCESS",
                "file": os.path.basename(out_file)
            })
        except Exception as e:
            print(f"[IMAGE ERR] Download error {mid}: {e}", flush=True)
            record_result({
                "key": key,
                "model": mid,
                "kind": "image",
                "params": "auto",
                "latency_s": latency,
                "status": f"DOWNLOAD_ERR: {str(e)[:40]}",
                "file": None
            })
    else:
        err = job_data.get("error") or status
        print(f"[IMAGE FAIL] {mid} -> {err} ({latency}s)", flush=True)
        record_result({
            "key": key,
            "model": mid,
            "kind": "image",
            "params": "auto",
            "latency_s": latency,
            "status": f"FAIL: {str(err)[:60]}",
            "file": None
        })

def test_video_task(vm, d):
    mid = vm["id"]
    key = f"video_{mid}_{d}s"
    current_results = load_results()
    if any(r.get("key") == key for r in current_results):
        print(f"[SKIP] Video model already tested: {mid} ({d}s)", flush=True)
        return

    print(f"[VIDEO] Starting: {mid} ({d}s, 16:9)...", flush=True)
    t0 = time.time()
    res = submit_job({
        "operation": "create",
        "model": mid,
        "duration": d,
        "aspect": "16:9",
        "prompt": "Cinematic aerial camera gliding smoothly over misty mountain peaks at sunset",
    })

    job_id = res.get("id")
    if not job_id:
        err = res.get("error", "No job ID returned")
        print(f"[VIDEO FAIL] Submit failed for {mid} ({d}s): {err}", flush=True)
        record_result({
            "key": key,
            "model": mid,
            "kind": "video",
            "params": f"{d}s 16:9",
            "latency_s": round(time.time() - t0, 1),
            "status": f"FAIL: {err[:60]}",
            "file": None
        })
        return

    job_data = poll_job(job_id, timeout_sec=300)
    latency = round(time.time() - t0, 1)
    status = job_data.get("status")
    output = job_data.get("output")

    if status == "done" and output and output.get("url"):
        out_file = os.path.join(TESTS_DIR, f"{mid}_{d}s.mp4")
        try:
            download_file(output["url"], out_file)
            annotate_video(out_file, f"{mid} ({d}s)")
            print(f"[VIDEO SUCCESS] {mid} ({d}s) -> {out_file} ({latency}s)", flush=True)
            record_result({
                "key": key,
                "model": mid,
                "kind": "video",
                "params": f"{d}s 16:9",
                "latency_s": latency,
                "status": "SUCCESS",
                "file": os.path.basename(out_file)
            })
        except Exception as e:
            print(f"[VIDEO ERR] Download error {mid} ({d}s): {e}", flush=True)
            record_result({
                "key": key,
                "model": mid,
                "kind": "video",
                "params": f"{d}s 16:9",
                "latency_s": latency,
                "status": f"DOWNLOAD_ERR: {str(e)[:40]}",
                "file": None
            })
    else:
        err = job_data.get("error") or status
        print(f"[VIDEO FAIL] {mid} ({d}s) -> {err} ({latency}s)", flush=True)
        record_result({
            "key": key,
            "model": mid,
            "kind": "video",
            "params": f"{d}s 16:9",
            "latency_s": latency,
            "status": f"FAIL: {str(err)[:60]}",
            "file": None
        })

def run_benchmark():
    models = fetch_models()
    image_models = [m for m in models if m.get("kind") == "image"]
    video_models = [m for m in models if m.get("kind") == "video"]

    print(f"Total image models: {len(image_models)}, video models: {len(video_models)}", flush=True)

    # 1. Run Image Models with ThreadPool (5 workers)
    print("=== Launching Image Benchmark (5 workers) ===", flush=True)
    with ThreadPoolExecutor(max_workers=5) as executor:
        futures = [executor.submit(test_image_task, im) for im in image_models]
        for f in as_completed(futures):
            try:
                f.result()
            except Exception as e:
                print(f"Image worker exception: {e}", flush=True)

    # 2. Run Video Models with ThreadPool (4 workers)
    print("=== Launching Video Benchmark (4 workers) ===", flush=True)
    video_tasks = []
    for vm in video_models:
        mid = vm["id"]
        durations = vm.get("durations") or [4, 8, 15]
        durations = sorted(list(set([int(d) for d in durations])))
        selected_durs = [durations[0]]
        if len(durations) > 2:
            mid_val = durations[len(durations)//2]
            if mid_val not in selected_durs:
                selected_durs.append(mid_val)
            if durations[-1] not in selected_durs:
                selected_durs.append(durations[-1])
        elif len(durations) == 2:
            selected_durs.append(durations[-1])
        for d in selected_durs:
            video_tasks.append((vm, d))

    with ThreadPoolExecutor(max_workers=4) as executor:
        futures = [executor.submit(test_video_task, vm, d) for vm, d in video_tasks]
        for f in as_completed(futures):
            try:
                f.result()
            except Exception as e:
                print(f"Video worker exception: {e}", flush=True)

    print("=== Benchmark Finished ===", flush=True)

if __name__ == "__main__":
    run_benchmark()
