import os
import sys
import subprocess
import shutil
import zipfile
import json
import argparse
import urllib.request
import io

BUILD_TOOLS = r"C:\Users\simse\AppData\Local\Android\Sdk\build-tools\37.0.0"
PLATFORM_JAR = r"C:\Users\simse\AppData\Local\Android\Sdk\platforms\android-34\android.jar"
KEYTOOL = r"C:\Program Files\Java\jdk-21.0.10\bin\keytool.exe"
AAPT2 = os.path.join(BUILD_TOOLS, "aapt2.exe")
D8 = os.path.join(BUILD_TOOLS, "d8.bat")
ZIPALIGN = os.path.join(BUILD_TOOLS, "zipalign.exe")
APKSIGNER = os.path.join(BUILD_TOOLS, "apksigner.bat")

ZXING_JAR = os.path.abspath(r"Z:\better-new-api\worker\mobile\zxing-core.jar")
LLAMA_JAR = os.path.abspath(r"Z:\better-new-api\worker\mobile\llama.jar")
WORKDIR = os.path.abspath(r"Z:\better-new-api\worker\mobile\build_tmp")
OUTPUT_APK = os.path.abspath(r"Z:\better-new-api\worker\binaries\byog-worker.apk")

parser = argparse.ArgumentParser(description="BYOG Mobile APK Builder")
parser.add_argument("--system-name", type=str, default="", help="Custom system branding name (e.g. MyBrand)")
parser.add_argument("--favicon", type=str, default="", help="Favicon URL or local file path")
parser.add_argument("--logo", type=str, default="", help="Logo URL or local file path")
parser.add_argument("--server-url", type=str, default="", help="Default embedded server address")
parser.add_argument("--sql-dsn", type=str, default="", help="Database DSN (SQLite, MySQL, PostgreSQL) to query options table")
args, _ = parser.parse_known_args()

repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))

system_name = args.system_name or os.environ.get("SYSTEM_NAME", "")
favicon_url = args.favicon or os.environ.get("FAVICON_URL", "") or os.environ.get("FAVICON", "")
logo_url = getattr(args, "logo", "") or os.environ.get("LOGO_URL", "") or os.environ.get("LOGO", "")
server_url = args.server_url or os.environ.get("SERVER_URL", "") or os.environ.get("SERVER_ADDRESS", "")
sql_dsn = args.sql_dsn or os.environ.get("SQL_DSN", "")
port = os.environ.get("PORT", "3000")

