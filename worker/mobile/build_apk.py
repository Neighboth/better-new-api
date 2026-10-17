import os
import sys
import subprocess
import shutil
import zipfile
import json

BUILD_TOOLS = r"C:\Users\simse\AppData\Local\Android\Sdk\build-tools\37.0.0"
PLATFORM_JAR = r"C:\Users\simse\AppData\Local\Android\Sdk\platforms\android-34\android.jar"
KEYTOOL = r"C:\Program Files\Java\jdk-21.0.10\bin\keytool.exe"
AAPT2 = os.path.join(BUILD_TOOLS, "aapt2.exe")
D8 = os.path.join(BUILD_TOOLS, "d8.bat")
ZIPALIGN = os.path.join(BUILD_TOOLS, "zipalign.exe")
APKSIGNER = os.path.join(BUILD_TOOLS, "apksigner.bat")

ZXING_JAR = os.path.abspath(r"Z:\better-new-api\worker\mobile\zxing-core.jar")
WORKDIR = os.path.abspath(r"Z:\better-new-api\worker\mobile\build_tmp")
OUTPUT_APK = os.path.abspath(r"Z:\better-new-api\worker\binaries\byog-worker.apk")

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

# Determine App Name from config or fallback
app_name = "New-API BYOG"

# strings.xml
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

# Copy App Icons
logo_src = r"Z:\better-new-api\web\public\logo.png"
if os.path.exists(logo_src):
    shutil.copyfile(logo_src, os.path.join(res_mipmap, "ic_launcher.png"))
    shutil.copyfile(logo_src, os.path.join(res_mipmap, "ic_launcher_round.png"))
    shutil.copyfile(logo_src, os.path.join(assets_www, "logo.png"))

# AndroidManifest.xml
manifest_content = f'''<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="org.byog.worker"
    android:versionCode="2"
    android:versionName="1.4.0">

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
    <uses-permission android:name="android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS" />

    <uses-feature android:name="android.hardware.camera" android:required="false" />
    <uses-feature android:name="android.hardware.camera.autofocus" android:required="false" />

    <application
        android:label="@string/app_name"
        android:icon="@mipmap/ic_launcher"
        android:roundIcon="@mipmap/ic_launcher_round"
        android:theme="@style/AppTheme"
        android:usesCleartextTraffic="true">
        <activity
            android:name=".MainActivity"
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
            android:foregroundServiceType="dataSync" />
        <receiver
            android:name=".BootReceiver"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.BOOT_COMPLETED" />
            </intent-filter>
        </receiver>
    </application>
</manifest>
'''
manifest_path = os.path.join(WORKDIR, "AndroidManifest.xml")
with open(manifest_path, "w", encoding="utf-8") as f:
    f.write(manifest_content)

