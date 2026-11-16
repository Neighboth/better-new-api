import os
import json
import time
import subprocess
import urllib.request
import urllib.error
from PIL import Image, ImageDraw, ImageFont

API_KEY = "sk_live_0ajK-kVJH-cUn0bF992kXgumUPOFU1oJaMisANV2Ei0"
BASE_URL = "https://videoapi.aerobox.eu.cc"
TESTS_DIR = r"S:\tests"
os.makedirs(TESTS_DIR, exist_ok=True)

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "X-API-Key": API_KEY,
    "Content-Type": "application/json",
    "Accept": "application/json",
    "User-Agent": "curl/8.4.0",
}

def annotate_image(image_path, model_id):
    try:
        img = Image.open(image_path).convert("RGBA")
        overlay = Image.new("RGBA", img.size, (255, 255, 255, 0))
        draw = ImageDraw.Draw(overlay)
        text = f"Model: {model_id}"
        font_size = max(18, int(img.height * 0.035))
        try:
            font = ImageFont.truetype("arial.ttf", font_size)
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
        print(f"Error annotating image {image_path}: {e}")

def annotate_video(video_path, model_id):
    tmp_path = video_path + ".annotated.mp4"
    vf = f"drawtext=text='Model\\: {model_id}':fontcolor=white:fontsize=22:box=1:boxcolor=black@0.65:boxborderw=6:x=24:y=h-th-24"
    cmd = ["ffmpeg", "-y", "-i", video_path, "-vf", vf, "-codec:a", "copy", tmp_path]
    try:
        subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60, check=True)
        if os.path.exists(tmp_path) and os.path.getsize(tmp_path) > 0:
            os.replace(tmp_path, video_path)
    except Exception as e:
        if os.path.exists(tmp_path):
            try:
                os.remove(tmp_path)
            except Exception:
                pass
        print(f"Video annotation warning for {video_path}: {e}")

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

def poll_job(job_id, timeout_sec=300):
    start = time.time()
    while time.time() - start < timeout_sec:
        req = urllib.request.Request(f"{BASE_URL}/v1/jobs/{job_id}", headers=HEADERS)
        try:
            with urllib.request.urlopen(req) as resp:
                data = json.loads(resp.read().decode())
                status = data.get("status")
                if status in ("done", "error"):
                    return data
        except Exception as e:
            pass
        time.sleep(3)
    return {"status": "timeout", "error": f"Polling timed out after {timeout_sec}s"}

def download_file(file_url, dest_path):
    if file_url.startswith("/"):
        file_url = BASE_URL + file_url
    req = urllib.request.Request(file_url, headers=HEADERS)
    with urllib.request.urlopen(req) as resp, open(dest_path, "wb") as f:
        f.write(resp.read())

print("AEPI benchmark helper initialized.")

def run_benchmark():
    models = fetch_models()
    image_models = [m for m in models if m.get("kind") == "image"]
    video_models = [m for m in models if m.get("kind") == "video"]
    
    results = []
    results_file = os.path.join(TESTS_DIR, "benchmark_results.json")
    if os.path.exists(results_file):
        try:
            with open(results_file, "r", encoding="utf-8") as f:
                results = json.load(f)
        except Exception:
            results = []

    tested_keys = {r.get("key") for r in results}

    print(f"Total image models: {len(image_models)}, video models: {len(video_models)}")

    # 1. Test Image Models
    for idx, im in enumerate(image_models):
        mid = im["id"]
        key = f"image_{mid}"
        if key in tested_keys:
            print(f"Skipping already tested image model: {mid}")
            continue

        print(f"[{idx+1}/{len(image_models)}] Testing Image Model: {mid}...")
        t0 = time.time()
        res = submit_job({
            "operation": "image",
            "model": mid,
            "prompt": "An artistic digital masterpiece with luminous lighting and fine detail",
        })
        
        job_id = res.get("id")
        if not job_id:
            err = res.get("error", "No job ID returned")
            print(f"  Submit failed for {mid}: {err}")
            results.append({
                "key": key,
                "model": mid,
                "kind": "image",
                "params": "1k auto",
                "latency_s": round(time.time() - t0, 1),
                "status": f"FAIL: {err[:60]}",
                "file": None
            })
            continue

        job_data = poll_job(job_id, timeout_sec=120)
        latency = round(time.time() - t0, 1)
        status = job_data.get("status")
        output = job_data.get("output")

        if status == "done" and output and output.get("url"):
            out_file = os.path.join(TESTS_DIR, f"{mid}.jpg")
            try:
                download_file(output["url"], out_file)
                annotate_image(out_file, mid)
                print(f"  Success: {mid} -> {out_file} ({latency}s)")
                results.append({
                    "key": key,
                    "model": mid,
                    "kind": "image",
                    "params": f"{output.get('width', '')}x{output.get('height', '')}",
                    "latency_s": latency,
                    "status": "SUCCESS",
                    "file": os.path.basename(out_file)
                })
            except Exception as e:
                print(f"  Download error for {mid}: {e}")
                results.append({
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
            print(f"  Failed: {mid} -> {err}")
            results.append({
                "key": key,
                "model": mid,
                "kind": "image",
                "params": "auto",
                "latency_s": latency,
                "status": f"FAIL: {str(err)[:60]}",
                "file": None
            })

        with open(results_file, "w", encoding="utf-8") as f:
            json.dump(results, f, indent=2)

    # 2. Test Video Models across shortest, mid, longest durations
    for idx, vm in enumerate(video_models):
        mid = vm["id"]
        durations = vm.get("durations") or [4, 8, 15]
        # shortest, mid, longest
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
            key = f"video_{mid}_{d}s"
            if key in tested_keys:
                print(f"Skipping already tested video model: {mid} ({d}s)")
                continue

            print(f"[{idx+1}/{len(video_models)}] Testing Video Model: {mid} (duration: {d}s, aspect: 16:9)...")
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
                print(f"  Submit failed for {mid} ({d}s): {err}")
                results.append({
                    "key": key,
                    "model": mid,
                    "kind": "video",
                    "params": f"{d}s 16:9",
                    "latency_s": round(time.time() - t0, 1),
                    "status": f"FAIL: {err[:60]}",
                    "file": None
                })
                continue

            job_data = poll_job(job_id, timeout_sec=240)
            latency = round(time.time() - t0, 1)
            status = job_data.get("status")
            output = job_data.get("output")

            if status == "done" and output and output.get("url"):
                out_file = os.path.join(TESTS_DIR, f"{mid}_{d}s.mp4")
                try:
                    download_file(output["url"], out_file)
                    annotate_video(out_file, f"{mid} ({d}s)")
                    print(f"  Success: {mid} ({d}s) -> {out_file} ({latency}s)")
                    results.append({
                        "key": key,
                        "model": mid,
                        "kind": "video",
                        "params": f"{d}s 16:9",
                        "latency_s": latency,
                        "status": "SUCCESS",
                        "file": os.path.basename(out_file)
                    })
                except Exception as e:
                    print(f"  Download error for {mid} ({d}s): {e}")
                    results.append({
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
                print(f"  Failed: {mid} ({d}s) -> {err}")
                results.append({
                    "key": key,
                    "model": mid,
                    "kind": "video",
                    "params": f"{d}s 16:9",
                    "latency_s": latency,
                    "status": f"FAIL: {str(err)[:60]}",
                    "file": None
                })

            with open(results_file, "w", encoding="utf-8") as f:
                json.dump(results, f, indent=2)

if __name__ == "__main__":
    run_benchmark()