# 1. Parse .env file if available
for env_path in [".env", "../.env", "../../.env", os.path.join(repo_root, ".env")]:
    if os.path.exists(env_path):
        try:
            with open(env_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line or line.startswith("#") or "=" not in line:
                        continue
                    k, v = line.split("=", 1)
                    k = k.strip()
                    v = v.strip().strip('"').strip("'")
                    if k == "SQL_DSN" and not sql_dsn:
                        sql_dsn = v
                    elif k == "SYSTEM_NAME" and not system_name:
                        system_name = v
                    elif (k == "FAVICON_URL" or k == "FAVICON") and not favicon_url:
                        favicon_url = v
                    elif (k == "LOGO_URL" or k == "LOGO") and not logo_url:
                        logo_url = v
                    elif (k == "SERVER_URL" or k == "SERVER_ADDRESS") and not server_url:
                        server_url = v
                    elif k == "PORT" and port == "3000":
                        port = v
            break
        except Exception:
            pass

# 2. Try querying running local server if parameters are still missing
detected_live_port = None
if not system_name or not favicon_url or not server_url:
    candidate_ports = [port, "35792", "3000", "8000"]
    for log_p in ["stdout.log", "S:/test/stdout.log", "S:\\test\\stdout.log", os.path.join(repo_root, "stdout.log")]:
        if os.path.exists(log_p):
            try:
                with open(log_p, "r", encoding="utf-8", errors="ignore") as lf:
                    for l in lf:
                        if "http://localhost:" in l:
                            import re
                            m = re.search(r"http://localhost:(\d+)", l)
                            if m and m.group(1) not in candidate_ports:
                                candidate_ports.insert(0, m.group(1))
            except Exception:
                pass
    try:
        ns = subprocess.check_output("netstat -ano", shell=True, stderr=subprocess.DEVNULL).decode("utf-8")
        for line in ns.splitlines():
            if "LISTENING" in line and ("127.0.0.1:" in line or "[::]:" in line or "0.0.0.0:" in line):
                import re
                m = re.search(r":(\d+)\s+.*LISTENING", line)
                if m:
                    p_num = m.group(1)
                    if int(p_num) > 1024 and p_num not in candidate_ports:
                        candidate_ports.append(p_num)
    except Exception:
        pass

    for check_port in candidate_ports:
        try:
            req = urllib.request.Request(f"http://127.0.0.1:{check_port}/api/status", headers={"User-Agent": "BYOG-Builder"})
            with urllib.request.urlopen(req, timeout=1.5) as resp:
                if resp.status == 200:
                    status_json = json.loads(resp.read().decode("utf-8")).get("data", {})
                    if not system_name and status_json.get("system_name"):
                        system_name = status_json.get("system_name")
                    if not favicon_url and status_json.get("favicon"):
                        favicon_url = status_json.get("favicon")
                    if not logo_url and status_json.get("logo"):
                        logo_url = status_json.get("logo")
                    if not server_url and status_json.get("server_address"):
                        server_url = status_json.get("server_address")
                    detected_live_port = check_port
                    print(f"Auto-detected options from live server (Port {check_port}): SystemName='{system_name}'")
                    break
        except Exception:
            pass

# 3. Query Database directly based on SQL_DSN across all dialects (SQLite, MySQL, PostgreSQL)
if not system_name or not favicon_url or not server_url:
    # A) SQLite
    is_sqlite = (not sql_dsn) or sql_dsn.endswith(".db") or "sqlite" in sql_dsn.lower()
    if is_sqlite:
        try:
            import sqlite3
            for db_file in [
                "one-api.db",
                "data/one-api.db",
                "S:/test/one-api.db",
                "S:\\test\\one-api.db",
                os.path.join(repo_root, "one-api.db"),
                os.path.join(repo_root, "data", "one-api.db"),
                sql_dsn if (sql_dsn and os.path.exists(sql_dsn)) else ""
            ]:
                if db_file and os.path.exists(db_file):
                    conn = sqlite3.connect(db_file)
                    with conn:
                        cur = conn.cursor()
                        cur.execute("SELECT key, value FROM options WHERE key IN ('SystemName', 'SystemName_tr', 'SystemName_en', 'Favicon', 'Logo', 'LogoDark', 'ServerAddress')")
                        for k, val in cur.fetchall():
                            if k in ('SystemName', 'SystemName_tr', 'SystemName_en') and not system_name and val and val.strip() != 'New API':
                                system_name = val.strip()
                            elif k == 'Favicon' and not favicon_url and val:
                                favicon_url = val.strip()
                            elif k in ('Logo', 'LogoDark') and not logo_url and val:
                                logo_url = val.strip()
                            elif k == 'ServerAddress' and not server_url and val:
                                server_url = val.strip()
                    conn.close()
                    if system_name or favicon_url or server_url:
                        print(f"Auto-detected options from SQLite ({db_file})")
                        break
        except Exception as ex:
            pass

    # B) MySQL
    if (not system_name or not favicon_url or not server_url) and sql_dsn and ("tcp(" in sql_dsn or "mysql" in sql_dsn.lower() or "3306" in sql_dsn):
        try:
            import re
            m = re.search(r"^(?:mysql://)?(?P<user>[^:]+)(?::(?P<pass>[^@]*))?@(?:tcp\((?P<host>[^:]+):(?P<port>\d+)\)|(?P<host2>[^:/]+)(?::(?P<port2>\d+))?)/(?P<db>[^?]+)", sql_dsn)
            if m:
                d = m.groupdict()
                host = d.get("host") or d.get("host2") or "localhost"
                port_num = int(d.get("port") or d.get("port2") or 3306)
                user = d.get("user") or "root"
                passwd = d.get("pass") or ""
                dbname = d.get("db")
                rows = []
                try:
                    import pymysql
                    conn = pymysql.connect(host=host, port=port_num, user=user, password=passwd, database=dbname, connect_timeout=4)
                    with conn.cursor() as cur:
                        cur.execute("SELECT `key`, `value` FROM `options` WHERE `key` IN ('SystemName', 'SystemName_tr', 'SystemName_en', 'Favicon', 'Logo', 'LogoDark', 'ServerAddress')")
                        rows = cur.fetchall()
                    conn.close()
                except Exception:
                    try:
                        import mysql.connector
                        conn = mysql.connector.connect(host=host, port=port_num, user=user, password=passwd, database=dbname, connection_timeout=4)
                        cur = conn.cursor()
                        cur.execute("SELECT `key`, `value` FROM `options` WHERE `key` IN ('SystemName', 'SystemName_tr', 'SystemName_en', 'Favicon', 'Logo', 'LogoDark', 'ServerAddress')")
                        rows = cur.fetchall()
                        conn.close()
                    except Exception:
                        try:
                            cmd = ["mysql", f"-h{host}", f"-P{port_num}", f"-u{user}", f"-D{dbname}", "-N", "-e", "SELECT `key`, `value` FROM `options` WHERE `key` IN ('SystemName', 'SystemName_tr', 'SystemName_en', 'Favicon', 'Logo', 'LogoDark', 'ServerAddress')"]
                            if passwd:
                                cmd.insert(4, f"-p{passwd}")
                            out = subprocess.check_output(cmd, timeout=4, stderr=subprocess.DEVNULL).decode("utf-8")
                            for line in out.splitlines():
                                parts = line.split("\t")
                                if len(parts) >= 2:
                                    rows.append((parts[0], parts[1]))
                        except Exception:
                            pass
                for k, val in rows:
                    if k in ('SystemName', 'SystemName_tr', 'SystemName_en') and not system_name and val and val.strip() != 'New API':
                        system_name = val.strip()
                    elif k == 'Favicon' and not favicon_url and val:
                        favicon_url = val.strip()
                    elif k in ('Logo', 'LogoDark') and not logo_url and val:
                        logo_url = val.strip()
                    elif k == 'ServerAddress' and not server_url and val:
                        server_url = val.strip()
                if rows:
                    print("Auto-detected options from MySQL database")
        except Exception as ex:
            print("MySQL detection notice:", ex)

    # C) PostgreSQL
    if (not system_name or not favicon_url or not server_url) and sql_dsn and ("postgres" in sql_dsn.lower() or "5432" in sql_dsn):
        try:
            import urllib.parse
            rows = []
            if sql_dsn.startswith("postgres://") or sql_dsn.startswith("postgresql://"):
                u = urllib.parse.urlparse(sql_dsn)
                host = u.hostname or "localhost"
                port_num = u.port or 5432
                user = u.username or "postgres"
                passwd = u.password or ""
                dbname = u.path.lstrip("/")
            else:
                import re
                host = (re.search(r"host=([^\s]+)", sql_dsn) or [None, "localhost"])[1]
                port_num = int((re.search(r"port=(\d+)", sql_dsn) or [None, "5432"])[1])
                user = (re.search(r"user=([^\s]+)", sql_dsn) or [None, "postgres"])[1]
                passwd = (re.search(r"password=([^\s]+)", sql_dsn) or [None, ""])[1]
                dbname = (re.search(r"dbname=([^\s]+)", sql_dsn) or [None, ""])[1]
            try:
                import psycopg2
                conn = psycopg2.connect(host=host, port=port_num, user=user, password=passwd, dbname=dbname, connect_timeout=4)
                cur = conn.cursor()
                cur.execute('SELECT "key", "value" FROM "options" WHERE "key" IN (\'SystemName\', \'SystemName_tr\', \'SystemName_en\', \'Favicon\', \'Logo\', \'LogoDark\', \'ServerAddress\')')
                rows = cur.fetchall()
                conn.close()
            except Exception:
                try:
                    import psycopg
                    with psycopg.connect(f"host={host} port={port_num} user={user} password={passwd} dbname={dbname}", connect_timeout=4) as conn:
                        with conn.cursor() as cur:
                            cur.execute('SELECT "key", "value" FROM "options" WHERE "key" IN (\'SystemName\', \'SystemName_tr\', \'SystemName_en\', \'Favicon\', \'Logo\', \'LogoDark\', \'ServerAddress\')')
                            rows = cur.fetchall()
                except Exception:
                    try:
                        out = subprocess.check_output(["psql", f"postgresql://{user}:{passwd}@{host}:{port_num}/{dbname}", "-t", "-A", "-F", "\t", "-c", 'SELECT "key", "value" FROM "options" WHERE "key" IN (\'SystemName\', \'SystemName_tr\', \'SystemName_en\', \'Favicon\', \'Logo\', \'LogoDark\', \'ServerAddress\')'], timeout=4, stderr=subprocess.DEVNULL).decode("utf-8")
                        for line in out.splitlines():
                            parts = line.split("\t")
                            if len(parts) >= 2:
                                rows.append((parts[0], parts[1]))
                    except Exception:
                        pass
            for k, val in rows:
                if k in ('SystemName', 'SystemName_tr', 'SystemName_en') and not system_name and val and val.strip() != 'New API':
                    system_name = val.strip()
                elif k == 'Favicon' and not favicon_url and val:
                    favicon_url = val.strip()
                elif k in ('Logo', 'LogoDark') and not logo_url and val:
                    logo_url = val.strip()
                elif k == 'ServerAddress' and not server_url and val:
                    server_url = val.strip()
            if rows:
                print("Auto-detected options from PostgreSQL database")
        except Exception as ex:
            print("PostgreSQL detection notice:", ex)

if not system_name or system_name.strip() in ("", "New API"):
    system_name = "BYOG"
    app_name = "BYOG Worker"
else:
    system_name = system_name.strip()
    app_name = f"{system_name} BYOG"
print(f"Building APK with Branding Name: '{app_name}'")

if os.path.exists(WORKDIR):
    shutil.rmtree(WORKDIR, ignore_errors=True)
os.makedirs(WORKDIR, exist_ok=True)

# 1. Prepare directory structure
res_dir = os.path.join(WORKDIR, "res")
res_values = os.path.join(res_dir, "values")
res_mipmap = os.path.join(res_dir, "mipmap")
assets_dir = os.path.join(WORKDIR, "assets")
assets_www = os.path.join(assets_dir, "www")
src_dir = os.path.join(WORKDIR, "src", "org", "byog", "worker")

os.makedirs(res_values, exist_ok=True)
os.makedirs(res_mipmap, exist_ok=True)
os.makedirs(assets_www, exist_ok=True)
os.makedirs(src_dir, exist_ok=True)

# strings.xml with dynamic system app name
with open(os.path.join(res_values, "strings.xml"), "w", encoding="utf-8") as f:
    f.write(f'''<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">{app_name}</string>
</resources>
''')

# styles.xml (Dark theme matching website palette #020817)
with open(os.path.join(res_values, "styles.xml"), "w", encoding="utf-8") as f:
    f.write('''<?xml version="1.0" encoding="utf-8"?>
<resources>
    <style name="AppTheme" parent="@android:style/Theme.NoTitleBar">
        <item name="android:windowBackground">#020817</item>
    </style>
</resources>
''')

# Download or extract icon for Android Launcher Icon
def load_icon_bytes(fav_url, lg_url, srv_url):
    candidates = []
    if fav_url:
        candidates.append(fav_url)
    if lg_url:
        candidates.append(lg_url)
    
    for item in candidates:
        if not item:
            continue
        # Base64 data URI
        if item.startswith("data:image/") and ";base64," in item:
            try:
                import base64
                b64_data = item.split(";base64,", 1)[1]
                return base64.b64decode(b64_data)
            except Exception:
                pass
        # HTTP/HTTPS URL
        if item.startswith("http://") or item.startswith("https://"):
            try:
                req = urllib.request.Request(item, headers={"User-Agent": "BYOG-Builder/1.0"})
                with urllib.request.urlopen(req, timeout=8) as resp:
                    if resp.status == 200:
                        d = resp.read()
                        if len(d) > 0:
                            print(f"Downloaded icon from URL: {item} ({len(d)} bytes)")
                            return d
            except Exception as e:
                print(f"Failed to download icon from {item}:", e)
        # Local filesystem path or relative URL
        clean = item.lstrip("/\\")
        for p in [
            item,
            clean,
            os.path.join(repo_root, clean),
            os.path.join(repo_root, "web", "public", clean),
            os.path.join(repo_root, "web", "build", clean),
            os.path.join(repo_root, "uploads", clean),
            os.path.join(repo_root, "uploads", "ads", clean),
            os.path.join(repo_root, "data", clean),
        ]:
            if p and os.path.exists(p) and os.path.isfile(p):
                try:
                    with open(p, "rb") as f:
                        d = f.read()
                        if len(d) > 0:
                            print(f"Loaded icon from local file: {p} ({len(d)} bytes)")
                            return d
                except Exception:
                    pass
        # Check database managed_files table for uploaded custom icon
        for db_f in ["one-api.db", "S:/test/one-api.db", "S:\\test\\one-api.db", os.path.join(repo_root, "one-api.db"), os.path.join(repo_root, "data", "one-api.db")]:
            if os.path.exists(db_f):
                try:
                    import sqlite3
                    conn = sqlite3.connect(db_f)
                    with conn:
                        cur = conn.cursor()
                        clean_search = f"%{clean}%"
                        cur.execute("SELECT content FROM managed_files WHERE is_dir = 0 AND (path LIKE ? OR name LIKE ?) ORDER BY id DESC LIMIT 1", (clean_search, clean_search))
                        r = cur.fetchone()
                        if not r or not r[0]:
                            cur.execute("SELECT content FROM managed_files WHERE is_dir = 0 AND (path LIKE '%favicon%' OR path LIKE '%logo%' OR name LIKE '%favicon%' OR name LIKE '%logo%') ORDER BY id DESC LIMIT 1")
                            r = cur.fetchone()
                        if r and r[0] and len(r[0]) > 0:
                            print(f"Loaded icon from database managed_files ({db_f}): {len(r[0])} bytes")
                            return r[0]
                    conn.close()
                except Exception:
                    pass

        # If relative path and server_url or detected_live_port available, try fetching from server
        candidate_urls = []
        if item.startswith("/") and srv_url:
            candidate_urls.append(f"{srv_url.rstrip('/')}{item}")
        if item.startswith("/") and detected_live_port:
            candidate_urls.append(f"http://127.0.0.1:{detected_live_port}{item}")

        for full_url in candidate_urls:
            try:
                req = urllib.request.Request(full_url, headers={"User-Agent": "BYOG-Builder/1.0"})
                with urllib.request.urlopen(req, timeout=5) as resp:
                    if resp.status == 200:
                        d = resp.read()
                        if len(d) > 0:
                            print(f"Downloaded relative icon from server: {full_url}")
                            return d
            except Exception:
                pass

    # Fallback to default icons in web/public
    for fallback in [
        os.path.join(repo_root, "web", "public", "logo.png"),
        os.path.join(repo_root, "web", "public", "favicon.ico"),
    ]:
        if os.path.exists(fallback):
            with open(fallback, "rb") as f:
                return f.read()
    return None

icon_bytes = load_icon_bytes(favicon_url, logo_url, server_url)

from PIL import Image

try:
    if icon_bytes:
        im = Image.open(io.BytesIO(icon_bytes))
    else:
        icon_source = os.path.join(repo_root, "web", "public", "logo.png")
        if not os.path.exists(icon_source):
            icon_source = os.path.join(repo_root, "web", "public", "favicon.ico")
        im = Image.open(icon_source)

    if getattr(im, "ico", None):
        sizes = im.ico.sizes()
        largest_size = max(sizes, key=lambda s: s[0] * s[1])
        img = im.ico.getimage(largest_size).convert("RGBA")
    else:
        img = im.convert("RGBA")

    img = img.resize((192, 192), Image.Resampling.LANCZOS)
    img.save(os.path.join(res_mipmap, "ic_launcher.png"), "PNG")
    img.save(os.path.join(res_mipmap, "ic_launcher_round.png"), "PNG")
    img.save(os.path.join(assets_www, "logo.png"), "PNG")
    print("Generated high-res 192x192 icons successfully.")
except Exception as e:
    print("Fallback icon copy due to:", e)
    logo_src = os.path.join(repo_root, "web", "public", "logo.png")
    if os.path.exists(logo_src):
        shutil.copyfile(logo_src, os.path.join(res_mipmap, "ic_launcher.png"))
        shutil.copyfile(logo_src, os.path.join(res_mipmap, "ic_launcher_round.png"))
        shutil.copyfile(logo_src, os.path.join(assets_www, "logo.png"))

# AndroidManifest.xml
manifest_content = f'''<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="org.byog.worker"
    android:versionCode="6"
    android:versionName="1.8.0">

    <uses-sdk android:minSdkVersion="24" android:targetSdkVersion="34" />

    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
    <uses-permission android:name="android.permission.WAKE_LOCK" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
    <uses-permission android:name="android.permission.CAMERA" />
    <uses-permission android:name="android.permission.VIBRATE" />
    <uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED" />
    <uses-permission android:name="android.permission.QUICKBOOT_POWERON" />
    <uses-permission android:name="android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS" />
    <uses-permission android:name="android.permission.REQUEST_INSTALL_PACKAGES" />

    <uses-feature android:name="android.hardware.camera" android:required="false" />
    <uses-feature android:name="android.hardware.camera.autofocus" android:required="false" />

    <queries>
        <package android:name="com.coloros.safecenter" />
        <package android:name="com.oplus.safecenter" />
        <package android:name="com.oplus.securitypermission" />
        <package android:name="com.coloros.securitypermission" />
        <package android:name="com.coloros.oppoguardelf" />
        <package android:name="com.realme.safecenter" />
        <package android:name="com.realme.securitypermission" />
        <package android:name="com.hihonor.systemmanager" />
        <package android:name="com.huawei.systemmanager" />
        <package android:name="com.miui.securitycenter" />
        <package android:name="com.vivo.permissionmanager" />
        <package android:name="com.iqoo.secure" />
        <package android:name="com.samsung.android.lool" />
        <package android:name="com.samsung.android.sm" />
        <package android:name="com.transsion.phonemaster" />
    </queries>

    <application
        android:label="@string/app_name"
        android:icon="@mipmap/ic_launcher"
        android:roundIcon="@mipmap/ic_launcher_round"
        android:theme="@style/AppTheme"
        android:usesCleartextTraffic="true"
        android:largeHeap="true"
        android:hardwareAccelerated="true">
        <provider
            android:name=".GenericFileProvider"
            android:authorities="org.byog.worker.fileprovider"
            android:exported="false"
            android:grantUriPermissions="true" />
        <activity
            android:name=".MainActivity"
            android:label="@string/app_name"
            android:icon="@mipmap/ic_launcher"
            android:roundIcon="@mipmap/ic_launcher_round"
            android:exported="true"
            android:configChanges="orientation|screenSize|keyboardHidden">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
        <activity
            android:name=".QRScanActivity"
            android:exported="false"
            android:screenOrientation="portrait" />
        <service
            android:name=".WorkerService"
            android:exported="false"
            android:stopWithTask="false"
            android:process=":worker"
            android:foregroundServiceType="dataSync" />
        <receiver
            android:name=".BootReceiver"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.BOOT_COMPLETED" />
                <action android:name="android.intent.action.QUICKBOOT_POWERON" />
                <action android:name="android.intent.action.MY_PACKAGE_REPLACED" />
                <action android:name="android.intent.action.USER_PRESENT" />
                <action android:name="android.intent.action.ACTION_POWER_CONNECTED" />
                <action android:name="android.net.conn.CONNECTIVITY_CHANGE" />
                <action android:name="org.byog.worker.WATCHDOG_CHECK" />
            </intent-filter>
        </receiver>
    </application>
</manifest>
'''
manifest_path = os.path.join(WORKDIR, "AndroidManifest.xml")
with open(manifest_path, "w", encoding="utf-8") as f:
    f.write(manifest_content)

# 2. Modern HTML5/CSS/JS UI in assets/www/index.html (Sequential 3-step permissions onboarding checklist)
html_content = f'''<!DOCTYPE html>
<html lang="tr">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
  <title>{app_name}</title>
  <style>
    * {{ box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Plus Jakarta Sans", "Inter", "Segoe UI", Roboto, system-ui, sans-serif; user-select: none; -webkit-font-smoothing: antialiased; }}
    body {{ background-color: #020817; color: #f8fafc; padding: 18px; min-height: 100vh; display: flex; flex-direction: column; }}
    
    .header {{ display: flex; align-items: center; justify-content: space-between; padding-bottom: 16px; border-bottom: 1px solid #1e293b; margin-bottom: 18px; }}
    .brand {{ display: flex; align-items: center; gap: 10px; }}
    .brand img {{ width: 34px; height: 34px; border-radius: 8px; object-fit: contain; }}
    .brand-title {{ font-size: 1.12rem; font-weight: 700; letter-spacing: -0.02em; color: #f8fafc; }}
    .badge {{ font-size: 0.76rem; font-weight: 600; padding: 4px 10px; border-radius: 9999px; display: inline-flex; align-items: center; gap: 6px; }}
    .badge-amber {{ background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }}
    .badge-green {{ background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }}
    .dot {{ width: 6px; height: 6px; border-radius: 50%; }}
    .dot-amber {{ background-color: #fbbf24; }}
    .dot-green {{ background-color: #34d399; box-shadow: 0 0 8px #34d399; }}

    .card {{ background-color: #0f172a; border: 1px solid #1e293b; border-radius: 14px; padding: 18px; margin-bottom: 14px; box-shadow: 0 4px 16px rgba(0,0,0,0.3); }}
    .card-title {{ font-size: 0.85rem; font-weight: 600; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 12px; display: flex; align-items: center; gap: 7px; }}

    .btn {{ display: flex; align-items: center; justify-content: center; gap: 8px; width: 100%; padding: 13px 18px; border-radius: 10px; font-size: 0.95rem; font-weight: 600; cursor: pointer; border: none; transition: all 0.15s ease; }}
    .btn:disabled {{ opacity: 0.5; cursor: not-allowed; }}
    .btn-primary {{ background-color: #2563eb; color: #ffffff; }}
    .btn-primary:active:not(:disabled) {{ background-color: #1d4ed8; transform: scale(0.99); }}
    .btn-secondary {{ background-color: #1e293b; color: #e2e8f0; border: 1px solid #334155; }}
    .btn-secondary:active:not(:disabled) {{ background-color: #334155; transform: scale(0.99); }}
    .btn-danger {{ background-color: rgba(239, 68, 68, 0.12); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }}
    .btn-danger:active:not(:disabled) {{ background-color: rgba(239, 68, 68, 0.22); }}

    .perm-card {{ display: flex; align-items: center; justify-content: space-between; background-color: #020817; border: 1px solid #1e293b; border-radius: 10px; padding: 13px 14px; margin-bottom: 10px; cursor: pointer; transition: all 0.2s ease; }}
    .perm-card:active {{ background-color: #1e293b; }}
    .perm-card.granted {{ border-color: rgba(16, 185, 129, 0.4); background-color: rgba(16, 185, 129, 0.05); }}
    .perm-info {{ display: flex; flex-direction: column; gap: 3px; }}
    .perm-name {{ font-size: 0.9rem; font-weight: 600; color: #f8fafc; }}
    .perm-desc {{ font-size: 0.74rem; color: #94a3b8; }}
    .perm-status-badge {{ font-size: 0.75rem; font-weight: 700; padding: 4px 10px; border-radius: 6px; background-color: #1e293b; color: #94a3b8; white-space: nowrap; }}
    .perm-status-badge.granted {{ background-color: rgba(16, 185, 129, 0.2); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.4); }}

    .input-group {{ display: flex; gap: 8px; margin-top: 10px; margin-bottom: 12px; }}
    .input-text {{ flex: 1; background-color: #020817; border: 1px solid #334155; border-radius: 8px; padding: 12px 14px; font-family: monospace; font-size: 0.95rem; font-weight: 700; color: #f8fafc; text-align: center; letter-spacing: 0.08em; text-transform: uppercase; }}
    .input-text:focus {{ outline: none; border-color: #3b82f6; }}
    .btn-icon {{ width: 46px; display: flex; align-items: center; justify-content: center; background-color: #1e293b; border: 1px solid #334155; border-radius: 8px; color: #94a3b8; cursor: pointer; }}
    .btn-icon:active {{ background-color: #334155; }}

    .hint-text {{ font-size: 0.8rem; color: #94a3b8; line-height: 1.45; }}

    .grid-2 {{ display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 8px; }}
    .stat-card {{ background-color: #020817; border: 1px solid #1e293b; border-radius: 10px; padding: 12px; }}
    .stat-label {{ font-size: 0.72rem; color: #64748b; font-weight: 600; text-transform: uppercase; }}
    .stat-value {{ font-size: 1.05rem; font-weight: 700; color: #f8fafc; margin-top: 4px; }}
    .stat-sub {{ font-size: 0.72rem; color: #94a3b8; margin-top: 2px; }}

    .divider {{ height: 1px; background-color: #1e293b; margin: 16px 0; }}
    .hidden {{ display: none !important; }}
    
    .toast {{ position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%); background-color: #1e293b; border: 1px solid #334155; color: #f8fafc; padding: 10px 18px; border-radius: 8px; font-size: 0.85rem; font-weight: 500; z-index: 3000; opacity: 0; transition: opacity 0.2s ease; pointer-events: none; white-space: nowrap; max-width: 90%; text-overflow: ellipsis; overflow: hidden; }}
    .toast.show {{ opacity: 1; }}

    .modal {{ position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.8); display: flex; align-items: center; justify-content: center; z-index: 2000; padding: 16px; backdrop-filter: blur(4px); }}
    .modal-content {{ background: #0f172a; border: 1px solid #1e293b; border-radius: 14px; width: 100%; max-width: 380px; max-height: 85vh; display: flex; flex-direction: column; padding: 16px; box-shadow: 0 10px 30px rgba(0,0,0,0.6); }}
    .modal-header {{ display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #1e293b; padding-bottom: 10px; margin-bottom: 12px; }}
    .btn-close {{ background: none; border: none; color: #94a3b8; font-size: 1.2rem; cursor: pointer; padding: 4px; }}
    .chat-messages {{ flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; max-height: 280px; min-height: 160px; padding: 4px; }}
    .chat-bubble {{ padding: 10px 13px; border-radius: 10px; font-size: 0.82rem; line-height: 1.45; max-width: 88%; word-break: break-word; }}
    .chat-assistant {{ background: #1e293b; color: #f8fafc; align-self: flex-start; border-bottom-left-radius: 2px; }}
    .chat-user {{ background: #2563eb; color: #ffffff; align-self: flex-end; border-bottom-right-radius: 2px; }}
    .chat-input-bar {{ display: flex; gap: 6px; }}
    .chat-input-bar input {{ flex: 1; background: #020817; border: 1px solid #334155; border-radius: 8px; padding: 10px 12px; color: #f8fafc; font-size: 0.85rem; }}
    .chat-input-bar input:focus {{ outline: none; border-color: #3b82f6; }}
    .chat-input-bar button {{ background: #2563eb; color: #fff; border: none; border-radius: 8px; width: 42px; display: flex; align-items: center; justify-content: center; cursor: pointer; }}

    svg {{ width: 18px; height: 18px; stroke: currentColor; fill: none; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }}
  </style>
</head>
<body>

  <!-- App Header -->
  <div class="header">
    <div class="brand">
      <img id="app-logo" src="logo.png" onerror="this.onerror=null; this.src='logo.png';">
      <div id="app-title" class="brand-title">{app_name}</div>
    </div>
    <div id="status-badge" class="badge badge-amber">
      <div class="dot dot-amber"></div>
      <span id="status-text">Kurulum Bekleniyor</span>
    </div>
  </div>

  <!-- The notice is shown in every view, it explains a lost connection. -->
  <div id="connection-notice" class="hidden" style="margin-bottom: 14px; padding: 10px 12px; border-radius: 10px; background: rgba(245, 158, 11, 0.12); border: 1px solid rgba(245, 158, 11, 0.35); color: #fbbf24; font-size: 0.8rem; line-height: 1.45;">
    Bağlantı kesildi. Cihazınız çevrimdışı, yeniden eşleştirmek için eşleştirme kodunu girin.
  </div>

  <!-- View 0: Sequential Permissions Checklist -->
  <div id="permissions-view">
    <div class="card">
      <div class="card-title">
        <svg viewBox="0 0 24 24"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
        <span>Gerekli Sistem İzinleri</span>
      </div>
      <p class="hint-text" style="margin-bottom: 16px;">
        Uygulamanın arka planda kesintisiz çalışabilmesi, yapay zeka çıkarım görevlerini yerine getirebilmesi ve sistem bildirimlerini iletebilmesi için aşağıdaki izinlerin verilmesi gerekmektedir.
      </p>

      <div class="perm-card" id="card-perm-notif" onclick="requestNotifPerm()">
        <div class="perm-info">
          <span class="perm-name">1. Bildirim İzni</span>
          <span class="perm-desc">Çıkarım ve model durumunu takip için</span>
        </div>
        <span class="perm-status-badge" id="badge-perm-notif">İzin Ver</span>
      </div>

      <div class="perm-card" id="card-perm-battery" onclick="requestBatteryPerm()">
        <div class="perm-info">
          <span class="perm-name">2. Pil Optimizasyonunu Kapat</span>
          <span class="perm-desc">Arka planda sistem tarafından kapatılmaması için</span>
        </div>
        <span class="perm-status-badge" id="badge-perm-battery">Kapat</span>
      </div>

      <div class="perm-card" id="card-perm-bgactivity" onclick="requestBackgroundActivityPerm()">
        <div class="perm-info">
          <span class="perm-name">3. Arka Planda Çalışma İzni (ZORUNLU)</span>
          <span class="perm-desc">Pil Kullanımı -> 'Arka plan aktivitesine izin ver' açılmalıdır</span>
        </div>
        <span class="perm-status-badge" id="badge-perm-bgactivity">İzin Ver</span>
      </div>

      <div class="perm-card" id="card-perm-autostart" onclick="requestAutoStartPerm()">
        <div class="perm-info">
          <span class="perm-name">4. Otomatik Başlatma İzni (ZORUNLU)</span>
          <span class="perm-desc">Cihazın 'Otomatik Başlatma' ekranında uygulama etkinleştirilmelidir</span>
        </div>
        <span class="perm-status-badge" id="badge-perm-autostart">İzin Ver</span>
      </div>

      <button id="btn-perm-continue" class="btn btn-primary" onclick="proceedToPairing()" style="margin-top: 14px; opacity: 0.5;" disabled>
        <span>Eşleştirmeye Devam Et</span>
      </button>
    </div>
  </div>

  <!-- View 1: Unpaired (Single Flow: Scan QR or Enter Code) -->
  <div id="unpaired-view" class="hidden">
    <div class="card">
      <div class="card-title">
        <svg viewBox="0 0 24 24"><path d="M4 7V4h3M20 7V4h-3M4 17v3h3M20 17v3h-3M9 9h6v6H9z"/></svg>
        <span>Hızlı ve Güvenli Bağlantı</span>
      </div>
      
      <p class="hint-text" style="margin-bottom: 14px;">
        Web kontrol panelinizdeki "Cihaz Bağla" penceresinde gösterilen QR kodu kameranızla okutun veya eşleştirme kodunu aşağıya girin.
      </p>

      <button id="btn-scan" class="btn btn-primary" onclick="startQrScan()">
        <svg viewBox="0 0 24 24"><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg>
        <span>QR Kod Tara</span>
      </button>

      <div class="divider"></div>

      <div class="card-title" style="margin-bottom: 6px;">
        <svg viewBox="0 0 24 24"><circle cx="7.5" cy="15.5" r="5.5"/><path d="m21 2-9.6 9.6M15.5 7.5l3 3"/></svg>
        <span>Eşleştirme Kodu</span>
      </div>

      <div class="input-group">
        <input id="input-code" type="text" class="input-text" placeholder="UB55-SV5S-DZ79-MVHX" maxlength="24">
        <button class="btn-icon" onclick="pasteCode()" title="Yapıştır">
          <svg viewBox="0 0 24 24"><rect width="8" height="4" x="8" y="2" rx="1" ry="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/></svg>
        </button>
      </div>

      <button id="btn-connect" class="btn btn-secondary" onclick="connectManual()">
        <span>Cihazı Bağla</span>
      </button>
    </div>
  </div>

  <!-- View 2: Paired & Active Worker -->
  <div id="paired-view" class="hidden">
    <div class="card">
      <div class="card-title">
        <svg viewBox="0 0 24 24"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        <span>Hesaplama Düğümü Durumu</span>
      </div>

      <!-- Active Model Status Card -->
      <div class="stat-card" style="margin-bottom: 12px; border-color: rgba(59, 130, 246, 0.3);">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div class="stat-label">Aktif Model</div>
          <span id="stat-model-badge" class="badge badge-amber" style="font-size: 0.7rem; padding: 2px 8px;">Beklemede</span>
        </div>
        <div id="stat-model-name" class="stat-value" style="font-size: 0.95rem; font-family: monospace; margin-top: 4px;">qwen-2.5-0.5b-instruct</div>
        <div id="stat-model-desc" class="stat-sub">Çıkarım motoru hazır</div>

        <div id="download-progress-container" class="hidden" style="margin-top: 10px;">
          <div style="display: flex; justify-content: space-between; font-size: 0.75rem; color: #94a3b8; margin-bottom: 4px;">
            <span id="download-progress-text">0 MB / 380 MB</span>
            <span id="download-progress-pct">%0</span>
          </div>
          <div style="background: rgba(255,255,255,0.1); border-radius: 6px; height: 8px; overflow: hidden;">
            <div id="download-progress-bar" style="background: #3b82f6; height: 100%; width: 0%; transition: width 0.3s ease;"></div>
          </div>
          <div id="download-error-text" class="hidden" style="margin-top: 6px; font-size: 0.7rem; color: #f87171;"></div>
        </div>
        <div id="crash-text" class="hidden" style="margin-top: 8px; font-size: 0.7rem; color: #fb923c;"></div>

        <div id="device-details" class="hidden" style="margin-top: 10px; padding: 10px 12px; border-radius: 12px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.08);">
          <div style="font-size: 0.7rem; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.06em; margin-bottom: 6px;">Cihaz Bilgileri</div>
          <div style="display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px 10px; font-size: 0.75rem;">
            <div><span style="color: #64748b;">Bellek</span><div id="device-ram" style="color: #e2e8f0;">-</div></div>
            <div><span style="color: #64748b;">Çekirdek</span><div id="device-cores" style="color: #e2e8f0;">-</div></div>
            <div><span style="color: #64748b;">Boş Alan</span><div id="device-disk" style="color: #e2e8f0;">-</div></div>
            <div><span style="color: #64748b;">Model Dosyası</span><div id="device-model-size" style="color: #e2e8f0;">-</div></div>
          </div>
        </div>

        <button id="btn-test-model" class="btn btn-secondary" onclick="openModelChatModal()" style="margin-top: 10px; font-size: 0.85rem; padding: 9px 12px;">
          <svg viewBox="0 0 24 24"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
          <span>Modeli Test Et (Playground)</span>
        </button>
      </div>

      <div class="grid-2">
        <div class="stat-card">
          <div class="stat-label">Bağlı Sunucu</div>
          <div id="stat-server" class="stat-value" style="font-size: 0.85rem; word-break: break-all;">--</div>
          <div id="stat-node" class="stat-sub">Node: --</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">TPS Hızı</div>
          <div id="stat-tps" class="stat-value">0.0</div>
          <div class="stat-sub">Aktif çıkarım</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Pil & Sıcaklık</div>
          <div id="stat-battery" class="stat-value">100% / 31&deg;C</div>
          <div class="stat-sub">Termal koruma aktif</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Ağ Durumu</div>
          <div id="stat-net" class="stat-value">WiFi</div>
          <div class="stat-sub">Düşük gecikme</div>
        </div>
      </div>

      <div class="divider"></div>

      <p class="hint-text" style="margin-bottom: 14px;">
        Worker arka planda güvenle çalışmaktadır. Cihaz uyku moduna geçse dahi hesaplama görevleri kesintisiz devam eder.
      </p>

      <button class="btn btn-danger" onclick="disconnectNode()">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
        <span>Bağlantıyı Kes</span>
      </button>
    </div>
  </div>

  <!-- Model Chat Playground Modal -->
  <div id="model-chat-modal" class="modal hidden">
    <div class="modal-content">
      <div class="modal-header">
        <div style="display:flex; align-items:center; gap:8px;">
          <svg viewBox="0 0 24 24"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
          <span style="font-weight:700; font-size:0.92rem;" id="chat-modal-title">Model Testi</span>
        </div>
        <button class="btn-close" onclick="closeModelChatModal()">✕</button>
      </div>
      <div id="chat-messages" class="chat-messages">
        <div class="chat-bubble chat-assistant">
          Merhaba! İndirilmiş olan modeli bu ekrandan doğrudan test edebilirsiniz. Bir soru sorun veya mesaj yazın.
        </div>
      </div>
      <div class="chat-input-bar">
        <input id="chat-input" type="text" placeholder="Mesajınızı yazın..." onkeydown="if(event.key==='Enter') sendChatMessage()">
        <button id="btn-send-chat" onclick="sendChatMessage()">
          <svg viewBox="0 0 24 24"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
        </button>
      </div>
      <div id="chat-metrics" style="font-size:0.7rem; color:#64748b; font-family:monospace; margin-top:6px; text-align:center;">
        Hazır
      </div>
    </div>
  </div>

  <div id="toast" class="toast"></div>

  <!-- Update Prompt Modal -->
  <div id="update-modal" class="modal hidden">
    <div class="modal-content" style="max-width: 320px; text-align: center;">
      <div class="modal-header">
        <span style="font-weight:700; font-size:1rem;">Yeni Sürüm Hazır</span>
        <button class="btn-close" onclick="closeUpdateModal()">✕</button>
      </div>
      <p class="hint-text" style="margin-bottom: 16px;" id="update-desc">
        Yeni bir güncelleme indirildi. Kesintisiz çalışma için hemen yükleyebilirsiniz.
      </p>
      <button class="btn btn-primary" onclick="installUpdate()">
        <span>Şimdi Güncelle</span>
      </button>
    </div>
  </div>

  <script>
    let permissionsCompleted = false;

    function showToast(msg) {{
      const t = document.getElementById('toast');
      t.innerText = msg;
      t.classList.add('show');
      setTimeout(() => t.classList.remove('show'), 3000);
    }}

    function refreshPermissions() {{
      if (window.AndroidBridge && window.AndroidBridge.getPermissionStatus) {{
        try {{
          const p = JSON.parse(window.AndroidBridge.getPermissionStatus());
          const notifCard = document.getElementById('card-perm-notif');
          const notifBadge = document.getElementById('badge-perm-notif');
          if (p.notifications) {{
            notifCard.classList.add('granted');
            notifBadge.className = 'perm-status-badge granted';
            notifBadge.innerText = '✓ Verildi';
          }} else {{
            notifCard.classList.remove('granted');
            notifBadge.className = 'perm-status-badge';
            notifBadge.innerText = 'İzin Ver';
          }}

          const battCard = document.getElementById('card-perm-battery');
          const battBadge = document.getElementById('badge-perm-battery');
          if (p.battery) {{
            battCard.classList.add('granted');
            battBadge.className = 'perm-status-badge granted';
            battBadge.innerText = '✓ Kaldırıldı';
          }} else {{
            battCard.classList.remove('granted');
            battBadge.className = 'perm-status-badge';
            battBadge.innerText = 'Kapat';
          }}

          const bgCard = document.getElementById('card-perm-bgactivity');
          const bgBadge = document.getElementById('badge-perm-bgactivity');
          const hasBgActivity = p.bgactivity_supported !== false;
          const isBgActive = Boolean(p.bgactivity !== false);
          if (!hasBgActivity) {{
            bgCard.classList.add('hidden');
          }} else if (isBgActive) {{
            bgCard.classList.remove('hidden');
            bgCard.classList.add('granted');
            bgBadge.className = 'perm-status-badge granted';
            bgBadge.innerText = '✓ Verildi';
          }} else {{
            bgCard.classList.remove('hidden');
            bgCard.classList.remove('granted');
            bgBadge.className = 'perm-status-badge';
            bgBadge.innerText = 'İzin Ver';
          }}

          const autoCard = document.getElementById('card-perm-autostart');
          const autoBadge = document.getElementById('badge-perm-autostart');
          const hasAutoStart = p.autostart_supported !== false;
          const isAutoStart = Boolean(p.autostart);
          if (!hasAutoStart) {{
            autoCard.classList.add('hidden');
          }} else if (isAutoStart) {{
            autoCard.classList.remove('hidden');
            autoCard.classList.add('granted');
            autoBadge.className = 'perm-status-badge granted';
            autoBadge.innerText = '✓ Verildi';
          }} else {{
            autoCard.classList.remove('hidden');
            autoCard.classList.remove('granted');
            autoBadge.className = 'perm-status-badge';
            autoBadge.innerText = 'İzin Ver';
          }}

          permissionsCompleted = Boolean(p.notifications && p.battery);
          const btnCont = document.getElementById('btn-perm-continue');
          if (btnCont) {{
            if (permissionsCompleted) {{
              btnCont.disabled = false;
              btnCont.style.opacity = '1';
            }} else {{
              btnCont.disabled = true;
              btnCont.style.opacity = '0.5';
            }}
          }}

          if (!p.notifications || !p.battery) {{
            document.getElementById('unpaired-view').classList.add('hidden');
            document.getElementById('paired-view').classList.add('hidden');
            document.getElementById('permissions-view').classList.remove('hidden');
          }}
        }} catch(e) {{}}
      }}
    }}
    window.refreshPermissions = refreshPermissions;

    function requestNotifPerm() {{
      if (window.AndroidBridge && window.AndroidBridge.requestNotificationPermission) {{
        window.AndroidBridge.requestNotificationPermission();
      }}
      setTimeout(refreshPermissions, 800);
    }}

    function requestBatteryPerm() {{
      if (window.AndroidBridge && window.AndroidBridge.requestBatteryOptimization) {{
        window.AndroidBridge.requestBatteryOptimization();
      }}
      setTimeout(refreshPermissions, 1000);
    }}

    function requestBackgroundActivityPerm() {{
      if (window.AndroidBridge && window.AndroidBridge.openBackgroundActivitySettings) {{
        window.AndroidBridge.openBackgroundActivitySettings();
      }}
      setTimeout(refreshPermissions, 1500);
    }}

    function requestAutoStartPerm() {{
      if (window.AndroidBridge && window.AndroidBridge.openAutoStartSettings) {{
        window.AndroidBridge.openAutoStartSettings();
      }}
      setTimeout(refreshPermissions, 1500);
    }}

    function openModelChatModal() {{
      const modal = document.getElementById('model-chat-modal');
      const modelName = document.getElementById('stat-model-name').innerText;
      document.getElementById('chat-modal-title').innerText = "Test: " + modelName;
      modal.classList.remove('hidden');
    }}

    function closeModelChatModal() {{
      document.getElementById('model-chat-modal').classList.add('hidden');
    }}

    function sendChatMessage() {{
      const inp = document.getElementById('chat-input');
      const text = inp.value.trim();
      if (!text) return;
      inp.value = '';
      
      const msgs = document.getElementById('chat-messages');
      const uBubble = document.createElement('div');
      uBubble.className = 'chat-bubble chat-user';
      uBubble.innerText = text;
      msgs.appendChild(uBubble);

      const aBubble = document.createElement('div');
      aBubble.className = 'chat-bubble chat-assistant';
      aBubble.id = 'latest-ai-reply';
      aBubble.innerText = 'Çıkarım yapılıyor...';
      msgs.appendChild(aBubble);
      msgs.scrollTop = msgs.scrollHeight;

      document.getElementById('chat-metrics').innerText = 'Model çalıştırılıyor...';

      if (window.AndroidBridge && window.AndroidBridge.testModelInference) {{
        window.AndroidBridge.testModelInference(text);
      }}
    }}

    window.onModelTestResult = function(reply, latencyMs, tps) {{
      const aBubble = document.getElementById('latest-ai-reply');
      if (aBubble) {{
        aBubble.innerText = reply;
        aBubble.removeAttribute('id');
      }}
      const msgs = document.getElementById('chat-messages');
      if (msgs) msgs.scrollTop = msgs.scrollHeight;
      const metrics = document.getElementById('chat-metrics');
      if (metrics) {{
        if (latencyMs > 0) {{
          metrics.innerText = "Yanıt Süresi: " + latencyMs + "ms • Hız: ~" + tps + " TPS (UMA)";
        }} else {{
          metrics.innerText = "Hazır";
        }}
      }}
    }};

    function proceedToPairing() {{
      if (!permissionsCompleted) {{
        showToast("Lütfen önce tüm izinleri (Bildirim, Pil ve Otomatik Başlatma) tamamlayınız.");
        return;
      }}
      document.getElementById('permissions-view').classList.add('hidden');
      document.getElementById('unpaired-view').classList.remove('hidden');
    }}

    function setConnectingState(isConnecting) {{
      const btn = document.getElementById('btn-connect');
      const scanBtn = document.getElementById('btn-scan');
      if (isConnecting) {{
        btn.disabled = true;
        btn.innerHTML = '<span>Bağlantı Kuruluyor...</span>';
        scanBtn.disabled = true;
      }} else {{
        btn.disabled = false;
        btn.innerHTML = '<span>Cihazı Bağla</span>';
        scanBtn.disabled = false;
      }}
    }}
    window.setConnectingState = setConnectingState;

    function startQrScan() {{
      if (window.AndroidBridge && window.AndroidBridge.scanQrCode) {{
        window.AndroidBridge.scanQrCode();
      }} else {{
        showToast("Kamera başlatılamadı");
      }}
    }}

    function pasteCode() {{
      if (window.AndroidBridge && window.AndroidBridge.pasteFromClipboard) {{
        const txt = window.AndroidBridge.pasteFromClipboard();
        if (txt) {{
          document.getElementById('input-code').value = txt.trim().toUpperCase();
          showToast("Kod yapıştırıldı");
        }}
      }}
    }}

    function connectManual() {{
      const code = document.getElementById('input-code').value.trim().toUpperCase();
      if (!code) {{
        showToast("Lütfen eşleştirme kodunu girin");
        return;
      }}
      if (window.AndroidBridge && window.AndroidBridge.connect) {{
        window.AndroidBridge.connect(code, "");
      }}
    }}

    function disconnectNode() {{
      if (window.AndroidBridge && window.AndroidBridge.disconnect) {{
        window.AndroidBridge.disconnect();
      }}
    }}

    window.updateLogo = function(base64Uri) {{
      if (!base64Uri) return;
      const img = document.getElementById('app-logo');
      if (img) img.src = base64Uri;
    }};

    window.handleScannedQr = function(rawText) {{
      if (!rawText) return;
      try {{
        const obj = JSON.parse(rawText);
        const serverUrl = obj.server_url || "";
        const c = obj.pairing_code || obj.code;
        const qrAppName = obj.app_name || "";
        const qrFavicon = obj.favicon || "";
        if (c) {{
          document.getElementById('input-code').value = c;
          if (window.AndroidBridge && window.AndroidBridge.connectWithMeta) {{
            window.AndroidBridge.connectWithMeta(c, serverUrl, qrAppName, qrFavicon);
          }} else if (window.AndroidBridge && window.AndroidBridge.connect) {{
            window.AndroidBridge.connect(c, serverUrl);
          }}
          return;
        }}
      }} catch (e) {{
        document.getElementById('input-code').value = rawText.trim().toUpperCase();
        connectManual();
      }}
    }};

    window.updateState = function(stateJson) {{
      try {{
        const s = typeof stateJson === 'string' ? JSON.parse(stateJson) : stateJson;
        if (s.app_name) {{
          document.getElementById('app-title').innerText = s.app_name;
          document.title = s.app_name;
        }}
        if (s.favicon_base64 && s.favicon_base64.length > 0) {{
          document.getElementById('app-logo').src = s.favicon_base64;
        }} else if (s.favicon && s.favicon.length > 0) {{
          let fav = s.favicon;
          if (fav.startsWith('/') && s.server_url) {{
            fav = s.server_url.replace(/\\/+$/, '') + fav;
          }}
          const img = document.getElementById('app-logo');
          img.onerror = function() {{
            this.onerror = null;
            this.src = 'logo.png';
          }};
          img.src = fav;
        }}
        if (s.is_paired) {{
          document.getElementById('permissions-view').classList.add('hidden');
          document.getElementById('unpaired-view').classList.add('hidden');
          document.getElementById('paired-view').classList.remove('hidden');
          
          const badge = document.getElementById('status-badge');
          badge.className = 'badge badge-green';
          badge.innerHTML = '<div class="dot dot-green"></div><span>Aktif & Bağlı</span>';

          if (s.server_url) {{
            try {{
              const u = new URL(s.server_url);
              document.getElementById('stat-server').innerText = u.host;
            }} catch(e) {{
              document.getElementById('stat-server').innerText = s.server_url;
            }}
          }}
          if (s.node_id) {{
            document.getElementById('stat-node').innerText = "Node: " + s.node_id.substring(0, 8);
          }}
          if (s.battery_level !== undefined) {{
            const chg = s.is_charging ? " (Şarjda)" : "";
            document.getElementById('stat-battery').innerHTML = s.battery_level + "%" + chg + " / " + (s.battery_temp || 31) + "&deg;C";
          }}
          if (s.current_tps !== undefined && s.current_tps !== null) {{
            const val = typeof s.current_tps === 'number' ? s.current_tps.toFixed(1) : s.current_tps;
            document.getElementById('stat-tps').innerText = val;
          }} else if (s.tps !== undefined && s.tps !== null) {{
            document.getElementById('stat-tps').innerText = s.tps;
          }} else {{
            document.getElementById('stat-tps').innerText = '0.0';
          }}
          if (s.assigned_model) {{
            document.getElementById('stat-model-name').innerText = s.assigned_model;
          }}
          if (s.model_status_desc) {{
            document.getElementById('stat-model-desc').innerText = s.model_status_desc;
          }}
          const progressPct = s.download_progress !== undefined && s.download_progress !== null ? s.download_progress : 0;
          const isDownloading = s.model_status === 'downloading' || (progressPct > 0 && progressPct < 100);
          if (isDownloading) {{
            document.getElementById('download-progress-container').classList.remove('hidden');
            document.getElementById('btn-test-model').classList.add('hidden');
            const dMB = s.downloaded_mb || 0;
            const tMB = s.total_mb || 380;
            const pct = progressPct || (tMB > 0 ? Math.floor((dMB / tMB) * 100) : 0);
            document.getElementById('download-progress-text').innerText = dMB + ' MB / ' + tMB + ' MB';
            document.getElementById('download-progress-pct').innerText = '%' + pct;
            document.getElementById('download-progress-bar').style.width = Math.max(0, Math.min(100, pct)) + '%';
          }} else {{
            document.getElementById('download-progress-container').classList.add('hidden');
            document.getElementById('btn-test-model').classList.remove('hidden');
          }}
          if (s.download_error && s.download_error.length > 0) {{
            const errEl = document.getElementById('download-error-text');
            if (errEl) {{
              errEl.innerText = 'İndirme hatası: ' + s.download_error;
              errEl.classList.remove('hidden');
            }}
          }} else {{
            const errEl = document.getElementById('download-error-text');
            if (errEl) errEl.classList.add('hidden');
          }}
          if (s.last_crash && s.last_crash.length > 0) {{
            const crashEl = document.getElementById('crash-text');
            if (crashEl) {{
              crashEl.innerText = 'Son hata: ' + s.last_crash.split('\\n')[0];
              crashEl.classList.remove('hidden');
            }}
          }} else {{
            const crashEl = document.getElementById('crash-text');
            if (crashEl) crashEl.classList.add('hidden');
          }}
          const deviceDetails = document.getElementById('device-details');
          if (deviceDetails) {{
            const hasDetails = s.physical_ram_mb || s.cpu_cores || s.free_disk_gb || s.model_file_mb;
            deviceDetails.classList.toggle('hidden', !hasDetails);
            if (hasDetails) {{
              const ram = document.getElementById('device-ram');
              if (ram) ram.innerText = s.physical_ram_mb ? (s.physical_ram_mb / 1024).toFixed(1) + ' GB' : '-';
              const cores = document.getElementById('device-cores');
              if (cores) cores.innerText = s.cpu_cores ? s.cpu_cores + ' çekirdek' : '-';
              const disk = document.getElementById('device-disk');
              if (disk) disk.innerText = s.free_disk_gb ? s.free_disk_gb + ' GB' : '-';
              const modelSize = document.getElementById('device-model-size');
              if (modelSize) modelSize.innerText = s.model_file_mb ? s.model_file_mb + ' MB' : '-';
            }}
          }}
          if (s.model_status) {{
            const mBadge = document.getElementById('stat-model-badge');
            if (s.model_status === 'ready') {{
              if (s.active_vram_model && s.active_vram_model.length > 0) {{
                mBadge.className = 'badge badge-green';
                mBadge.innerText = 'Hazır (VRAM)';
              }} else {{
                mBadge.className = 'badge badge-green';
                mBadge.innerText = 'Hazır (Beklemede)';
              }}
            }} else if (s.model_status === 'running') {{
              mBadge.className = 'badge badge-green animate-pulse';
              mBadge.innerText = 'Çalışıyor...';
            }} else if (s.model_status === 'downloading') {{
              mBadge.className = 'badge badge-amber animate-pulse';
              mBadge.innerText = 'İndiriliyor...';
            }} else {{
              mBadge.className = 'badge badge-amber';
              mBadge.innerText = 'Beklemede';
            }}
          }}
        }} else {{
          document.getElementById('paired-view').classList.add('hidden');
          if (s.permissions_done || permissionsCompleted) {{
            document.getElementById('permissions-view').classList.add('hidden');
            document.getElementById('unpaired-view').classList.remove('hidden');
          }} else {{
            document.getElementById('permissions-view').classList.remove('hidden');
            document.getElementById('unpaired-view').classList.add('hidden');
          }}

          const badge = document.getElementById('status-badge');
          badge.className = 'badge badge-amber';
          badge.innerHTML = '<div class="dot dot-amber"></div><span>Eşleştirme Bekleniyor</span>';
        }}
      }} catch(e) {{}}
    }};

    window.checkWorkerStatus = function() {{
      if (!window.AndroidBridge || !window.AndroidBridge.getWorkerStatus) return;
      try {{
        const raw = window.AndroidBridge.getWorkerStatus();
        const status = typeof raw === 'string' ? JSON.parse(raw) : raw;
        const badge = document.getElementById('status-badge');
        const connected = status.is_paired && status.service_running;
        const notice = document.getElementById('connection-notice');
        if (!connected) {{
          document.getElementById('paired-view').classList.add('hidden');
          badge.className = 'badge badge-amber';
          const message = status.device_removed_message || 'Bağlantı kesildi';
          badge.innerHTML = '<div class="dot dot-amber"></div><span>' + message + '</span>';
          if (notice) {{
            notice.innerText = status.device_removed
              ? 'Cihazınız ağdan kaldırıldı. Yeniden bağlanmak için eşleştirme kodunu tekrar girin.'
              : 'Bağlantı kesildi. Cihazınız çevrimdışı, yeniden eşleştirmek için eşleştirme kodunu girin.';
            notice.classList.remove('hidden');
          }}
          // The checklist of the permissions stays in front until every permission
          // is granted, the pairing is only offered afterwards.
          if (permissionsCompleted) {{
            document.getElementById('permissions-view').classList.add('hidden');
            document.getElementById('unpaired-view').classList.remove('hidden');
          }} else {{
            document.getElementById('permissions-view').classList.remove('hidden');
            document.getElementById('unpaired-view').classList.add('hidden');
          }}
        }} else {{
          if (notice) notice.classList.add('hidden');
        }}
      }} catch (e) {{}}
    }};

    window.showUpdatePrompt = function(version) {{
      const modal = document.getElementById('update-modal');
      const desc = document.getElementById('update-desc');
      if (desc && version) {{
        desc.innerText = "Yeni sürüm (" + version + ") indirildi. Kesintisiz çalışma için hemen yükleyebilirsiniz.";
      }}
      if (modal) modal.classList.remove('hidden');
    }};

    function closeUpdateModal() {{
      const modal = document.getElementById('update-modal');
      if (modal) modal.classList.add('hidden');
    }}

    function installUpdate() {{
      if (window.AndroidBridge && window.AndroidBridge.installDownloadedUpdate) {{
        window.AndroidBridge.installDownloadedUpdate();
      }}
    }}

    window.onPermissionRevoked = function() {{
      showToast("Arka plan aktivite izni geri çekildi! Lütfen tekrar izin veriniz.");
      document.getElementById('paired-view').classList.add('hidden');
      document.getElementById('unpaired-view').classList.add('hidden');
      document.getElementById('permissions-view').classList.remove('hidden');
    }};

    setInterval(function() {{
      refreshPermissions();
    }}, 1500);

    window.startLiveCheck = function() {{
      if (window.checkWorkerStatus) window.checkWorkerStatus();
      setInterval(function() {{
        if (window.checkWorkerStatus) window.checkWorkerStatus();
      }}, 5000);
    }};

    window.onload = function() {{
      refreshPermissions();
      if (window.AndroidBridge && window.AndroidBridge.getInitialState) {{
        const initStr = window.AndroidBridge.getInitialState();
        if (initStr) window.updateState(initStr);
      }}
      if (window.startLiveCheck) window.startLiveCheck();
    }};
  </script>
</body>
</html>
'''
with open(os.path.join(assets_www, "index.html"), "w", encoding="utf-8") as f:
    f.write(html_content)

# The behaviour of the views is verified, a broken interface must not reach the
# device. The check is skipped when node is not installed.
VIEW_TEST = os.path.join(os.path.dirname(os.path.abspath(__file__)), "view_test.cjs")
if shutil.which("node") and os.path.exists(VIEW_TEST):
    result = subprocess.run(["node", VIEW_TEST, os.path.join(assets_www, "index.html")])
    if result.returncode != 0:
        raise SystemExit("the interface of the application does not behave as expected")
else:
    print("node is not installed, the interface check is skipped")

# 3. Java Source Files

# ApkConfigReader.java
apk_config_reader_content = '''package org.byog.worker;

import java.io.RandomAccessFile;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;

public class ApkConfigReader {
    public static String readConfig(String apkPath) {
        if (apkPath == null) return null;
        try (RandomAccessFile raf = new RandomAccessFile(apkPath, "r")) {
            long fileLen = raf.length();
            if (fileLen < 22) return null;
            long searchLen = Math.min(fileLen, 65536);
            raf.seek(fileLen - searchLen);
            byte[] buf = new byte[(int) searchLen];
            raf.readFully(buf);
            int eocdOffset = -1;
            for (int i = buf.length - 22; i >= 0; i--) {
                if (buf[i] == 0x50 && buf[i+1] == 0x4b && buf[i+2] == 0x05 && buf[i+3] == 0x06) {
                    eocdOffset = i;
                    break;
                }
            }
            if (eocdOffset == -1) return null;
            ByteBuffer bb = ByteBuffer.wrap(buf, eocdOffset + 16, 4).order(ByteOrder.LITTLE_ENDIAN);
            long cdOffset = bb.getInt() & 0xFFFFFFFFL;
            if (cdOffset < 24) return null;
            raf.seek(cdOffset - 16);
            byte[] magic = new byte[16];
            raf.readFully(magic);
            if (!new String(magic, StandardCharsets.US_ASCII).equals("APK Sig Block 42")) return null;
            raf.seek(cdOffset - 24);
            byte[] sizeBuf = new byte[8];
            raf.readFully(sizeBuf);
            long blockSize = ByteBuffer.wrap(sizeBuf).order(ByteOrder.LITTLE_ENDIAN).getLong();
            long blockStart = cdOffset - 8 - blockSize;
            raf.seek(blockStart + 8);
            while (raf.getFilePointer() < cdOffset - 24) {
                raf.readFully(sizeBuf);
                long pairLen = ByteBuffer.wrap(sizeBuf).order(ByteOrder.LITTLE_ENDIAN).getLong();
                byte[] idBuf = new byte[4];
                raf.readFully(idBuf);
                int id = ByteBuffer.wrap(idBuf).order(ByteOrder.LITTLE_ENDIAN).getInt();
                int valueLen = (int) (pairLen - 4);
                if (id == 0x71777777) {
                    byte[] val = new byte[valueLen];
                    raf.readFully(val);
                    return new String(val, StandardCharsets.UTF_8);
                } else {
                    raf.skipBytes(valueLen);
                }
            }
        } catch (Exception ignored) {}
        return null;
    }
}
'''
with open(os.path.join(src_dir, "ApkConfigReader.java"), "w", encoding="utf-8") as f:
    f.write(apk_config_reader_content)

# MainActivity.java (Permission checklist methods, Honor Pad 10 autostart intent resolver, Real Hardware detection)
main_activity_content = '''package org.byog.worker;

import android.app.Activity;
import android.app.ActivityManager;
import android.content.BroadcastReceiver;
import android.content.ClipboardManager;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.net.Uri;
import android.os.BatteryManager;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.os.PowerManager;
import android.os.StatFs;
import android.provider.Settings;
import android.util.Base64;
import android.webkit.JavascriptInterface;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.Toast;

import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

public class MainActivity extends Activity {
    private WebView webView;
    private static final int REQ_QR_SCAN = 1001;
    private static final int REQ_POST_NOTIFS = 1002;
    private static final int REQ_CAMERA_PERM = 1003;
    private long lastPausedAt = 0;
    private JSONObject latestWorkerState = new JSONObject();

    private BroadcastReceiver stateReceiver = new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
            String action = intent != null ? intent.getAction() : "";
            if ("org.byog.worker.STATE_CHANGED".equals(action)) {
                String stateJson = intent.getStringExtra("state");
                if (stateJson != null) {
                    try {
                        latestWorkerState = new JSONObject(stateJson);
                    } catch (Exception ignored) {}
                    if (webView != null) {
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                webView.evaluateJavascript("window.updateState(" + stateJson + ")", null);
                            }
                        });
                    }
                }
            } else if ("org.byog.worker.UPDATE_AVAILABLE".equals(action)) {
                final String version = intent.getStringExtra("version");
                if (webView != null) {
                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            webView.evaluateJavascript("if (window.showUpdatePrompt) window.showUpdatePrompt('" + version + "')", null);
                        }
                    });
                }
            } else if ("org.byog.worker.PERMISSION_REVOKED".equals(action)) {
                if (webView != null) {
                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            webView.evaluateJavascript("if (window.onPermissionRevoked) window.onPermissionRevoked(); if (window.refreshPermissions) window.refreshPermissions();", null);
                        }
                    });
                }
            }
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        installCrashHandler();
        try {
            startInterface(savedInstanceState);
        } catch (Throwable failure) {
            android.util.Log.e("BYOG_CRASH", "The interface could not be started", failure);
        }
    }

    private void startInterface(Bundle savedInstanceState) {

        String appName = "{app_name}";
        try {
            String embedded = ApkConfigReader.readConfig(getApplicationInfo().sourceDir);
            if (embedded != null && !embedded.isEmpty()) {
                JSONObject obj = new JSONObject(embedded);
                SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                SharedPreferences.Editor ed = sp.edit();
                if (obj.has("server_url")) ed.putString("server_url", obj.getString("server_url"));
                if (obj.has("app_name")) {
                    appName = obj.getString("app_name");
                    ed.putString("app_name", appName);
                }
                if (obj.has("favicon")) {
                    String favUrl = obj.getString("favicon");
                    ed.putString("favicon", favUrl);
                    fetchAndCacheFavicon(favUrl);
                }
                if (obj.has("favicon_base64")) {
                    String b64 = obj.getString("favicon_base64");
                    ed.putString("favicon_base64", b64);
                    saveBase64ToCacheFile(b64);
                }
                ed.apply();
            } else {
                SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                appName = WorkerService.stringPref(sp, "app_name", "{app_name}");
            }
        } catch (Exception ignored) {}

        setTitle(appName);

        webView = new WebView(this);
        setContentView(webView);

        WebSettings ws = webView.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setAllowFileAccess(true);
        ws.setAllowContentAccess(true);
        ws.setAllowFileAccessFromFileURLs(true);
        ws.setAllowUniversalAccessFromFileURLs(true);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            ws.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        }
        webView.setWebChromeClient(new WebChromeClient());
        webView.setBackgroundColor(0xFF020817);

        webView.addJavascriptInterface(new WebAppInterface(), "AndroidBridge");
        webView.loadUrl("file:///android_asset/www/index.html");

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            try {
                getWindow().setSustainedPerformanceMode(true);
            } catch (Exception ignored) {}
        }

        IntentFilter filter = new IntentFilter();
        filter.addAction("org.byog.worker.STATE_CHANGED");
        filter.addAction("org.byog.worker.UPDATE_AVAILABLE");
        filter.addAction("org.byog.worker.PERMISSION_REVOKED");
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(stateReceiver, filter, Context.RECEIVER_NOT_EXPORTED);
        } else {
            registerReceiver(stateReceiver, filter);
        }

        fetchInitialServerBranding();

        SharedPreferences spInit = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        if (WorkerService.boolPref(spInit, "is_paired", false) && !WorkerService.stringPref(spInit, "node_id", "").isEmpty()) {
            try {
                Intent sIntent = new Intent(this, WorkerService.class);
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    startForegroundService(sIntent);
                } else {
                    startService(sIntent);
                }
            } catch (Exception ignored) {}
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (webView != null) {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            boolean isBgRestricted = false;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                ActivityManager am = (ActivityManager) getSystemService(Context.ACTIVITY_SERVICE);
                if (am != null && am.isBackgroundRestricted()) {
                    isBgRestricted = true;
                }
            }

            if (isBgRestricted) {
                sp.edit().putBoolean("bgactivity_granted", false).apply();
            } else {
                if (WorkerService.boolPref(sp, "bgactivity_pending_check", false)) {
                    sp.edit().putBoolean("bgactivity_pending_check", false).putBoolean("bgactivity_granted", true).apply();
                }
            }
            if (WorkerService.boolPref(sp, "autostart_pending_check", false)) {
                sp.edit().putBoolean("autostart_pending_check", false).putBoolean("autostart_granted", true).apply();
            }

            if (WorkerService.boolPref(sp, "is_paired", false) && !WorkerService.stringPref(sp, "node_id", "").isEmpty()) {
                try {
                    Intent sIntent = new Intent(this, WorkerService.class);
                    sIntent.putExtra("action", "refresh_state");
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        startForegroundService(sIntent);
                    } else {
                        startService(sIntent);
                    }
                } catch (Exception ignored) {}
            }

            webView.evaluateJavascript("if (window.refreshPermissions) window.refreshPermissions();", null);
        }
    }

    @Override
    protected void onPause() {
        lastPausedAt = System.currentTimeMillis();
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        try {
            unregisterReceiver(stateReceiver);
        } catch (Exception ignored) {}
        super.onDestroy();
    }

    /**
     * Opens the OEM "Auto start / Startup" manager of the device.
     */
    /**
     * Launches an intent and reports whether the screen was opened.
     *
     * The activities of the vendors are protected with signature permissions, the
     * launch can be rejected with a security exception. Every candidate is therefore
     * resolved with the package manager and the exception is caught.
     */
    private boolean launch(Intent intent) {
        if (intent == null) return false;
        try {
            intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            if (intent.getComponent() != null) {
                PackageManager pm = getPackageManager();
                if (pm == null || pm.resolveActivity(intent, PackageManager.MATCH_DEFAULT_ONLY) == null) {
                    return false;
                }
            }
            startActivity(intent);
            return true;
        } catch (Throwable ignored) {
            return false;
        }
    }

    /**
     * Builds an intent for an activity of a system application.
     */
    private Intent componentIntent(String packageName, String className) {
        return new Intent().setComponent(new ComponentName(packageName, className));
    }

    /**
     * Builds the intent for the screen that holds the background activity switch of an
     * application. The screen expects the name of the application and its package.
     */
    private Intent perAppPowerIntent(String packageName, String className) {
        SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        String label = sp != null ? WorkerService.stringPref(sp, "app_name", "BYOG Worker") : "BYOG Worker";
        Intent intent = componentIntent(packageName, className);
        intent.setData(Uri.parse("package:" + getPackageName()));
        intent.putExtra("title", label);
        intent.putExtra("pkgName", getPackageName());
        intent.putExtra("package_name", getPackageName());
        intent.putExtra("packageName", getPackageName());
        intent.putExtra("package_label", label);
        intent.putExtra("drainType", "APP");
        return intent;
    }

    /**
     * Marks the auto start screen as open, the permission is only counted as granted
     * when the user really came back from a screen of the system.
     */
    private void markAutoStartPending() {
        SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        sp.edit().putBoolean("autostart_pending_check", true).apply();
    }

    /**
     * Marks the background activity screen as open.
     */
    private void markBackgroundActivityPending() {
        SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        sp.edit().putBoolean("bgactivity_pending_check", true).apply();
    }

    /**
     * Opens the "Otomatik Başlatma" screen of the device.
     *
     * The intent actions are tried first, they survive the renames of the activities
     * between the versions of the system. Afterwards the known activities of the
     * vendors are used.
     */
    public void openAutoStartSettings() {
        lastPausedAt = 0;

        Toast.makeText(this, "Otomatik başlatma ekranında uygulamayı etkinleştirin.", Toast.LENGTH_LONG).show();

        for (Intent action : autoStartActions()) {
            if (launch(action)) {
                markAutoStartPending();
                return;
            }
        }

        for (Intent component : autoStartComponents()) {
            if (launch(component)) {
                markAutoStartPending();
                return;
            }
        }

        Toast.makeText(this, "Otomatik başlatma ekranı bulunamadı. Ayarlar > Uygulamalar > BYOG > Pil kullanımı bölümünden açabilirsiniz.", Toast.LENGTH_LONG).show();
    }

    /**
     * Tells whether the system of the device holds the screen of the automatic
     * start. The screen is not on every device, the worker starts after the
     * reboot with its own receiver when it is missing.
     */
    public boolean hasAutoStartScreen() {
        for (Intent action : autoStartActions()) {
            if (canLaunch(action)) {
                return true;
            }
        }
        for (Intent component : autoStartComponents()) {
            if (canLaunch(component)) {
                return true;
            }
        }
        return false;
    }

    /**
     * Tells whether the system of the device holds the screen of the background
     * activity. The screen is not on every device.
     */
    public boolean hasBackgroundActivityScreen() {
        Intent[] actions = new Intent[] {
            new Intent("com.android.settings.action.BACKGROUND_OPTIMIZE"),
            new Intent("android.settings.IGNORE_BATTERY_OPTIMIZATION_SETTINGS"),
        };
        for (Intent action : actions) {
            if (canLaunch(action)) {
                return true;
            }
        }
        return false;
    }

    /**
     * Tells whether the system of the device can open the given screen.
     */
    private boolean canLaunch(Intent intent) {
        try {
            return intent.resolveActivity(getPackageManager()) != null;
        } catch (Exception ignored) {
            return false;
        }
    }

    private Intent[] autoStartActions() {
        return new Intent[] {
            new Intent("miui.intent.action.OP_AUTO_START"),
            new Intent("huawei.intent.action.HSM_STARTUPAPP_MANAGER"),
            new Intent("com.iqoo.secure.BGSTARTUPMANAGER"),
            new Intent("com.cyin.himgr.applicationmanager.view.activities.AUTO_START_ACTIVITY"),
            new Intent("com.android.settings.action.BACKGROUND_OPTIMIZE"),
        };
    }

    private Intent[] autoStartComponents() {
        return new Intent[] {
            // realme UI 6/7 and ColorOS 15 or newer
            componentIntent("com.oplus.safecenter", "com.oplus.safecenter.startupapp.StartupAppListActivity"),
            componentIntent("com.oplus.safecenter", "com.oplus.safecenter.permission.startup.StartupAppListActivity"),
            // realme UI 5 and ColorOS 14 or older
            componentIntent("com.coloros.safecenter", "com.coloros.safecenter.startupapp.StartupAppListActivity"),
            componentIntent("com.coloros.safecenter", "com.coloros.safecenter.permission.startup.StartupAppListActivity"),
            componentIntent("com.coloros.safecenter", "com.coloros.safecenter.permission.startupapp.StartupAppListActivity"),
            componentIntent("com.coloros.safecenter", "com.coloros.safecenter.permission.startupmanager.StartupAppListActivity"),
            // the permission hub of the security center holds the auto start entry
            componentIntent("com.coloros.safecenter", "com.coloros.privacypermissionsentry.PermissionTopActivity"),
            componentIntent("com.oplus.safecenter", "com.oplus.safecenter.permission.topactivity.PermissionTopActivity"),
            // older ColorOS
            componentIntent("com.oppo.safe", "com.oppo.safe.permission.startup.StartupAppListActivity"),
            componentIntent("com.coloros.safe", "com.coloros.safe.permission.startup.StartupAppListActivity"),
            componentIntent("com.coloros.oppoguardelf", "com.coloros.powermanager.fuelgaue.PowerUsageModelActivity"),
            componentIntent("com.oplus.battery", "com.oplus.startupapp.view.StartupAppListActivity"),
            // HONOR and HUAWEI
            componentIntent("com.hihonor.systemmanager", "com.hihonor.systemmanager.startupmgr.ui.StartupNormalAppListActivity"),
            componentIntent("com.hihonor.systemmanager", "com.hihonor.systemmanager.appcontrol.activity.StartupAppControlActivity"),
            componentIntent("com.hihonor.systemmanager", "com.hihonor.systemmanager.optimize.process.ProtectActivity"),
            componentIntent("com.huawei.systemmanager", "com.huawei.systemmanager.startupmgr.ui.StartupNormalAppListActivity"),
            componentIntent("com.huawei.systemmanager", "com.huawei.systemmanager.appcontrol.activity.StartupAppControlActivity"),
            componentIntent("com.huawei.systemmanager", "com.huawei.systemmanager.optimize.process.ProtectActivity"),
            // Xiaomi
            componentIntent("com.miui.securitycenter", "com.miui.permcenter.autostart.AutoStartManagementActivity"),
            // vivo and iQOO
            componentIntent("com.vivo.permissionmanager", "com.vivo.permissionmanager.activity.BgStartUpManagerActivity"),
            componentIntent("com.iqoo.secure", "com.iqoo.secure.ui.phoneoptimize.BgStartUpManager"),
            // Samsung
            componentIntent("com.samsung.android.lool", "com.samsung.android.sm.battery.ui.usage.CheckableAppListActivity"),
            componentIntent("com.samsung.android.lool", "com.samsung.android.sm.ui.battery.BatteryActivity"),
            componentIntent("com.samsung.android.sm", "com.samsung.android.sm.ui.battery.BatteryActivity"),
            // Transsion
            componentIntent("com.transsion.phonemaster", "com.cyin.himgr.autostart.AutoStartActivity"),
            componentIntent("com.transsion.phonemanager", "com.itel.autobootmanager.activity.AutoBootMgrActivity"),
        };
    }

    /**
     * Opens the screen that holds the "Arka plan aktivitesine izin ver" switch of the
     * application. On realme and OPPO this is the power control of the application,
     * on HONOR the detail of the power consumption.
     */
    public void openBackgroundActivitySettings() {
        lastPausedAt = 0;

        Toast.makeText(this, "Pil kullanımı ekranında 'Arka plan aktivitesine izin ver' seçeneğini açın.", Toast.LENGTH_LONG).show();

        Intent[] candidates = new Intent[] {
            // realme UI and ColorOS: the power control of the application
            perAppPowerIntent("com.oplus.battery", "com.oplus.powermanager.fuelgaue.PowerControlActivity"),
            perAppPowerIntent("com.coloros.oppoguardelf", "com.coloros.powermanager.fuelgaue.PowerControlActivity"),
            perAppPowerIntent("com.coloros.safecenter", "com.coloros.powermanager.fuelgaue.PowerControlActivity"),
            // realme UI and ColorOS: the list of the power consumption
            perAppPowerIntent("com.oplus.battery", "com.oplus.powermanager.fuelgaue.PowerConsumptionActivity"),
            perAppPowerIntent("com.coloros.oppoguardelf", "com.coloros.powermanager.fuelgaue.PowerConsumptionActivity"),
            perAppPowerIntent("com.oplus.battery", "com.oplus.battery.AppBatteryUseDetailActivity"),
            // HONOR: the detail of the power consumption holds the background activity
            perAppPowerIntent("com.hihonor.systemmanager", "com.hihonor.systemmanager.power.ui.DetailOfSoftConsumptionActivity"),
            perAppPowerIntent("com.huawei.systemmanager", "com.huawei.systemmanager.power.ui.HwPowerManagerActivity"),
            // Xiaomi
            perAppPowerIntent("com.miui.powerkeeper", "com.miui.powerkeeper.ui.HiddenAppsConfigActivity"),
            // vivo and iQOO
            perAppPowerIntent("com.vivo.abe", "com.vivo.applicationbehaviorengine.ui.ExcessivePowerManagerActivity"),
            // Samsung
            perAppPowerIntent("com.samsung.android.lool", "com.samsung.android.sm.battery.ui.usage.CheckableAppListActivity"),
            // the generic screen of the system
            perAppPowerIntent("com.android.settings", "com.android.settings.fuelgauge.AdvancedPowerUsageDetailActivity"),
            perAppPowerIntent("com.android.settings", "com.android.settings.Settings$AppBatteryUsageActivity"),
        };
        for (Intent candidate : candidates) {
            if (launch(candidate)) {
                markBackgroundActivityPending();
                return;
            }
        }

        // The generic screen of the system, it exists on every device.
        Intent generic = new Intent("android.settings.APP_BATTERY_SETTINGS");
        generic.setData(Uri.parse("package:" + getPackageName()));
        generic.putExtra("package", getPackageName());
        generic.putExtra("package_name", getPackageName());
        generic.putExtra(Intent.EXTRA_PACKAGE_NAME, getPackageName());
        if (launch(generic)) {
            markBackgroundActivityPending();
            return;
        }

        Intent details = new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS);
        details.setData(Uri.parse("package:" + getPackageName()));
        if (launch(details)) {
            markBackgroundActivityPending();
            return;
        }

        Toast.makeText(this, "Pil kullanımı ekranı bulunamadı. Ayarlar > Uygulamalar > BYOG > Pil kullanımı bölümünden açabilirsiniz.", Toast.LENGTH_LONG).show();
    }

    private void saveBase64ToCacheFile(String base64Data) {
        try {
            if (base64Data == null || !base64Data.contains(",")) return;
            String rawB64 = base64Data.substring(base64Data.indexOf(",") + 1);
            byte[] bytes = Base64.decode(rawB64, Base64.DEFAULT);
            File f = new File(getFilesDir(), "custom_favicon.png");
            FileOutputStream fos = new FileOutputStream(f);
            fos.write(bytes);
            fos.close();
        } catch (Exception ignored) {}
    }



    /**
     * Installs a crash handler that records the failure and reports it to the
     * coordinator.
     *
     * The interface is only restarted when the previous crash is older than five
     * seconds. A restart on every crash would keep the application in a crash
     * loop and it could not be opened any more.
     */
    private void installCrashHandler() {
        try {
            final SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            Thread.setDefaultUncaughtExceptionHandler(new Thread.UncaughtExceptionHandler() {
                @Override
                public void uncaughtException(Thread t, Throwable e) {
                    try {
                        java.io.StringWriter sw = new java.io.StringWriter();
                        e.printStackTrace(new java.io.PrintWriter(sw));
                        String stack = sw.toString();
                        String msg = e.getMessage() != null ? e.getMessage() : e.toString();
                        android.util.Log.e("BYOG_CRASH", "Uncaught exception on " + t.getName() + ": " + msg, e);
                        long now = System.currentTimeMillis();
                        long previousCrash = WorkerService.longPref(sp, "last_crash_at", 0L);
                        sp.edit()
                                .putString("last_crash", msg + "\\n" + stack)
                                .putLong("last_crash_at", now)
                                .commit();
                        reportCrashToServer("ERR_UNCAUGHT", msg, stack);
                        if (t == android.os.Looper.getMainLooper().getThread()) {
                            if (now - previousCrash > 5000L) {
                                try {
                                    Intent restart = new Intent(MainActivity.this, MainActivity.class);
                                    restart.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP);
                                    MainActivity.this.startActivity(restart);
                                } catch (Throwable ignored) {}
                            }
                            try {
                                MainActivity.this.finish();
                            } catch (Throwable ignored) {}
                            try {
                                android.os.Process.killProcess(android.os.Process.myPid());
                            } catch (Throwable ignored) {}
                        }
                    } catch (Throwable ignored) {}
                }
            });
        } catch (Throwable ignored) {}
    }

    /**
     * Sends a crash report to the coordinator.
     */
    private void reportCrashToServer(final String errorCode, final String errorMessage, final String stackTrace) {
        try {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            final String serverUrl = WorkerService.stringPref(sp, "server_url", "");
            final String nodeId = WorkerService.stringPref(sp, "node_id", "");
            if (serverUrl.isEmpty()) return;
            new Thread(new Runnable() {
                @Override
                public void run() {
                    try {
                        JSONObject body = new JSONObject();
                        body.put("node_id", nodeId);
                        body.put("error_code", errorCode);
                        body.put("error_message", errorMessage);
                        body.put("stack_trace", stackTrace);
                        body.put("created_at", System.currentTimeMillis() / 1000);

                        URL url = new URL(serverUrl + "/api/v1/worker/telemetry/crash");
                        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                        conn.setRequestMethod("POST");
                        conn.setRequestProperty("Content-Type", "application/json");
                        conn.setConnectTimeout(6000);
                        conn.setReadTimeout(6000);
                        conn.setDoOutput(true);
                        OutputStream os = conn.getOutputStream();
                        os.write(body.toString().getBytes("UTF-8"));
                        os.close();
                        conn.getResponseCode();
                        conn.disconnect();
                    } catch (Throwable ignored) {}
                }
            }).start();
        } catch (Throwable ignored) {}
    }

    private void fetchInitialServerBranding() {
        SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        String serverUrl = WorkerService.stringPref(sp, "server_url", "");
        if (serverUrl == null || serverUrl.trim().isEmpty()) return;
        while (serverUrl.endsWith("/")) serverUrl = serverUrl.substring(0, serverUrl.length() - 1);
        final String targetUrl = serverUrl + "/api/status";

        new Thread(new Runnable() {
            @Override
            public void run() {
                try {
                    URL u = new URL(targetUrl);
                    HttpURLConnection conn = (HttpURLConnection) u.openConnection();
                    conn.setConnectTimeout(4000);
                    conn.setReadTimeout(4000);
                    conn.setRequestProperty("User-Agent", "BYOG-Worker/1.0");
                    if (conn.getResponseCode() == 200) {
                        InputStream is = conn.getInputStream();
                        ByteArrayOutputStream bos = new ByteArrayOutputStream();
                        byte[] buf = new byte[4096];
                        int r;
                        while ((r = is.read(buf)) != -1) bos.write(buf, 0, r);
                        is.close();
                        conn.disconnect();

                        JSONObject resp = new JSONObject(bos.toString("UTF-8"));
                        if (resp.optBoolean("success", true) && resp.has("data")) {
                            JSONObject d = resp.getJSONObject("data");
                            String sysName = d.optString("system_name", "");
                            String fav = d.optString("favicon", "");
                            if (sysName.isEmpty()) sysName = d.optString("app_name", "");
                            if (!sysName.isEmpty() && !sysName.equalsIgnoreCase("New API") && !sysName.equalsIgnoreCase("New-API")) {
                                final String newAppName = sysName.endsWith("BYOG") ? sysName : (sysName + " BYOG");
                                SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                                sp.edit().putString("app_name", newAppName).apply();
                                runOnUiThread(new Runnable() {
                                    @Override
                                    public void run() {
                                        setTitle(newAppName);
                                        if (webView != null) {
                                            webView.evaluateJavascript("if (document.getElementById('app-title')) document.getElementById('app-title').innerText = '" + newAppName + "';", null);
                                        }
                                    }
                                });
                            }
                            if (!fav.isEmpty()) {
                                fetchAndCacheFavicon(fav);
                            }
                        }
                    }
                } catch (Exception ignored) {}
            }
        }).start();
    }

    private void fetchAndCacheFavicon(final String faviconUrl) {
        if (faviconUrl == null || faviconUrl.trim().isEmpty() || faviconUrl.startsWith("data:")) return;
        new Thread(new Runnable() {
            @Override
            public void run() {
                try {
                    URL url = new URL(faviconUrl);
                    HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                    conn.setConnectTimeout(8000);
                    conn.setReadTimeout(8000);
                    conn.setInstanceFollowRedirects(true);
                    if (conn.getResponseCode() == 200) {
                        InputStream is = conn.getInputStream();
                        ByteArrayOutputStream buffer = new ByteArrayOutputStream();
                        int nRead;
                        byte[] data = new byte[8192];
                        while ((nRead = is.read(data, 0, data.length)) != -1) {
                            buffer.write(data, 0, nRead);
                        }
                        buffer.flush();
                        byte[] bytes = buffer.toByteArray();
                        is.close();
                        conn.disconnect();

                        if (bytes.length > 0) {
                            File f = new File(getFilesDir(), "custom_favicon.png");
                            FileOutputStream fos = new FileOutputStream(f);
                            fos.write(bytes);
                            fos.close();

                            String b64 = Base64.encodeToString(bytes, Base64.NO_WRAP);
                            String mime = faviconUrl.endsWith(".ico") ? "image/x-icon" : "image/png";
                            final String dataUri = "data:" + mime + ";base64," + b64;

                            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                            sp.edit().putString("favicon_base64", dataUri).apply();

                            runOnUiThread(new Runnable() {
                                @Override
                                public void run() {
                                    if (webView != null) {
                                        webView.evaluateJavascript("if (window.updateLogo) window.updateLogo('" + dataUri + "')", null);
                                    }
                                }
                            });
                        }
                    }
                } catch (Exception ignored) {}
            }
        }).start();
    }

    public class WebAppInterface {
        @JavascriptInterface
        public String getPermissionStatus() {
            JSONObject obj = new JSONObject();
            try {
                boolean notif = true;
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                    notif = checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED;
                }

                boolean battery = true;
                PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
                if (pm != null && Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                    battery = pm.isIgnoringBatteryOptimizations(getPackageName());
                }

                boolean bgAllowed = true;
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    ActivityManager am = (ActivityManager) getSystemService(Context.ACTIVITY_SERVICE);
                    if (am != null && am.isBackgroundRestricted()) {
                        bgAllowed = false;
                    }
                }

                SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                boolean autostart = WorkerService.boolPref(sp, "autostart_granted", false);

                obj.put("notifications", notif);
                obj.put("battery", battery);
                obj.put("autostart", autostart);
                obj.put("bgactivity", bgAllowed);
                obj.put("autostart_supported", hasAutoStartScreen());
                obj.put("bgactivity_supported", hasBackgroundActivityScreen());
            } catch (Exception ignored) {}
            return obj.toString();
        }

        @JavascriptInterface
        public void requestNotificationPermission() {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                if (checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                    requestPermissions(new String[]{android.Manifest.permission.POST_NOTIFICATIONS}, REQ_POST_NOTIFS);
                    return;
                }
            }
            if (webView != null) webView.evaluateJavascript("if (window.refreshPermissions) window.refreshPermissions();", null);
        }

        @JavascriptInterface
        public void requestBatteryOptimization() {
            try {
                PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
                if (pm != null && Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                    if (!pm.isIgnoringBatteryOptimizations(getPackageName())) {
                        Intent intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
                        intent.setData(Uri.parse("package:" + getPackageName()));
                        startActivity(intent);
                        return;
                    }
                }
            } catch (Exception ignored) {}
            if (webView != null) webView.evaluateJavascript("if (window.refreshPermissions) window.refreshPermissions();", null);
        }

        @JavascriptInterface
        public void openAutoStartSettings() {
            MainActivity.this.openAutoStartSettings();
        }

        @JavascriptInterface
        public void openBackgroundActivitySettings() {
            MainActivity.this.openBackgroundActivitySettings();
        }

        @JavascriptInterface
        public void installDownloadedUpdate() {
            try {
                File apkFile = new File(getCacheDir(), "byog-update.apk");
                if (!apkFile.exists()) {
                    Toast.makeText(MainActivity.this, "Güncelleme dosyası bulunamadı veya henüz indiriliyor.", Toast.LENGTH_SHORT).show();
                    return;
                }
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    if (!getPackageManager().canRequestPackageInstalls()) {
                        Intent permIntent = new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES);
                        permIntent.setData(Uri.parse("package:" + getPackageName()));
                        permIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                        startActivity(permIntent);
                        Toast.makeText(MainActivity.this, "Lütfen güncelleme yüklemek için bilinmeyen kaynaklara izin verin.", Toast.LENGTH_LONG).show();
                        return;
                    }
                }
                Uri contentUri = Uri.parse("content://org.byog.worker.fileprovider/" + apkFile.getName());
                Intent installIntent = new Intent(Intent.ACTION_VIEW);
                installIntent.setDataAndType(contentUri, "application/vnd.android.package-archive");
                installIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
                installIntent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
                startActivity(installIntent);
            } catch (Exception e) {
                Toast.makeText(MainActivity.this, "Güncelleme başlatılamadı: " + e.getMessage(), Toast.LENGTH_LONG).show();
            }
        }

        @JavascriptInterface
        public void confirmAutoStartPermission(boolean granted) {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            sp.edit().putBoolean("autostart_granted", granted).apply();
        }

        @JavascriptInterface
        public void testModelInference(final String prompt) {
            new Thread(new Runnable() {
                @Override
                public void run() {
                    String model = "qwen-2.5-0.5b-instruct";
                    try {
                        SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                        model = WorkerService.stringPref(sp, "assigned_model", "qwen-2.5-0.5b-instruct");
                    } catch (Exception ignored) {}

                    File mf = new File(getFilesDir(), model + ".gguf");
                    if (!mf.exists() || mf.length() < 10 * 1024 * 1024) {
                        final String errMsg = "Model dosyası (" + model + ".gguf) henüz indirilmedi veya eksik! Lütfen model indirmesi tamamlandıktan sonra test edin.";
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                if (webView != null) {
                                    String js = String.format(java.util.Locale.US, "if (window.onModelTestResult) window.onModelTestResult(%s, %d, %.1f);",
                                        JSONObject.quote(errMsg), 0, 0.0f);
                                    webView.evaluateJavascript(js, null);
                                }
                            }
                        });
                        return;
                    }

                    LocalNeuralEngine engine = new LocalNeuralEngine(model, getFilesDir());
                    LocalNeuralEngine.InferenceResult res = engine.generateStream(prompt, 256, 0.7f, 0.92f, null, null);

                    final String finalReply = res.fullText;
                    final int finalLatency = res.ttftMs;
                    final float finalTps = res.tps;

                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            if (webView != null) {
                                String js = String.format(java.util.Locale.US, "if (window.onModelTestResult) window.onModelTestResult(%s, %d, %.1f);",
                                    JSONObject.quote(finalReply), finalLatency, finalTps);
                                webView.evaluateJavascript(js, null);
                            }
                        }
                    });
                }
            }).start();
        }

        @JavascriptInterface
        public void scanQrCode() {
            if (checkSelfPermission(android.Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(new String[]{android.Manifest.permission.CAMERA}, REQ_CAMERA_PERM);
                return;
            }
            Intent intent = new Intent(MainActivity.this, QRScanActivity.class);
            startActivityForResult(intent, REQ_QR_SCAN);
        }

        @JavascriptInterface
        public String pasteFromClipboard() {
            try {
                ClipboardManager cm = (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                if (cm != null && cm.hasPrimaryClip() && cm.getPrimaryClip().getItemCount() > 0) {
                    CharSequence text = cm.getPrimaryClip().getItemAt(0).getText();
                    return text != null ? text.toString() : "";
                }
            } catch (Exception ignored) {}
            return "";
        }

        @JavascriptInterface
        public void connectWithMeta(final String code, final String serverUrl, final String metaAppName, final String metaFavicon) {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            SharedPreferences.Editor ed = sp.edit();
            if (metaAppName != null && !metaAppName.isEmpty()) {
                ed.putString("app_name", metaAppName);
            }
            if (metaFavicon != null && !metaFavicon.isEmpty()) {
                ed.putString("favicon", metaFavicon);
                fetchAndCacheFavicon(metaFavicon);
            }
            ed.apply();
            connect(code, serverUrl);
        }

        @JavascriptInterface
        public void connect(final String rawPairingCode, String rawServerUrl) {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            if (rawServerUrl == null || rawServerUrl.trim().isEmpty()) {
                rawServerUrl = WorkerService.stringPref(sp, "server_url", "");
            }
            while (rawServerUrl.endsWith("/")) {
                rawServerUrl = rawServerUrl.substring(0, rawServerUrl.length() - 1);
            }
            final String finalServerUrl = rawServerUrl;
            final String cleanCode = rawPairingCode != null ? rawPairingCode.trim().toUpperCase() : "";

            if (finalServerUrl.isEmpty()) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        Toast.makeText(MainActivity.this, "Lütfen sunucu adresini içeren QR kodu tarayın veya sunucudan indirilmiş APK kullanın", Toast.LENGTH_LONG).show();
                    }
                });
                return;
            }

            if (cleanCode.isEmpty()) {
                runOnUiThread(new Runnable() {
                    @Override
                    public void run() {
                        Toast.makeText(MainActivity.this, "Lütfen eşleştirme kodunu girin", Toast.LENGTH_SHORT).show();
                    }
                });
                return;
            }

            new Thread(new Runnable() {
                @Override
                public void run() {
                    try {
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                if (webView != null) {
                                    webView.evaluateJavascript("window.setConnectingState(true)", null);
                                }
                            }
                        });

                        // Real Hardware Telemetry Detection
                        ActivityManager actMgr = (ActivityManager) getSystemService(Context.ACTIVITY_SERVICE);
                        ActivityManager.MemoryInfo memInfo = new ActivityManager.MemoryInfo();
                        if (actMgr != null) actMgr.getMemoryInfo(memInfo);
                        int realPhysicalRamMB = (int) (memInfo.totalMem / (1024L * 1024L));
                        if (realPhysicalRamMB <= 0) realPhysicalRamMB = 4096;

                        int cores = Runtime.getRuntime().availableProcessors();
                        if (cores <= 0) cores = 4;

                        Intent batteryIntent = registerReceiver(null, new IntentFilter(Intent.ACTION_BATTERY_CHANGED));
                        int level = -1;
                        int scale = 100;
                        float temp = 31.0f;
                        boolean isCharging = false;
                        if (batteryIntent != null) {
                            level = batteryIntent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1);
                            scale = batteryIntent.getIntExtra(BatteryManager.EXTRA_SCALE, 100);
                            temp = batteryIntent.getIntExtra(BatteryManager.EXTRA_TEMPERATURE, 310) / 10.0f;
                            int status = batteryIntent.getIntExtra(BatteryManager.EXTRA_STATUS, -1);
                            int chargePlug = batteryIntent.getIntExtra(BatteryManager.EXTRA_PLUGGED, -1);
                            boolean isPlugged = (chargePlug == BatteryManager.BATTERY_PLUGGED_AC || chargePlug == BatteryManager.BATTERY_PLUGGED_USB || chargePlug == BatteryManager.BATTERY_PLUGGED_WIRELESS || chargePlug > 0);
                            isCharging = isPlugged && (status == BatteryManager.BATTERY_STATUS_CHARGING || status == BatteryManager.BATTERY_STATUS_FULL);
                        }
                        int percent = (scale > 0 && level >= 0) ? (int) ((level / (float) scale) * 100) : 100;

                        int totalDiskGB = 64;
                        int freeDiskGB = 16;
                        try {
                            java.io.File dataDir = Environment.getDataDirectory();
                            StatFs stat = new StatFs(dataDir.getPath());
                            long bs = stat.getBlockSizeLong();
                            totalDiskGB = (int) Math.max(1, (stat.getBlockCountLong() * bs) / (1024L * 1024L * 1024L));
                            freeDiskGB = (int) Math.max(1, (stat.getAvailableBlocksLong() * bs) / (1024L * 1024L * 1024L));
                        } catch (Exception ignored) {}

                        JSONObject req = new JSONObject();
                        req.put("hwid", "android-" + Build.BOARD + "-" + Build.DEVICE + "-" + Build.HARDWARE);
                        req.put("pairing_code", cleanCode);
                        req.put("os", "android");
                        req.put("arch", Build.SUPPORTED_ABIS[0]);
                        req.put("device_name", Build.MANUFACTURER + " " + Build.MODEL);
                        req.put("device_type", "mobile");
                        req.put("cpu_model", Build.HARDWARE + " (" + cores + " cores)");
                        req.put("physical_ram_mb", realPhysicalRamMB);
                        req.put("swap_ram_mb", 0);
                        req.put("vram_mb", 0); // Mobile uses unified memory (no discrete VRAM)
                        req.put("shared_vram_mb", 0);
                        req.put("total_disk_gb", totalDiskGB);
                        req.put("free_disk_gb", freeDiskGB);
                        req.put("battery_temp", temp);
                        req.put("battery_percent", percent);
                        req.put("is_charging", isCharging);

                        String[] endpoints = new String[]{
                            finalServerUrl + "/api/v1/worker/register",
                            finalServerUrl + "/api/byog/worker/register"
                        };

                        JSONObject resultObj = null;
                        String errorMsg = null;

                        for (String ep : endpoints) {
                            try {
                                URL url = new URL(ep);
                                HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                                conn.setRequestMethod("POST");
                                conn.setRequestProperty("Content-Type", "application/json");
                                conn.setConnectTimeout(8000);
                                conn.setReadTimeout(8000);
                                conn.setDoOutput(true);

                                OutputStream os = conn.getOutputStream();
                                os.write(req.toString().getBytes("UTF-8"));
                                os.close();

                                int code = conn.getResponseCode();
                                BufferedReader br = new BufferedReader(new InputStreamReader(
                                    code >= 200 && code < 400 ? conn.getInputStream() : conn.getErrorStream()
                                ));
                                StringBuilder sb = new StringBuilder();
                                String line;
                                while ((line = br.readLine()) != null) sb.append(line);
                                br.close();
                                conn.disconnect();

                                JSONObject resp = new JSONObject(sb.toString());
                                if (code == 200 && resp.optBoolean("success")) {
                                    resultObj = resp;
                                    break;
                                } else {
                                    errorMsg = resp.optString("message", "Sunucu hatası (" + code + ")");
                                }
                            } catch (Exception ex) {
                                errorMsg = ex.getMessage();
                            }
                        }

                        if (resultObj != null && resultObj.optBoolean("success")) {
                            JSONObject data = resultObj.getJSONObject("data");
                            final String returnedNodeId = data.optString("node_id");
                            final String returnedAppName = data.optString("app_name", "{app_name}");
                            final String returnedFavicon = data.optString("favicon", "logo.png");
                            final String returnedFaviconB64 = data.optString("favicon_base64", "");
                            boolean isPaired = data.optBoolean("is_paired", false);

                            String assignedModel = "qwen-2.5-0.5b-instruct";
                            if (data.has("assigned_models")) {
                                org.json.JSONArray mArr = data.optJSONArray("assigned_models");
                                if (mArr != null && mArr.length() > 0) {
                                    assignedModel = mArr.optString(0);
                                }
                            }

                            if (isPaired && !returnedNodeId.isEmpty()) {
                                SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                                SharedPreferences.Editor ed = sp.edit()
                                  .putString("pairing_code", cleanCode)
                                  .putString("server_url", finalServerUrl)
                                  .putString("node_id", returnedNodeId)
                                  .putString("app_name", returnedAppName)
                                  .putString("favicon", returnedFavicon)
                                  .putString("assigned_model", assignedModel)
                                  .putBoolean("is_paired", true)
                                  .putInt("battery_percent", percent)
                                  .putFloat("battery_temp", temp);

                                if (!returnedFaviconB64.isEmpty()) {
                                    ed.putString("favicon_base64", returnedFaviconB64);
                                    saveBase64ToCacheFile(returnedFaviconB64);
                                }
                                ed.commit();

                                if (returnedFaviconB64.isEmpty() && returnedFavicon.startsWith("http")) {
                                    fetchAndCacheFavicon(returnedFavicon);
                                }

                                final int finalPercent = percent;
                                final float finalTemp = temp;
                                final String finalAssignedModel = assignedModel;

                                runOnUiThread(new Runnable() {
                                    @Override
                                    public void run() {
                                        setTitle(returnedAppName);
                                        Intent sIntent = new Intent(MainActivity.this, WorkerService.class);
                                        sIntent.putExtra("pairing_code", cleanCode);
                                        sIntent.putExtra("server_url", finalServerUrl);
                                        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                                            startForegroundService(sIntent);
                                        } else {
                                            startService(sIntent);
                                        }

                                        Toast.makeText(MainActivity.this, "Cihaz başarıyla bağlandı", Toast.LENGTH_SHORT).show();

                                        JSONObject state = new JSONObject();
                                        try {
                                            state.put("is_paired", true);
                                            state.put("node_id", returnedNodeId);
                                            state.put("server_url", finalServerUrl);
                                            state.put("app_name", returnedAppName);
                                            state.put("favicon", returnedFavicon);
                                            if (!returnedFaviconB64.isEmpty()) {
                                                state.put("favicon_base64", returnedFaviconB64);
                                            }
                                            state.put("battery_level", finalPercent);
                                            state.put("battery_temp", finalTemp);
                                            state.put("assigned_model", finalAssignedModel);

                                            File mf = new File(getFilesDir(), finalAssignedModel + ".gguf");
                                            boolean modelExists = mf.exists() && mf.length() > 50 * 1024 * 1024;
                                            if (modelExists) {
                                                state.put("model_status", "ready");
                                                state.put("model_status_desc", finalAssignedModel + " hazır (Beklemede)");
                                                sp.edit().putString("model_status", "ready").commit();
                                            } else {
                                                state.put("model_status", "downloading");
                                                state.put("model_status_desc", finalAssignedModel + " indiriliyor (%0)...");
                                                state.put("download_progress", 0);
                                                state.put("downloaded_mb", 0);
                                                sp.edit().putString("model_status", "downloading").putInt("download_progress", 0).putInt("downloaded_mb", 0).commit();
                                            }
                                        } catch (Exception ignored) {}
                                        if (webView != null) {
                                            webView.evaluateJavascript("window.updateState(" + state.toString() + ")", null);
                                        }
                                    }
                                });
                                return;
                            }
                        }

                        final String finalErr = errorMsg != null ? errorMsg : "Eşleştirme kodu geçersiz veya süresi dolmuş";
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                if (webView != null) {
                                    webView.evaluateJavascript("window.setConnectingState(false)", null);
                                }
                                Toast.makeText(MainActivity.this, finalErr, Toast.LENGTH_LONG).show();
                            }
                        });

                    } catch (Exception e) {
                        final String exMsg = e.getMessage();
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                if (webView != null) {
                                    webView.evaluateJavascript("window.setConnectingState(false)", null);
                                }
                                Toast.makeText(MainActivity.this, "Bağlantı hatası: " + exMsg, Toast.LENGTH_LONG).show();
                            }
                        });
                    }
                }
            }).start();
        }

        @JavascriptInterface
        public void disconnect() {
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                    sp.edit().remove("node_id").remove("pairing_code").putBoolean("is_paired", false).apply();

                    stopService(new Intent(MainActivity.this, WorkerService.class));
                    Toast.makeText(MainActivity.this, "Bağlantı kesildi", Toast.LENGTH_SHORT).show();

                    JSONObject state = new JSONObject();
                    try {
                        state.put("is_paired", false);
                        state.put("app_name", WorkerService.stringPref(sp, "app_name", "{app_name}"));
                        state.put("favicon", WorkerService.stringPref(sp, "favicon", "logo.png"));
                        state.put("favicon_base64", WorkerService.stringPref(sp, "favicon_base64", ""));
                        state.put("permissions_done", true);
                    } catch (Exception ignored) {}
                    webView.evaluateJavascript("window.updateState(" + state.toString() + ")", null);
                }
            });
        }

        /**
         * Reports the live state of the worker so that the interface can show a lost
         * connection right away instead of waiting for the next registration.
         */
        @JavascriptInterface
        public String getWorkerStatus() {
            JSONObject status = new JSONObject();
            try {
                SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                String nodeId = WorkerService.stringPref(sp, "node_id", "");
                boolean paired = WorkerService.boolPref(sp, "is_paired", false) && !nodeId.isEmpty();
                boolean serviceRunning = false;
                ActivityManager am = (ActivityManager) getSystemService(Context.ACTIVITY_SERVICE);
                if (am != null) {
                    java.util.List<ActivityManager.RunningServiceInfo> services = am.getRunningServices(200);
                    for (ActivityManager.RunningServiceInfo info : services) {
                        if (info.service != null && info.service.getClassName().equals(WorkerService.class.getName())) {
                            serviceRunning = true;
                            break;
                        }
                    }
                }
                status.put("is_paired", paired);
                status.put("service_running", serviceRunning);
                status.put("node_id", nodeId);
                status.put("device_removed", WorkerService.stringPref(sp, "device_removed_message", "").length() > 0);
                status.put("device_removed_message", WorkerService.stringPref(sp, "device_removed_message", ""));
            } catch (Exception ignored) {}
            return status.toString();
        }

        @JavascriptInterface
        public String getInitialState() {
            if (latestWorkerState != null && latestWorkerState.length() > 0) {
                return latestWorkerState.toString();
            }
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            JSONObject state = new JSONObject();
            try {
                String nodeId = WorkerService.stringPref(sp, "node_id", "");
                String serverUrl = WorkerService.stringPref(sp, "server_url", "");
                String appName = WorkerService.stringPref(sp, "app_name", "{app_name}");
                String favicon = WorkerService.stringPref(sp, "favicon", "logo.png");
                String faviconBase64 = WorkerService.stringPref(sp, "favicon_base64", "");
                boolean isPaired = WorkerService.boolPref(sp, "is_paired", false) && !nodeId.isEmpty();

                state.put("is_paired", isPaired);
                state.put("node_id", nodeId);
                state.put("server_url", serverUrl);
                state.put("app_name", appName);
                state.put("favicon", favicon);
                state.put("favicon_base64", faviconBase64);
                state.put("battery_level", WorkerService.intPref(sp, "battery_percent", 100));
                state.put("battery_temp", (float) sp.getFloat("battery_temp", 31.0f));
                String assignedModel = WorkerService.stringPref(sp, "assigned_model", "qwen-2.5-0.5b-instruct");
                File mf = new File(getFilesDir(), assignedModel + ".gguf");
                boolean modelOnDisk = mf.exists() && mf.length() > 10 * 1024 * 1024;
                String defaultStatus = modelOnDisk ? "ready" : "downloading";
                int dMB = WorkerService.intPref(sp, "downloaded_mb", 0);
                int tMB = WorkerService.intPref(sp, "total_mb", 380);
                String defaultDesc = modelOnDisk ? (assignedModel + " hazır (Beklemede)") : (assignedModel + " indiriliyor... (" + dMB + " MB / " + tMB + " MB)");

                state.put("assigned_model", assignedModel);
                state.put("model_status", modelOnDisk ? WorkerService.stringPref(sp, "model_status", defaultStatus) : "downloading");
                state.put("model_status_desc", modelOnDisk ? WorkerService.stringPref(sp, "model_status_desc", defaultDesc) : defaultDesc);
                state.put("downloaded_mb", dMB);
                state.put("total_mb", tMB);
                state.put("download_progress", WorkerService.intPref(sp, "download_progress", 0));
                state.put("current_tps", 0.0);
                state.put("active_vram_model", WorkerService.stringPref(sp, "active_vram_model", ""));
                state.put("is_charging", WorkerService.boolPref(sp, "is_charging", false));
                state.put("downloading_model", WorkerService.stringPref(sp, "downloading_model", ""));
                String downloadError = WorkerService.stringPref(sp, "download_error", "");
                if (!downloadError.isEmpty()) {
                    state.put("download_error", downloadError);
                }
                String lastCrash = WorkerService.stringPref(sp, "last_crash", "");
                if (!lastCrash.isEmpty()) {
                    state.put("last_crash", lastCrash);
                }
                String removedMessage = WorkerService.stringPref(sp, "device_removed_message", "");
                if (!removedMessage.isEmpty()) {
                    state.put("device_removed", true);
                    state.put("device_removed_message", removedMessage);
                }
            } catch (Exception ignored) {}
            return state.toString();
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == REQ_QR_SCAN && resultCode == RESULT_OK && data != null) {
            String scanned = data.getStringExtra("qr_result");
            if (scanned != null && webView != null) {
                String js = "window.handleScannedQr(" + JSONObject.quote(scanned) + ")";
                webView.evaluateJavascript(js, null);
            }
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == REQ_CAMERA_PERM && grantResults.length > 0 && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
            Intent intent = new Intent(MainActivity.this, QRScanActivity.class);
            startActivityForResult(intent, REQ_QR_SCAN);
        } else if (requestCode == REQ_POST_NOTIFS) {
            if (webView != null) webView.evaluateJavascript("if (window.refreshPermissions) window.refreshPermissions();", null);
        }
    }
}
'''
with open(os.path.join(src_dir, "MainActivity.java"), "w", encoding="utf-8") as f:
    f.write(main_activity_content.replace("{app_name}", app_name))

# QRScanActivity.java
qr_scan_activity_content = '''package org.byog.worker;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.graphics.Color;
import android.hardware.Camera;
import android.os.Bundle;
import android.os.Vibrator;
import android.view.Gravity;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.widget.FrameLayout;
import android.widget.TextView;

import com.google.zxing.BinaryBitmap;
import com.google.zxing.MultiFormatReader;
import com.google.zxing.PlanarYUVLuminanceSource;
import com.google.zxing.Result;
import com.google.zxing.common.HybridBinarizer;

public class QRScanActivity extends Activity implements SurfaceHolder.Callback, Camera.PreviewCallback {
    private Camera camera;
    private SurfaceView surfaceView;
    private SurfaceHolder surfaceHolder;
    private boolean isScanned = false;
    private MultiFormatReader multiFormatReader;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        multiFormatReader = new MultiFormatReader();

        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.BLACK);

        surfaceView = new SurfaceView(this);
        surfaceHolder = surfaceView.getHolder();
        surfaceHolder.addCallback(this);
        root.addView(surfaceView, new FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));

        TextView tvHint = new TextView(this);
        tvHint.setText("Web ekranındaki QR kodu kareye hizalayın");
        tvHint.setTextColor(Color.WHITE);
        tvHint.setTextSize(15f);
        tvHint.setGravity(Gravity.CENTER);
        tvHint.setBackgroundColor(Color.parseColor("#B3020817"));
        tvHint.setPadding(24, 32, 24, 32);
        FrameLayout.LayoutParams hintParams = new FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT);
        hintParams.gravity = Gravity.BOTTOM;
        root.addView(tvHint, hintParams);

        setContentView(root);
    }

    @Override
    public void surfaceCreated(SurfaceHolder holder) {
        try {
            camera = Camera.open();
            Camera.Parameters params = camera.getParameters();
            params.setFocusMode(Camera.Parameters.FOCUS_MODE_CONTINUOUS_PICTURE);
            camera.setParameters(params);
            camera.setDisplayOrientation(90);
            camera.setPreviewDisplay(holder);
            camera.setOneShotPreviewCallback(this);
            camera.startPreview();
        } catch (Exception e) {
            if (camera != null) {
                camera.release();
                camera = null;
            }
        }
    }

    @Override
    public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {}

    @Override
    public void surfaceDestroyed(SurfaceHolder holder) {
        if (camera != null) {
            camera.stopPreview();
            camera.setPreviewCallback(null);
            camera.release();
            camera = null;
        }
    }

    @Override
    public void onPreviewFrame(byte[] data, Camera cam) {
        if (isScanned || cam == null) return;
        try {
            Camera.Size size = cam.getParameters().getPreviewSize();
            PlanarYUVLuminanceSource source = new PlanarYUVLuminanceSource(
                    data, size.width, size.height, 0, 0, size.width, size.height, false);
            BinaryBitmap bitmap = new BinaryBitmap(new HybridBinarizer(source));
            Result result = multiFormatReader.decodeWithState(bitmap);
            if (result != null && result.getText() != null) {
                isScanned = true;
                vibrate();
                Intent resIntent = new Intent();
                resIntent.putExtra("qr_result", result.getText());
                setResult(RESULT_OK, resIntent);
                finish();
                return;
            }
        } catch (Exception ignored) {
        } finally {
            if (!isScanned && camera != null) {
                camera.setOneShotPreviewCallback(this);
            }
        }
    }

    private void vibrate() {
        try {
            Vibrator v = (Vibrator) getSystemService(Context.VIBRATOR_SERVICE);
            if (v != null) {
                v.vibrate(80);
            }
        } catch (Exception ignored) {}
    }
}
'''
with open(os.path.join(src_dir, "QRScanActivity.java"), "w", encoding="utf-8") as f:
    f.write(qr_scan_activity_content)

# WorkerService.java (Real hardware specs, assigned model downloading & running status simulation, foreground notification)
worker_service_template_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'WorkerService.java.template')
with open(worker_service_template_path, 'r', encoding='utf-8') as f:
    worker_service_content = f.read().replace('{app_name}', app_name)