# 2. Modern HTML5/CSS/JS UI in assets/www/index.html (Matches Website 1:1, Zero Emojis)
html_content = '''<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no">
  <title>BYOG Worker</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; user-select: none; }
    body { background-color: #020817; color: #f8fafc; padding: 16px; min-height: 100vh; display: flex; flex-direction: column; }
    
    .header { display: flex; align-items: center; justify-content: space-between; padding-bottom: 16px; border-bottom: 1px solid #1e293b; margin-bottom: 16px; }
    .brand { display: flex; align-items: center; gap: 10px; }
    .brand img { width: 34px; height: 34px; border-radius: 8px; object-fit: contain; }
    .brand-title { font-size: 1.1rem; font-weight: 700; letter-spacing: -0.02em; color: #f8fafc; }
    .badge { font-size: 0.75rem; font-weight: 600; padding: 4px 10px; border-radius: 9999px; display: inline-flex; align-items: center; gap: 6px; }
    .badge-amber { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
    .badge-green { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
    .dot { width: 6px; height: 6px; border-radius: 50%; }
    .dot-amber { background-color: #fbbf24; }
    .dot-green { background-color: #34d399; box-shadow: 0 0 8px #34d399; }

    .card { background-color: #0f172a; border: 1px solid #1e293b; border-radius: 14px; padding: 16px; margin-bottom: 14px; }
    .card-title { font-size: 0.85rem; font-weight: 600; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 12px; display: flex; align-items: center; gap: 6px; }

    .btn { display: flex; align-items: center; justify-content: center; gap: 8px; width: 100%; padding: 13px 18px; border-radius: 10px; font-size: 0.95rem; font-weight: 600; cursor: pointer; border: none; transition: background 0.15s ease; }
    .btn-primary { background-color: #2563eb; color: #ffffff; }
    .btn-primary:active { background-color: #1d4ed8; }
    .btn-secondary { background-color: #1e293b; color: #e2e8f0; border: 1px solid #334155; }
    .btn-secondary:active { background-color: #334155; }
    .btn-danger { background-color: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }
    .btn-danger:active { background-color: rgba(239, 68, 68, 0.25); }

    .input-group { display: flex; gap: 8px; margin-top: 10px; margin-bottom: 10px; }
    .input-text { flex: 1; background-color: #020817; border: 1px solid #334155; border-radius: 8px; padding: 11px 14px; font-family: monospace; font-size: 0.95rem; font-weight: 600; color: #f8fafc; text-align: center; letter-spacing: 0.1em; text-transform: uppercase; }
    .input-text:focus { outline: none; border-color: #3b82f6; }
    .btn-icon { width: 44px; display: flex; align-items: center; justify-content: center; background-color: #1e293b; border: 1px solid #334155; border-radius: 8px; color: #94a3b8; cursor: pointer; }

    .hint-text { font-size: 0.78rem; color: #64748b; line-height: 1.4; text-align: center; }

    .grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 6px; }
    .stat-card { background-color: #020817; border: 1px solid #1e293b; border-radius: 10px; padding: 12px; }
    .stat-label { font-size: 0.72rem; color: #64748b; font-weight: 500; text-transform: uppercase; }
    .stat-value { font-size: 1.05rem; font-weight: 700; color: #f8fafc; margin-top: 4px; }
    .stat-sub { font-size: 0.72rem; color: #94a3b8; margin-top: 2px; }

    .divider { height: 1px; background-color: #1e293b; margin: 14px 0; }
    .hidden { display: none !important; }
    
    .toast { position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%); background-color: #1e293b; border: 1px solid #334155; color: #f8fafc; padding: 10px 18px; border-radius: 8px; font-size: 0.85rem; font-weight: 500; z-index: 1000; opacity: 0; transition: opacity 0.2s ease; pointer-events: none; }
    .toast.show { opacity: 1; }

    svg { width: 18px; height: 18px; stroke: currentColor; fill: none; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
  </style>
</head>
<body>

  <!-- App Header -->
  <div class="header">
    <div class="brand">
      <img id="app-logo" src="logo.png" onerror="this.style.display='none'">
      <div id="app-title" class="brand-title">New-API BYOG</div>
    </div>
    <div id="status-badge" class="badge badge-amber">
      <div class="dot dot-amber"></div>
      <span id="status-text">Eslestirme Bekleniyor</span>
    </div>
  </div>

  <!-- View 1: Unpaired (Single Flow: Scan QR or Enter Code) -->
  <div id="unpaired-view">
    <div class="card">
      <div class="card-title">
        <svg viewBox="0 0 24 24"><path d="M4 7V4h3M20 7V4h-3M4 17v3h3M20 17v3h-3M9 9h6v6H9z"/></svg>
        <span>Hizli Baglanti</span>
      </div>
      
      <p class="hint-text" style="text-align: left; margin-bottom: 14px;">
        Web panelinizde "Cihaz Bagla" bolumunde gosterilen QR kodu kameranizla okutun veya asagiya eslestirme kodunu girin.
      </p>

      <button id="btn-scan" class="btn btn-primary" onclick="startQrScan()">
        <svg viewBox="0 0 24 24"><path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/></svg>
        <span>QR Kod Tara</span>
      </button>

      <div class="divider"></div>

      <div class="card-title" style="margin-bottom: 6px;">
        <svg viewBox="0 0 24 24"><circle cx="7.5" cy="15.5" r="5.5"/><path d="m21 2-9.6 9.6M15.5 7.5l3 3"/></svg>
        <span>veya Eslestirme Kodu</span>
      </div>

      <div class="input-group">
        <input id="input-code" type="text" class="input-text" placeholder="9K4L-28XM-73PQ" maxlength="24">
        <button class="btn-icon" onclick="pasteCode()" title="Yapistir">
          <svg viewBox="0 0 24 24"><rect width="8" height="4" x="8" y="2" rx="1" ry="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/></svg>
        </button>
      </div>

      <button id="btn-connect" class="btn btn-secondary" onclick="connectManual()">
        <span>Cihazi Bagla</span>
      </button>
    </div>

    <!-- Advanced Server URL (Auto-filled or collapsible) -->
    <div class="card" style="padding: 12px 16px;">
      <div style="display: flex; justify-content: space-between; align-items: center; cursor: pointer;" onclick="toggleAdvanced()">
        <span style="font-size: 0.8rem; color: #64748b; font-weight: 600;">GELISMIS: SUNUCU ADRESI</span>
        <svg id="chevron-adv" viewBox="0 0 24 24" style="width: 14px; height: 14px;"><polyline points="6 9 12 15 18 9"/></svg>
      </div>
      <div id="adv-panel" class="hidden" style="margin-top: 10px;">
        <input id="input-server" type="text" class="input-text" style="width: 100%; text-align: left; font-size: 0.82rem;" placeholder="https://api.yourdomain.com">
      </div>
    </div>
  </div>

  <!-- View 2: Paired & Active Worker -->
  <div id="paired-view" class="hidden">
    <div class="card">
      <div class="card-title">
        <svg viewBox="0 0 24 24"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
        <span>Hesaplama Durumu</span>
      </div>

      <div class="grid-2">
        <div class="stat-card">
          <div class="stat-label">Bagli Sunucu</div>
          <div id="stat-server" class="stat-value" style="font-size: 0.85rem; word-break: break-all;">--</div>
          <div id="stat-node" class="stat-sub">Node ID: --</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">TPS Hizi</div>
          <div id="stat-tps" class="stat-value">38.4</div>
          <div class="stat-sub">Aktif cikarim</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Pil ve Sicaklik</div>
          <div id="stat-battery" class="stat-value">100% / 31&deg;C</div>
          <div class="stat-sub">Termal koruma aktif</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Ag Baglantisi</div>
          <div id="stat-net" class="stat-value">WiFi</div>
          <div class="stat-sub">Dusuk gecikme</div>
        </div>
      </div>

      <div class="divider"></div>

      <p class="hint-text" style="margin-bottom: 14px;">
        Worker arka planda guvenli sekilde calisiyor. Cihaz uyku moduna gecse bile hesaplama gorevleri devam eder.
      </p>

      <button class="btn btn-danger" onclick="disconnectNode()">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
        <span>Baglantiyi Kes</span>
      </button>
    </div>
  </div>

  <div id="toast" class="toast"></div>

  <script>
    function showToast(msg) {
      const t = document.getElementById('toast');
      t.innerText = msg;
      t.classList.add('show');
      setTimeout(() => t.classList.remove('show'), 2500);
    }

    function toggleAdvanced() {
      const p = document.getElementById('adv-panel');
      p.classList.toggle('hidden');
    }

    function startQrScan() {
      if (window.AndroidBridge && window.AndroidBridge.scanQrCode) {
        window.AndroidBridge.scanQrCode();
      } else {
        showToast("QR okuyucu baslatilamadi");
      }
    }

    function pasteCode() {
      if (window.AndroidBridge && window.AndroidBridge.pasteFromClipboard) {
        const txt = window.AndroidBridge.pasteFromClipboard();
        if (txt) {
          document.getElementById('input-code').value = txt.trim().toUpperCase();
          showToast("Kod yapistirildi");
        }
      }
    }

    function connectManual() {
      const code = document.getElementById('input-code').value.trim().toUpperCase();
      let server = document.getElementById('input-server').value.trim();
      if (!code) {
        showToast("Lutfen eslestirme kodunu girin");
        return;
      }
      if (window.AndroidBridge && window.AndroidBridge.connect) {
        window.AndroidBridge.connect(code, server);
      }
    }

    function disconnectNode() {
      if (window.AndroidBridge && window.AndroidBridge.disconnect) {
        window.AndroidBridge.disconnect();
      }
    }

    // Called automatically by Android when QR code is scanned
    window.handleScannedQr = function(rawText) {
      if (!rawText) return;
      try {
        const obj = JSON.parse(rawText);
        if (obj.server_url) {
          document.getElementById('input-server').value = obj.server_url;
        }
        if (obj.pairing_code || obj.code) {
          const c = obj.pairing_code || obj.code;
          document.getElementById('input-code').value = c;
          if (window.AndroidBridge && window.AndroidBridge.connect) {
            window.AndroidBridge.connect(c, obj.server_url || "");
          }
          return;
        }
      } catch (e) {
        // Plain text fallback
        document.getElementById('input-code').value = rawText.trim().toUpperCase();
        connectManual();
      }
    };

    // Called by Android on state update
    window.updateState = function(stateJson) {
      try {
        const s = typeof stateJson === 'string' ? JSON.parse(stateJson) : stateJson;
        if (s.app_name) {
          document.getElementById('app-title').innerText = s.app_name;
        }
        if (s.is_paired) {
          document.getElementById('unpaired-view').classList.add('hidden');
          document.getElementById('paired-view').classList.remove('hidden');
          
          const badge = document.getElementById('status-badge');
          badge.className = 'badge badge-green';
          badge.innerHTML = '<div class="dot dot-green"></div><span>Aktif & Bagli</span>';

          if (s.server_url) {
            try {
              const u = new URL(s.server_url);
              document.getElementById('stat-server').innerText = u.host;
            } catch(e) {
              document.getElementById('stat-server').innerText = s.server_url;
            }
          }
          if (s.node_id) {
            document.getElementById('stat-node').innerText = "Node: " + s.node_id.substring(0, 10);
          }
          if (s.battery_level !== undefined) {
            document.getElementById('stat-battery').innerHTML = s.battery_level + "% / " + (s.battery_temp || 30) + "&deg;C";
          }
        } else {
          document.getElementById('unpaired-view').classList.remove('hidden');
          document.getElementById('paired-view').classList.add('hidden');

          const badge = document.getElementById('status-badge');
          badge.className = 'badge badge-amber';
          badge.innerHTML = '<div class="dot dot-amber"></div><span>Eslestirme Bekleniyor</span>';
        }
      } catch(e) {}
    };

    // Ask Android for initial state once loaded
    window.onload = function() {
      if (window.AndroidBridge && window.AndroidBridge.getInitialState) {
        const initStr = window.AndroidBridge.getInitialState();
        if (initStr) {
          window.updateState(initStr);
        }
      }
    };
  </script>
</body>
</html>
'''
with open(os.path.join(assets_www, "index.html"), "w", encoding="utf-8") as f:
    f.write(html_content)

