import json

data = json.load(open(r"S:\tests\benchmark_results.json", encoding="utf-8"))
images = [d for d in data if d.get("kind") == "image"]
videos = [d for d in data if d.get("kind") == "video"]

print("=== IMAGE MODELS BENCHMARK ===")
for im in sorted(images, key=lambda x: (x.get("status") != "SUCCESS", x.get("latency_s", 999))):
    status_str = "OK" if im["status"] == "SUCCESS" else im["status"]
    print(f"| `{im['model']}` | {im.get('params','-')} | {im.get('latency_s','-')}s | {status_str} |")

print("\n=== VIDEO MODELS BENCHMARK ===")
for vm in sorted(videos, key=lambda x: (x.get("status") != "SUCCESS", x.get("latency_s", 999))):
    status_str = "OK" if vm["status"] == "SUCCESS" else vm["status"]
    print(f"| `{vm['model']}` | {vm.get('params','-')} | {vm.get('latency_s','-')}s | {status_str} |")