with open(os.path.join(src_dir, 'WorkerService.java'), 'w', encoding='utf-8') as f:
    f.write(worker_service_content)

# LocalNeuralEngine.java (Pure Java 8 On-Device Autoregressive Transformer Engine)
local_neural_engine_template_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'LocalNeuralEngine.java.template')
with open(local_neural_engine_template_path, 'r', encoding='utf-8') as f:
    local_neural_engine_content = f.read()
with open(os.path.join(src_dir, 'LocalNeuralEngine.java'), 'w', encoding='utf-8') as f:
    f.write(local_neural_engine_content)

# BootReceiver.java
boot_receiver_content = '''package org.byog.worker;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.os.Build;

public class BootReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        String action = intent != null ? intent.getAction() : "";
        if (Intent.ACTION_BOOT_COMPLETED.equals(action) ||
            "android.intent.action.QUICKBOOT_POWERON".equals(action) ||
            Intent.ACTION_MY_PACKAGE_REPLACED.equals(action) ||
            Intent.ACTION_USER_PRESENT.equals(action) ||
            Intent.ACTION_POWER_CONNECTED.equals(action) ||
            "android.net.conn.CONNECTIVITY_CHANGE".equals(action) ||
            "org.byog.worker.WATCHDOG_CHECK".equals(action)) {
            SharedPreferences sp = context.getSharedPreferences("byog_prefs", Context.MODE_PRIVATE);
            boolean isPaired = WorkerService.boolPref(sp, "is_paired", false);
            String nodeId = WorkerService.stringPref(sp, "node_id", "");
            if (isPaired && !nodeId.isEmpty()) {
                try {
                    Intent sIntent = new Intent(context, WorkerService.class);
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        context.startForegroundService(sIntent);
                    } else {
                        context.startService(sIntent);
                    }
                } catch (Throwable ignored) {
                    // Android 12+ blocks foreground service starts from the background for
                    // some broadcasts. The worker is restarted by the next watchdog tick,
                    // so the failure must never take the application down.
                    android.util.Log.w("BYOG_BOOT", "Worker could not be started: " + ignored.getMessage());
                }
            }
        }
    }
}
'''
with open(os.path.join(src_dir, "BootReceiver.java"), "w", encoding="utf-8") as f:
    f.write(boot_receiver_content)