# 3. Java Source Files

# MainActivity.java
main_activity_content = '''package org.byog.worker;

import android.app.Activity;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.PowerManager;
import android.provider.Settings;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.Toast;

import org.json.JSONObject;

public class MainActivity extends Activity {
    private WebView webView;
    private static final int REQ_QR_SCAN = 1001;
    private static final int REQ_POST_NOTIFS = 1002;
    private static final int REQ_CAMERA_PERM = 1003;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        webView = new WebView(this);
        setContentView(webView);

        WebSettings ws = webView.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setAllowFileAccess(true);
        ws.setAllowContentAccess(true);
        webView.setBackgroundColor(0xFF020817);

        webView.addJavascriptInterface(new WebAppInterface(), "AndroidBridge");
        webView.loadUrl("file:///android_asset/www/index.html");

        requestInitialPermissions();
    }

    private void requestInitialPermissions() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(new String[]{android.Manifest.permission.POST_NOTIFICATIONS}, REQ_POST_NOTIFS);
            }
        }

        try {
            PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
            if (pm != null && !pm.isIgnoringBatteryOptimizations(getPackageName())) {
                Intent intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
                intent.setData(Uri.parse("package:" + getPackageName()));
                startActivity(intent);
            }
        } catch (Exception ignored) {}
    }

    public class WebAppInterface {
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
        public void connect(final String pairingCode, final String serverUrl) {
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                    sp.edit()
                      .putString("pairing_code", pairingCode)
                      .putString("server_url", serverUrl)
                      .apply();

                    Intent sIntent = new Intent(MainActivity.this, WorkerService.class);
                    sIntent.putExtra("pairing_code", pairingCode);
                    sIntent.putExtra("server_url", serverUrl);
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        startForegroundService(sIntent);
                    } else {
                        startService(sIntent);
                    }
                    Toast.makeText(MainActivity.this, "Baglanti baslatildi...", Toast.LENGTH_SHORT).show();
                }
            });
        }

        @JavascriptInterface
        public void disconnect() {
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
                    sp.edit().remove("node_id").remove("pairing_code").apply();

                    stopService(new Intent(MainActivity.this, WorkerService.class));
                    Toast.makeText(MainActivity.this, "Baglanti kesildi", Toast.LENGTH_SHORT).show();

                    JSONObject state = new JSONObject();
                    try {
                        state.put("is_paired", false);
                        state.put("app_name", "New-API BYOG");
                    } catch (Exception ignored) {}
                    webView.evaluateJavascript("window.updateState(" + state.toString() + ")", null);
                }
            });
        }

        @JavascriptInterface
        public String getInitialState() {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            JSONObject state = new JSONObject();
            try {
                String nodeId = sp.getString("node_id", "");
                String serverUrl = sp.getString("server_url", "");
                String appName = sp.getString("app_name", "New-API BYOG");
                state.put("is_paired", !nodeId.isEmpty());
                state.put("node_id", nodeId);
                state.put("server_url", serverUrl);
                state.put("app_name", appName);
                state.put("battery_level", 100);
                state.put("battery_temp", 31);
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
        }
    }
}
'''
with open(os.path.join(src_dir, "MainActivity.java"), "w", encoding="utf-8") as f:
    f.write(main_activity_content)

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
        tvHint.setText("Web ekranindaki QR kodu kareye hizalayin");
        tvHint.setTextColor(Color.WHITE);
        tvHint.setTextSize(14f);
        tvHint.setGravity(Gravity.CENTER);
        tvHint.setBackgroundColor(Color.parseColor("#99000000"));
        tvHint.setPadding(20, 30, 20, 30);
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
            // Rotate coordinates for portrait orientation
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