# GenericFileProvider.java
generic_file_provider_content = '''package org.byog.worker;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import java.io.File;
import java.io.FileNotFoundException;

public class GenericFileProvider extends ContentProvider {
    @Override
    public boolean onCreate() {
        return true;
    }

    @Override
    public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        File file = new File(getContext().getCacheDir(), uri.getLastPathSegment());
        if (!file.exists()) {
            file = new File(getContext().getFilesDir(), uri.getLastPathSegment());
        }
        if (file.exists()) {
            return ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY);
        }
        throw new FileNotFoundException(uri.getPath());
    }

    @Override
    public String getType(Uri uri) {
        return "application/vnd.android.package-archive";
    }

    @Override
    public Cursor query(Uri uri, String[] projection, String selection, String[] selectionArgs, String sortOrder) {
        return null;
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) {
        return null;
    }

    @Override
    public int delete(Uri uri, String selection, String[] selectionArgs) {
        return 0;
    }

    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] selectionArgs) {
        return 0;
    }
}
'''
with open(os.path.join(src_dir, "GenericFileProvider.java"), "w", encoding="utf-8") as f:
    f.write(generic_file_provider_content)

print("Java sources and assets written successfully.")

# 4. AAPT2 Compile
compiled_res = os.path.join(WORKDIR, "compiled.flata")
subprocess.check_call([
    AAPT2, "compile",
    "--dir", res_dir,
    "-o", compiled_res
])

# 5. AAPT2 Link
unaligned_apk = os.path.join(WORKDIR, "unaligned.apk")
subprocess.check_call([
    AAPT2, "link",
    "-I", PLATFORM_JAR,
    "--manifest", manifest_path,
    "-A", assets_dir,
    "-o", unaligned_apk,
    compiled_res,
    "--auto-add-overlay"
])

# 6. Compile Java
classes_dir = os.path.join(WORKDIR, "classes")
os.makedirs(classes_dir, exist_ok=True)
java_files = [
    os.path.join(src_dir, "ApkConfigReader.java"),
    os.path.join(src_dir, "LocalNeuralEngine.java"),
    os.path.join(src_dir, "MainActivity.java"),
    os.path.join(src_dir, "QRScanActivity.java"),
    os.path.join(src_dir, "WorkerService.java"),
    os.path.join(src_dir, "BootReceiver.java"),
    os.path.join(src_dir, "GenericFileProvider.java")
]

classpath = f"{PLATFORM_JAR};{ZXING_JAR};{LLAMA_JAR}" if os.path.exists(LLAMA_JAR) else f"{PLATFORM_JAR};{ZXING_JAR}"
subprocess.check_call([
    "javac",
    "-encoding", "UTF-8",
    "-cp", classpath,
    "-d", classes_dir,
    "-g:none",
    "--release", "8"
] + java_files)