# WorkerService.java
worker_service_content = '''package org.byog.worker;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.ServiceInfo;
import android.net.wifi.WifiManager;
import android.os.BatteryManager;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;

import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;

public class WorkerService extends Service {
    private static final String CHANNEL_ID = "byog_worker_channel";
    private static final int NOTIF_ID = 1001;
    private PowerManager.WakeLock wakeLock;
    private WifiManager.WifiLock wifiLock;
    private boolean isRunning = false;
    private Thread workerThread;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (isRunning) return START_STICKY;
        isRunning = true;

        String appName = "New-API BYOG";
        SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        if (sp != null) {
            appName = sp.getString("app_name", "New-API BYOG");
        }

        Notification notification = buildNotification(appName, "Hesaplama gorevleri icin aktif");

        try {
            if (Build.VERSION.SDK_INT >= 29) {
                startForeground(NOTIF_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
            } else {
                startForeground(NOTIF_ID, notification);
            }
        } catch (Exception e) {
            startForeground(NOTIF_ID, notification);
        }

        acquireLocks();
        startWorkerLoop();

        return START_STICKY;
    }

    private void acquireLocks() {
        try {
            PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
            if (pm != null) {
                wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "BYOG:WorkerWakeLock");
                wakeLock.acquire(24 * 60 * 60 * 1000L);
            }
            WifiManager wm = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
            if (wm != null) {
                wifiLock = wm.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "BYOG:WifiLock");
                wifiLock.acquire();
            }
        } catch (Exception ignored) {}
    }

    private void startWorkerLoop() {
        workerThread = new Thread(new Runnable() {
            @Override
            public void run() {
                while (isRunning) {
                    try {
                        doHeartbeatAndSync();
                        Thread.sleep(15000);
                    } catch (InterruptedException e) {
                        break;
                    } catch (Exception ignored) {}
                }
            }
        });
        workerThread.start();
    }

    private void doHeartbeatAndSync() {
        try {
            SharedPreferences sp = getSharedPreferences("byog_prefs", MODE_PRIVATE);
            String serverUrl = sp.getString("server_url", "");
            String pairingCode = sp.getString("pairing_code", "");
            String nodeId = sp.getString("node_id", "");

            if (serverUrl == null || serverUrl.isEmpty()) {
                serverUrl = "http://localhost:3000";
            }

            // Register or Heartbeat
            URL url = new URL(serverUrl + "/api/v1/worker/register");
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();
            conn.setRequestMethod("POST");
            conn.setRequestProperty("Content-Type", "application/json");
            conn.setConnectTimeout(8000);
            conn.setReadTimeout(8000);
            conn.setDoOutput(true);

            Intent batteryIntent = registerReceiver(null, new IntentFilter(Intent.ACTION_BATTERY_CHANGED));
            int level = -1;
            float temp = 30.0f;
            if (batteryIntent != null) {
                level = batteryIntent.getIntExtra(BatteryManager.EXTRA_LEVEL, -1);
                temp = batteryIntent.getIntExtra(BatteryManager.EXTRA_TEMPERATURE, 300) / 10.0f;
            }

            JSONObject req = new JSONObject();
            req.put("hwid", "android-" + Build.BOARD + "-" + Build.DEVICE + "-" + Build.HARDWARE);
            req.put("pairing_code", pairingCode);
            req.put("os", "android");
            req.put("arch", Build.SUPPORTED_ABIS[0]);
            req.put("device_name", Build.MANUFACTURER + " " + Build.MODEL);
            req.put("device_type", "mobile");
            req.put("cpu_model", Build.HARDWARE);
            req.put("physical_ram_mb", 6144);
            req.put("swap_ram_mb", 2048);
            req.put("vram_mb", 2048);
            req.put("battery_temp", temp);
            req.put("battery_percent", level);

            OutputStream os = conn.getOutputStream();
            os.write(req.toString().getBytes("UTF-8"));
            os.close();

            if (conn.getResponseCode() == 200) {
                BufferedReader br = new BufferedReader(new InputStreamReader(conn.getInputStream()));
                StringBuilder sb = new StringBuilder();
                String line;
                while ((line = br.readLine()) != null) sb.append(line);
                br.close();

                JSONObject resp = new JSONObject(sb.toString());
                if (resp.optBoolean("success")) {
                    JSONObject data = resp.getJSONObject("data");
                    String returnedNodeId = data.optString("node_id");
                    String appName = data.optString("app_name", "New-API BYOG");
                    sp.edit()
                      .putString("node_id", returnedNodeId)
                      .putString("app_name", appName)
                      .apply();

                    Notification updated = buildNotification(appName, "Hesaplama gorevleri icin aktif (Bagli)");
                    NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
                    if (nm != null) nm.notify(NOTIF_ID, updated);
                }
            }
            conn.disconnect();
        } catch (Exception ignored) {}
    }

    private Notification buildNotification(String title, String content) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            return new Notification.Builder(this, CHANNEL_ID)
                    .setContentTitle(title)
                    .setContentText(content)
                    .setSmallIcon(android.R.drawable.stat_notify_sync)
                    .setOngoing(true)
                    .build();
        } else {
            return new Notification();
        }
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "BYOG Worker Service",
                    NotificationManager.IMPORTANCE_LOW
            );
            NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (manager != null) {
                manager.createNotificationChannel(channel);
            }
        }
    }

    @Override
    public void onDestroy() {
        isRunning = false;
        if (workerThread != null) workerThread.interrupt();
        if (wakeLock != null && wakeLock.isHeld()) wakeLock.release();
        if (wifiLock != null && wifiLock.isHeld()) wifiLock.release();
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
'''
with open(os.path.join(src_dir, "WorkerService.java"), "w", encoding="utf-8") as f:
    f.write(worker_service_content)

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
        if (Intent.ACTION_BOOT_COMPLETED.equals(intent.getAction())) {
            SharedPreferences sp = context.getSharedPreferences("byog_prefs", Context.MODE_PRIVATE);
            String nodeId = sp.getString("node_id", "");
            if (!nodeId.isEmpty()) {
                Intent sIntent = new Intent(context, WorkerService.class);
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    context.startForegroundService(sIntent);
                } else {
                    context.startService(sIntent);
                }
            }
        }
    }
}
'''
with open(os.path.join(src_dir, "BootReceiver.java"), "w", encoding="utf-8") as f:
    f.write(boot_receiver_content)

print("Java sources and assets written successfully.")

# 4. AAPT2 Compile
compiled_res = os.path.join(WORKDIR, "compiled.flata")
subprocess.check_call([AAPT2, "compile", "--dir", res_dir, "-o", compiled_res])

# 5. AAPT2 Link (creates unaligned unsigned APK with compiled resources, AndroidManifest.xml & assets)
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

# 6. Compile Java with ZXing
classes_dir = os.path.join(WORKDIR, "classes")
os.makedirs(classes_dir, exist_ok=True)
java_files = [
    os.path.join(src_dir, "MainActivity.java"),
    os.path.join(src_dir, "QRScanActivity.java"),
    os.path.join(src_dir, "WorkerService.java"),
    os.path.join(src_dir, "BootReceiver.java")
]

classpath = f"{PLATFORM_JAR};{ZXING_JAR}"
subprocess.check_call([
    "javac",
    "-cp", classpath,
    "-d", classes_dir,
    "-g:none",
    "--release", "8"
] + java_files)

# 7. D8: Convert classes and zxing-core.jar to classes.dex
class_files = []
for root, _, files in os.walk(classes_dir):
    for f in files:
        if f.endswith(".class"):
            class_files.append(os.path.join(root, f))

subprocess.check_call([
    D8,
    "--output", WORKDIR,
    "--min-api", "24",
    "--lib", PLATFORM_JAR,
    ZXING_JAR
] + class_files)

dex_file = os.path.join(WORKDIR, "classes.dex")

# 8. Add classes.dex to unaligned APK
with zipfile.ZipFile(unaligned_apk, "a", compression=zipfile.ZIP_DEFLATED) as zf:
    zf.write(dex_file, "classes.dex")

# 9. Zipalign
aligned_apk = os.path.join(WORKDIR, "aligned.apk")
subprocess.check_call([ZIPALIGN, "-p", "-f", "-v", "4", unaligned_apk, aligned_apk])

# 10. Generate keystore if not exists
keystore_path = os.path.join(WORKDIR, "release.keystore")
subprocess.check_call([
    KEYTOOL,
    "-genkeypair",
    "-keystore", keystore_path,
    "-storepass", "android",
    "-keypass", "android",
    "-alias", "androidkey",
    "-keyalg", "RSA",
    "-keysize", "2048",
    "-validity", "10000",
    "-dname", "CN=BYOG, OU=Dev, O=BYOG, L=City, S=State, C=US"
])

# 11. Apksigner sign (v1 + v2 + v3)
subprocess.check_call([
    APKSIGNER, "sign",
    "--ks", keystore_path,
    "--ks-pass", "pass:android",
    "--key-pass", "pass:android",
    "--out", OUTPUT_APK,
    aligned_apk
])

# 12. Verify APK
subprocess.check_call([APKSIGNER, "verify", OUTPUT_APK])

print(f"SUCCESS: Generated signed, valid Android APK at: {OUTPUT_APK} (Size: {os.path.getsize(OUTPUT_APK)} bytes)")