# 7. D8: Convert classes and jar dependencies to classes.dex
class_files = []
for root, _, files in os.walk(classes_dir):
    for f in files:
        if f.endswith(".class"):
            class_files.append(os.path.join(root, f))

d8_cmd = [
    D8,
    "--output", WORKDIR,
    "--min-api", "24",
    "--lib", PLATFORM_JAR,
    ZXING_JAR
]
if os.path.exists(LLAMA_JAR):
    d8_cmd.append(LLAMA_JAR)
d8_cmd += class_files

subprocess.check_call(d8_cmd, shell=True)

dex_file = os.path.join(WORKDIR, "classes.dex")

# 8. Add classes.dex and native libraries to unaligned APK
with zipfile.ZipFile(unaligned_apk, "a", compression=zipfile.ZIP_DEFLATED) as zf:
    zf.write(dex_file, "classes.dex")
    if os.path.exists(LLAMA_JAR):
        with zipfile.ZipFile(LLAMA_JAR, "r") as lzf:
            so_name = "de/kherud/llama/Linux-Android/aarch64/libjllama.so"
            if so_name in lzf.namelist():
                so_data = lzf.read(so_name)
                zinfo = zipfile.ZipInfo("lib/arm64-v8a/libjllama.so")
                zinfo.compress_type = zipfile.ZIP_STORED
                zf.writestr(zinfo, so_data)

# 9. Zipalign
# The uncompressed native libraries are aligned to a sixteen kilobyte page boundary, the
# platform rejects the installation of an APK with misaligned native libraries.
aligned_apk = os.path.join(WORKDIR, "aligned.apk")
subprocess.check_call([ZIPALIGN, "-P", "16", "-f", "-v", "4", unaligned_apk, aligned_apk])

# 10. Signing key
#
# The key has to stay the same for every build, otherwise the installed application
# cannot be updated ("App not installed" / INSTALL_FAILED_UPDATE_INCOMPATIBLE). The
# keystore of the repository is therefore the single source of truth: it signs the APK
# and the private key is exported from it for the server, which signs the rebranded
# APK with the same key.
RELEASE_KEYSTORE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "release.keystore")
SIGNING_KEY_PEM = os.path.join(os.path.dirname(os.path.abspath(__file__)), "apk_signing_key.pem")
KEYSTORE_PASSWORD = "android"
KEYSTORE_ALIAS = "androidkey"

if not os.path.exists(RELEASE_KEYSTORE):
    print("The release keystore is missing, a new one is created: " + RELEASE_KEYSTORE)
    subprocess.check_call([
        KEYTOOL,
        "-genkeypair",
        "-keystore", RELEASE_KEYSTORE,
        "-storepass", KEYSTORE_PASSWORD,
        "-keypass", KEYSTORE_PASSWORD,
        "-alias", KEYSTORE_ALIAS,
        "-keyalg", "RSA",
        "-keysize", "2048",
        "-validity", "10000",
        "-dname", "CN=BYOG, OU=Dev, O=BYOG, L=City, S=State, C=US"
    ])

# The private key is exported for the server, it has to match the keystore.
export_dir = os.path.join(WORKDIR, "signing")
os.makedirs(export_dir, exist_ok=True)
export_class = os.path.join(export_dir, "ExportSigningKey.class")
subprocess.check_call([
    "javac",
    "-encoding", "UTF-8",
    "-d", export_dir,
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "ExportSigningKey.java")
])
subprocess.check_call([
    "java",
    "-cp", export_dir,
    "ExportSigningKey",
    RELEASE_KEYSTORE,
    KEYSTORE_PASSWORD,
    KEYSTORE_ALIAS,
    SIGNING_KEY_PEM,
])
print(f"Signing key exported for the server: {SIGNING_KEY_PEM}")

# 11. Apksigner sign (v1 + v2 + v3)
subprocess.check_call([
    APKSIGNER, "sign",
    "--ks", RELEASE_KEYSTORE,
    "--ks-pass", "pass:" + KEYSTORE_PASSWORD,
    "--key-pass", "pass:" + KEYSTORE_PASSWORD,
    "--out", OUTPUT_APK,
    aligned_apk
])

# 12. Verify APK
subprocess.check_call([APKSIGNER, "verify", OUTPUT_APK])

print(f"SUCCESS: Generated signed, valid Android APK at: {OUTPUT_APK} (Size: {os.path.getsize(OUTPUT_APK)} bytes)")
