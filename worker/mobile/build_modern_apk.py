import os
import sys
import subprocess
import shutil
import zipfile
from PIL import Image

BUILD_TOOLS = r"C:\Users\simse\AppData\Local\Android\Sdk\build-tools\37.0.0"
PLATFORM_JAR = r"C:\Users\simse\AppData\Local\Android\Sdk\platforms\android-34\android.jar"
KEYTOOL = r"C:\Program Files\Java\jdk-21.0.10\bin\keytool.exe"
AAPT2 = os.path.join(BUILD_TOOLS, "aapt2.exe")
D8 = os.path.join(BUILD_TOOLS, "d8.bat")
ZIPALIGN = os.path.join(BUILD_TOOLS, "zipalign.exe")
APKSIGNER = os.path.join(BUILD_TOOLS, "apksigner.bat")

WORKDIR = os.path.abspath(r"Z:\better-new-api\worker\mobile\build_tmp_v2")
OUTPUT_APK = os.path.abspath(r"Z:\better-new-api\worker\binaries\byog-worker.apk")
ZXING_JAR = os.path.abspath(r"Z:\better-new-api\worker\mobile\zxing-core.jar")
ARM64_BIN = os.path.abspath(r"Z:\better-new-api\worker\binaries\byog-worker-android-arm64")

if not os.path.exists(ZXING_JAR):
    import urllib.request
    print("Downloading ZXing core library...")
    urllib.request.urlretrieve("https://repo1.maven.org/maven2/com/google/zxing/core/3.5.3/core-3.5.3.jar", ZXING_JAR)

if os.path.exists(WORKDIR):
    shutil.rmtree(WORKDIR, ignore_errors=True)
os.makedirs(WORKDIR, exist_ok=True)

# 1. Project structure
res_dir = os.path.join(WORKDIR, "res")
res_values = os.path.join(res_dir, "values")
os.makedirs(res_values, exist_ok=True)

# strings.xml
with open(os.path.join(res_values, "strings.xml"), "w", encoding="utf-8") as f:
    f.write('''<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">BYOG Worker</string>
    <string name="title_scan">Scan QR Code</string>
</resources>
''')

# Generate crisp icons for all mipmap densities from web/public/logo.png
densities = {
    "mipmap-mdpi": 48,
    "mipmap-hdpi": 72,
    "mipmap-xhdpi": 96,
    "mipmap-xxhdpi": 144,
    "mipmap-xxxhdpi": 192,
}
src_logo = Image.open(r"Z:\better-new-api\web\public\logo.png").convert("RGBA")
for folder, size in densities.items():
    mdir = os.path.join(res_dir, folder)
    os.makedirs(mdir, exist_ok=True)
    resized = src_logo.resize((size, size), Image.Resampling.LANCZOS)
    resized.save(os.path.join(mdir, "ic_launcher.png"), "PNG")

# AndroidManifest.xml
manifest_content = '''<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="org.byog.worker"
    android:versionCode="2"
    android:versionName="1.2.0">

    <uses-sdk android:minSdkVersion="24" android:targetSdkVersion="34" />

    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
    <uses-permission android:name="android.permission.WAKE_LOCK" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
    <uses-permission android:name="android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS" />
    <uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED" />
    <uses-permission android:name="android.permission.CAMERA" />
    <uses-permission android:name="android.permission.VIBRATE" />

    <uses-feature android:name="android.hardware.camera" android:required="false" />
    <uses-feature android:name="android.hardware.camera.autofocus" android:required="false" />

    <application
        android:label="@string/app_name"
        android:icon="@mipmap/ic_launcher"
        android:roundIcon="@mipmap/ic_launcher"
        android:theme="@android:style/Theme.DeviceDefault.NoActionBar">
        
        <activity
            android:name=".MainActivity"
            android:exported="true"
            android:configChanges="orientation|screenSize"
            android:windowSoftInputMode="adjustResize">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>

        <activity
            android:name=".QRScanActivity"
            android:exported="false"
            android:screenOrientation="portrait"
            android:theme="@android:style/Theme.DeviceDefault.NoActionBar.Fullscreen" />

        <service
            android:name=".WorkerService"
            android:exported="false"
            android:foregroundServiceType="dataSync" />

        <receiver
            android:name=".BootReceiver"
            android:exported="true"
            android:enabled="true">
            <intent-filter>
                <action android:name="android.intent.action.BOOT_COMPLETED" />
                <action android:name="android.intent.action.QUICKBOOT_POWERON" />
            </intent-filter>
        </receiver>
    </application>
</manifest>
'''
manifest_path = os.path.join(WORKDIR, "AndroidManifest.xml")
with open(manifest_path, "w", encoding="utf-8") as f:
    f.write(manifest_content)

# 2. Java Sources
src_dir = os.path.join(WORKDIR, "src", "org", "byog", "worker")
os.makedirs(src_dir, exist_ok=True)

# BootReceiver.java
with open(os.path.join(src_dir, "BootReceiver.java"), "w", encoding="utf-8") as f:
    f.write('''package org.byog.worker;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.os.Build;

public class BootReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        if (Intent.ACTION_BOOT_COMPLETED.equals(intent.getAction())) {
            SharedPreferences prefs = context.getSharedPreferences("byog_prefs", Context.MODE_PRIVATE);
            boolean wasActive = prefs.getBoolean("is_active", false);
            if (wasActive) {
                Intent serviceIntent = new Intent(context, WorkerService.class);
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    context.startForegroundService(serviceIntent);
                } else {
                    context.startService(serviceIntent);
                }
            }
        }
    }
}
''')

# QRScanActivity.java (Using android.hardware.Camera + ZXing)
with open(os.path.join(src_dir, "QRScanActivity.java"), "w", encoding="utf-8") as f:
    f.write('''package org.byog.worker;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.ImageFormat;
import android.hardware.Camera;
import android.os.Bundle;
import android.os.Vibrator;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.View;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.TextView;
import android.widget.Toast;

import com.google.zxing.BinaryBitmap;
import com.google.zxing.MultiFormatReader;
import com.google.zxing.PlanarYUVLuminanceSource;
import com.google.zxing.Result;
import com.google.zxing.common.HybridBinarizer;

import java.io.IOException;
import java.util.List;

@SuppressWarnings("deprecation")
public class QRScanActivity extends Activity implements SurfaceHolder.Callback, Camera.PreviewCallback {
    private Camera mCamera;
    private SurfaceView mSurfaceView;
    private SurfaceHolder mHolder;
    private boolean isScanning = true;
    private MultiFormatReader mReader = new MultiFormatReader();

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.BLACK);

        mSurfaceView = new SurfaceView(this);
        mHolder = mSurfaceView.getHolder();
        mHolder.addCallback(this);
        root.addView(mSurfaceView);

        // Overlay text
        TextView tvHint = new TextView(this);
        tvHint.setText("Scan BYOG Pairing QR Code on Desktop Screen");
        tvHint.setTextColor(Color.WHITE);
        tvHint.setTextSize(16);
        tvHint.setGravity(android.view.Gravity.CENTER);
        tvHint.setPadding(20, 80, 20, 20);
        root.addView(tvHint);

        // Cancel button
        Button btnCancel = new Button(this);
        btnCancel.setText("Cancel");
        btnCancel.setBackgroundColor(Color.parseColor("#334155"));
        btnCancel.setTextColor(Color.WHITE);
        FrameLayout.LayoutParams btnParams = new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.WRAP_CONTENT,
                FrameLayout.LayoutParams.WRAP_CONTENT,
                android.view.Gravity.BOTTOM | android.view.Gravity.CENTER_HORIZONTAL
        );
        btnParams.bottomMargin = 80;
        btnCancel.setLayoutParams(btnParams);
        btnCancel.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                finish();
            }
        });
        root.addView(btnCancel);

        setContentView(root);
    }

    @Override
    public void surfaceCreated(SurfaceHolder holder) {
        try {
            mCamera = Camera.open();
            mCamera.setDisplayOrientation(90);
            mCamera.setPreviewDisplay(holder);
            mCamera.setPreviewCallback(this);
            mCamera.startPreview();
        } catch (Exception e) {
            Toast.makeText(this, "Failed to open camera: " + e.getMessage(), Toast.LENGTH_SHORT).show();
            finish();
        }
    }

    @Override
    public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {
        if (mCamera != null) {
            try {
                Camera.Parameters parameters = mCamera.getParameters();
                List<String> focusModes = parameters.getSupportedFocusModes();
                if (focusModes != null && focusModes.contains(Camera.Parameters.FOCUS_MODE_CONTINUOUS_PICTURE)) {
                    parameters.setFocusMode(Camera.Parameters.FOCUS_MODE_CONTINUOUS_PICTURE);
                    mCamera.setParameters(parameters);
                }
            } catch (Exception ignored) {}
        }
    }

    @Override
    public void surfaceDestroyed(SurfaceHolder holder) {
        if (mCamera != null) {
            mCamera.setPreviewCallback(null);
            mCamera.stopPreview();
            mCamera.release();
            mCamera = null;
        }
    }

    @Override
    public void onPreviewFrame(byte[] data, Camera camera) {
        if (!isScanning || camera == null) return;

        Camera.Parameters parameters = camera.getParameters();
        Camera.Size size = parameters.getPreviewSize();
        int width = size.width;
        int height = size.height;

        try {
            PlanarYUVLuminanceSource source = new PlanarYUVLuminanceSource(
                    data, width, height, 0, 0, width, height, false
            );
            BinaryBitmap bitmap = new BinaryBitmap(new HybridBinarizer(source));
            Result result = mReader.decodeWithState(bitmap);
            if (result != null && result.getText() != null) {
                isScanning = false;
                try {
                    Vibrator v = (Vibrator) getSystemService(Context.VIBRATOR_SERVICE);
                    if (v != null) v.vibrate(80);
                } catch (Exception ignored) {}

                Intent intent = new Intent();
                intent.putExtra("scanned_code", result.getText());
                setResult(RESULT_OK, intent);
                finish();
            }
        } catch (Exception ignored) {
            // QR decode failed for this frame, continue scanning
        } finally {
            mReader.reset();
        }
    }
}
''')

# WorkerService.java
with open(os.path.join(src_dir, "WorkerService.java"), "w", encoding="utf-8") as f:
    f.write('''package org.byog.worker;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.ServiceInfo;
import android.net.ConnectivityManager;
import android.net.NetworkInfo;
import android.net.wifi.WifiManager;
import android.os.BatteryManager;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.PowerManager;
import android.util.Log;

import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileReader;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.UUID;

public class WorkerService extends Service {
    public static final String ACTION_STATUS_UPDATE = "org.byog.worker.STATUS_UPDATE";
    private static final String TAG = "BYOGWorkerService";
    private static final int NOTIFICATION_ID = 1001;
    private static final String CHANNEL_ID = "byog_worker_channel";

    public static boolean isRunning = false;
    public static String currentStatus = "Disconnected";
    public static String nodeId = "";
    public static double currentTps = 0.0;
    public static long totalTokens = 0;
    public static float batteryTemp = 0.0f;
    public static boolean isThrottled = false;
    public static int safeAllocatableHeapMB = 1024;
    public static int physicalRamMB = 4096;

    private PowerManager.WakeLock wakeLock;
    private WifiManager.WifiLock wifiLock;
    private Thread workerThread;
    private boolean shouldStop = false;
    private Handler mainHandler;

    private BroadcastReceiver batteryReceiver = new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
            if (Intent.ACTION_BATTERY_CHANGED.equals(intent.getAction())) {
                int tempRaw = intent.getIntExtra(BatteryManager.EXTRA_TEMPERATURE, 0);
                batteryTemp = tempRaw / 10.0f;
                // Thermal Cutoff at 50°C (Section 6.3)
                if (batteryTemp >= 50.0f) {
                    isThrottled = true;
                    currentStatus = "Thermal Guard: Paused (>50°C)";
                } else if (isThrottled && batteryTemp <= 45.0f) {
                    isThrottled = false;
                    currentStatus = "Active";
                }
                broadcastState();
            }
        }
    };

    @Override
    public void onCreate() {
        super.onCreate();
        mainHandler = new Handler(Looper.getMainLooper());
        createNotificationChannel();
        calculatePhysicalMemory();
        registerReceiver(batteryReceiver, new IntentFilter(Intent.ACTION_BATTERY_CHANGED));
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        isRunning = true;
        shouldStop = false;

        // WakeLock & WifiLock to keep worker alive in background (Section 6.2)
        try {
            PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
            if (pm != null && wakeLock == null) {
                wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "BYOG:WorkerWakeLock");
                wakeLock.acquire(24 * 60 * 60 * 1000L);
            }
            WifiManager wm = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
            if (wm != null && wifiLock == null) {
                wifiLock = wm.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF, "BYOG:WifiLock");
                wifiLock.acquire();
            }
        } catch (Exception e) {
            Log.e(TAG, "Error acquiring locks: " + e.getMessage());
        }

        // Start Foreground Service safely
        startForegroundServiceSafely();

        SharedPreferences prefs = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        prefs.edit().putBoolean("is_active", true).apply();

        String server = intent != null ? intent.getStringExtra("server") : null;
        String code = intent != null ? intent.getStringExtra("code") : null;
        if (server == null || server.isEmpty()) server = prefs.getString("server_url", "http://localhost:3000");
        if (code == null || code.isEmpty()) code = prefs.getString("pairing_code", "");

        final String finalServer = server;
        final String finalCode = code;

        if (workerThread == null || !workerThread.isAlive()) {
            workerThread = new Thread(new Runnable() {
                @Override
                public void run() {
                    runWorkerLoop(finalServer, finalCode);
                }
            });
            workerThread.start();
        }

        return START_STICKY;
    }

    private void startForegroundServiceSafely() {
        Notification notification = buildNotification("Initializing BYOG worker node...");
        try {
            if (Build.VERSION.SDK_INT >= 34) {
                startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
            } else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
            } else {
                startForeground(NOTIFICATION_ID, notification);
            }
        } catch (Exception e) {
            Log.e(TAG, "startForeground error: " + e.getMessage());
            try {
                startForeground(NOTIFICATION_ID, notification);
            } catch (Exception ignored) {}
        }
    }

    private void calculatePhysicalMemory() {
        try {
            BufferedReader reader = new BufferedReader(new FileReader("/proc/meminfo"));
            String line;
            int memTotalKB = 0;
            int memAvailableKB = 0;
            while ((line = reader.readLine()) != null) {
                if (line.startsWith("MemTotal:")) {
                    memTotalKB = parseKB(line);
                } else if (line.startsWith("MemAvailable:")) {
                    memAvailableKB = parseKB(line);
                }
            }
            reader.close();

            if (memTotalKB > 0) {
                physicalRamMB = memTotalKB / 1024;
                int availMB = memAvailableKB > 0 ? (memAvailableKB / 1024) : (physicalRamMB / 2);
                // Safe Budget Formula: min(MemAvailable * 0.55, PhysicalMemTotal * 0.40) (Section 6.1)
                safeAllocatableHeapMB = Math.min((int)(availMB * 0.55), (int)(physicalRamMB * 0.40));
                if (safeAllocatableHeapMB < 256) safeAllocatableHeapMB = 256;
            }
        } catch (Exception e) {
            physicalRamMB = 4096;
            safeAllocatableHeapMB = 1024;
        }
    }

    private int parseKB(String line) {
        String[] parts = line.split("\\\\s+");
        if (parts.length >= 2) {
            try {
                return Integer.parseInt(parts[1]);
            } catch (Exception ignored) {}
        }
        return 0;
    }

    private void runWorkerLoop(String serverUrl, String pairingCode) {
        currentStatus = "Connecting to server...";
        broadcastState();
        updateNotification("Connecting to " + serverUrl);

        SharedPreferences prefs = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        String hwid = prefs.getString("hwid", null);
        if (hwid == null) {
            hwid = "ANDROID-" + UUID.randomUUID().toString();
            prefs.edit().putString("hwid", hwid).apply();
        }

        while (!shouldStop) {
            try {
                // Register / Heartbeat with server
                JSONObject regBody = new JSONObject();
                regBody.put("hwid", hwid);
                regBody.put("pairing_code", pairingCode);
                regBody.put("os", "android");
                regBody.put("arch", "arm64");
                regBody.put("device_name", Build.MANUFACTURER + " " + Build.MODEL);
                regBody.put("device_type", "mobile");
                regBody.put("cpu_model", Build.HARDWARE != null ? Build.HARDWARE : "ARM64 Processor");
                regBody.put("gpu_model", "Vulkan / Mobile Compute Engine");
                regBody.put("physical_ram_mb", physicalRamMB);
                regBody.put("swap_ram_mb", 0); // ZRAM strictly ignored (Section 6.1)
                regBody.put("vram_mb", safeAllocatableHeapMB);
                regBody.put("bandwidth_down_mbps", 85.0);
                regBody.put("bandwidth_up_mbps", 40.0);
                regBody.put("ping_ms", 24);
                regBody.put("network_type", getNetworkType());

                URL url = new URL(serverUrl + "/api/v1/worker/register");
                HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                conn.setRequestMethod("POST");
                conn.setRequestProperty("Content-Type", "application/json");
                conn.setConnectTimeout(8000);
                conn.setReadTimeout(8000);
                conn.setDoOutput(true);

                OutputStream os = conn.getOutputStream();
                os.write(regBody.toString().getBytes("UTF-8"));
                os.close();

                int code = conn.getResponseCode();
                if (code == 200) {
                    InputStream is = conn.getInputStream();
                    BufferedReader reader = new BufferedReader(new InputStreamReader(is));
                    StringBuilder sb = new StringBuilder();
                    String line;
                    while ((line = reader.readLine()) != null) sb.append(line);
                    reader.close();

                    JSONObject res = new JSONObject(sb.toString());
                    if (res.optBoolean("success")) {
                        JSONObject data = res.optJSONObject("data");
                        if (data != null) {
                            nodeId = data.optString("node_id", "");
                            boolean isPaired = data.optBoolean("is_paired", false);
                            if (isPaired) {
                                currentStatus = "Active & Ready for Inferences";
                            } else {
                                currentStatus = "Online (Pending Pairing Confirmation)";
                            }
                        }
                    }
                } else {
                    currentStatus = "Server error (" + code + ")";
                }
                conn.disconnect();
            } catch (Exception e) {
                currentStatus = "Connection error: " + e.getMessage();
                Log.e(TAG, "Worker loop error: " + e.getMessage());
            }

            broadcastState();
            updateNotification("Active Node: " + (nodeId.isEmpty() ? "Connecting" : nodeId));

            // Sleep 10s between heartbeat checks
            try {
                Thread.sleep(10000);
            } catch (InterruptedException e) {
                break;
            }
        }
    }

    private String getNetworkType() {
        try {
            ConnectivityManager cm = (ConnectivityManager) getSystemService(Context.CONNECTIVITY_SERVICE);
            if (cm != null) {
                NetworkInfo info = cm.getActiveNetworkInfo();
                if (info != null && info.isConnected()) {
                    if (info.getType() == ConnectivityManager.TYPE_WIFI) return "wifi";
                    if (info.getType() == ConnectivityManager.TYPE_MOBILE) return "cellular";
                }
            }
        } catch (Exception ignored) {}
        return "unknown";
    }

    private void broadcastState() {
        Intent intent = new Intent(ACTION_STATUS_UPDATE);
        intent.putExtra("status", currentStatus);
        intent.putExtra("node_id", nodeId);
        intent.putExtra("tps", currentTps);
        intent.putExtra("tokens", totalTokens);
        intent.putExtra("battery_temp", batteryTemp);
        intent.putExtra("is_throttled", isThrottled);
        intent.putExtra("physical_ram", physicalRamMB);
        intent.putExtra("safe_ram", safeAllocatableHeapMB);
        sendBroadcast(intent);
    }

    private void updateNotification(String text) {
        NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (nm != null) {
            nm.notify(NOTIFICATION_ID, buildNotification(text));
        }
    }

    private Notification buildNotification(String text) {
        Intent intent = new Intent(this, MainActivity.class);
        PendingIntent pi = PendingIntent.getActivity(
                this, 0, intent,
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.M ? PendingIntent.FLAG_IMMUTABLE : 0
        );

        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            builder = new Notification.Builder(this, CHANNEL_ID);
        } else {
            builder = new Notification.Builder(this);
        }

        builder.setContentTitle("BYOG Worker Node Active")
                .setContentText(text)
                .setSmallIcon(android.R.drawable.stat_notify_sync)
                .setContentIntent(pi)
                .setOngoing(true);

        return builder.build();
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    CHANNEL_ID,
                    "BYOG Worker Background Service",
                    NotificationManager.IMPORTANCE_LOW
            );
            channel.setDescription("Background AI compute and reverse tunnel service");
            NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null) {
                nm.createNotificationChannel(channel);
            }
        }
    }

    @Override
    public void onDestroy() {
        shouldStop = true;
        isRunning = false;
        currentStatus = "Stopped";
        broadcastState();

        try {
            unregisterReceiver(batteryReceiver);
        } catch (Exception ignored) {}

        if (wakeLock != null && wakeLock.isHeld()) {
            try { wakeLock.release(); } catch (Exception ignored) {}
        }
        if (wifiLock != null && wifiLock.isHeld()) {
            try { wifiLock.release(); } catch (Exception ignored) {}
        }

        SharedPreferences prefs = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        prefs.edit().putBoolean("is_active", false).apply();

        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
''')

# MainActivity.java (Modern dark theme, zero-emoji, responsive, permission setup flow, QR scanner trigger)
with open(os.path.join(src_dir, "MainActivity.java"), "w", encoding="utf-8") as f:
    f.write('''package org.byog.worker;

import android.Manifest;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.BroadcastReceiver;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.Context;
import android.content.DialogInterface;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.PowerManager;
import android.provider.Settings;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import org.json.JSONObject;

public class MainActivity extends Activity {
    private static final int RC_CAMERA = 201;
    private static final int RC_QR_SCAN = 301;
    private static final int RC_NOTIF = 401;

    private EditText etPairingCode;
    private EditText etServerUrl;
    private LinearLayout llAdvancedServer;
    private TextView tvStatusBadge;
    private TextView tvNodeId;
    private TextView tvRamInfo;
    private TextView tvBatteryTemp;
    private Button btnAction;
    private Button btnScanQr;

    private BroadcastReceiver statusReceiver = new BroadcastReceiver() {
        @Override
        public void onReceive(Context context, Intent intent) {
            if (WorkerService.ACTION_STATUS_UPDATE.equals(intent.getAction())) {
                updateUiState();
            }
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        buildModernUi();
        checkAndRequestPermissions();
        loadSavedConfig();
    }

    @Override
    protected void onResume() {
        super.onResume();
        registerReceiver(statusReceiver, new IntentFilter(WorkerService.ACTION_STATUS_UPDATE));
        updateUiState();
    }

    @Override
    protected void onPause() {
        super.onPause();
        try {
            unregisterReceiver(statusReceiver);
        } catch (Exception ignored) {}
    }

    private void checkAndRequestPermissions() {
        // 1. Notification Permission (Android 13+)
        if (Build.VERSION.SDK_INT >= 33) {
            if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
                requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, RC_NOTIF);
            }
        }

        // 2. Battery Optimization Exemption (Section 6.2)
        try {
            PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
            if (pm != null && !pm.isIgnoringBatteryOptimizations(getPackageName())) {
                showBatteryOptimizationDialog();
            }
        } catch (Exception ignored) {}
    }

    private void showBatteryOptimizationDialog() {
        new AlertDialog.Builder(this)
                .setTitle("Background Persistence Setup")
                .setMessage("To participate in decentralized AI compute without interruption, please allow BYOG Worker to run in the background by disabling battery restrictions.")
                .setPositiveButton("Configure", new DialogInterface.OnClickListener() {
                    @Override
                    public void onClick(DialogInterface dialog, int which) {
                        try {
                            Intent intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
                            intent.setData(Uri.parse("package:" + getPackageName()));
                            startActivity(intent);
                        } catch (Exception e) {
                            try {
                                Intent fallback = new Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS);
                                startActivity(fallback);
                            } catch (Exception ignored) {}
                        }
                    }
                })
                .setNegativeButton("Later", null)
                .show();
    }

    private void buildModernUi() {
        ScrollView scrollView = new ScrollView(this);
        scrollView.setBackgroundColor(Color.parseColor("#0b0f19")); // Deep slate background

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(40, 60, 40, 60);

        // Header Card
        LinearLayout header = new LinearLayout(this);
        header.setOrientation(LinearLayout.VERTICAL);
        header.setPadding(30, 30, 30, 30);
        header.setBackground(createRoundedDrawable("#1e293b", "#334155", 20));

        TextView tvTitle = new TextView(this);
        tvTitle.setText("BYOG Mobile Worker");
        tvTitle.setTextSize(22);
        tvTitle.setTextColor(Color.WHITE);
        tvTitle.setTypeface(null, Typeface.BOLD);
        header.addView(tvTitle);

        TextView tvSubtitle = new TextView(this);
        tvSubtitle.setText("Decentralized Compute Edge Node (" + Build.MANUFACTURER + " " + Build.MODEL + ")");
        tvSubtitle.setTextSize(13);
        tvSubtitle.setTextColor(Color.parseColor("#94a3b8"));
        tvSubtitle.setPadding(0, 8, 0, 0);
        header.addView(tvSubtitle);

        root.addView(header);

        // Pairing Action Card
        LinearLayout pairCard = new LinearLayout(this);
        pairCard.setOrientation(LinearLayout.VERTICAL);
        pairCard.setPadding(36, 40, 36, 40);
        LinearLayout.LayoutParams cardParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT
        );
        cardParams.topMargin = 30;
        pairCard.setLayoutParams(cardParams);
        pairCard.setBackground(createRoundedDrawable("#111827", "#1f2937", 20));

        // Scan QR Button
        btnScanQr = new Button(this);
        btnScanQr.setText("SCAN PAIRING QR CODE");
        btnScanQr.setTextColor(Color.WHITE);
        btnScanQr.setTextSize(14);
        btnScanQr.setTypeface(null, Typeface.BOLD);
        btnScanQr.setBackground(createRoundedDrawable("#2563eb", "#1d4ed8", 14));
        btnScanQr.setPadding(30, 30, 30, 30);
        btnScanQr.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                startQrScan();
            }
        });
        pairCard.addView(btnScanQr);

        // Divider
        TextView tvOr = new TextView(this);
        tvOr.setText("OR ENTER PAIRING CODE");
        tvOr.setTextColor(Color.parseColor("#64748b"));
        tvOr.setTextSize(11);
        tvOr.setGravity(Gravity.CENTER);
        tvOr.setPadding(0, 30, 0, 20);
        pairCard.addView(tvOr);

        // Pairing Code Input with Paste Button
        LinearLayout codeRow = new LinearLayout(this);
        codeRow.setOrientation(LinearLayout.HORIZONTAL);

        etPairingCode = new EditText(this);
        etPairingCode.setHint("9K4L-28XM-73PQ-51ZR");
        etPairingCode.setHintTextColor(Color.parseColor("#475569"));
        etPairingCode.setTextColor(Color.parseColor("#38bdf8"));
        etPairingCode.setTextSize(16);
        etPairingCode.setTypeface(Typeface.MONOSPACE, Typeface.BOLD);
        etPairingCode.setGravity(Gravity.CENTER);
        etPairingCode.setBackground(createRoundedDrawable("#0f172a", "#334155", 12));
        etPairingCode.setPadding(20, 25, 20, 25);
        LinearLayout.LayoutParams etParams = new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1.0f);
        etPairingCode.setLayoutParams(etParams);
        codeRow.addView(etPairingCode);

        Button btnPaste = new Button(this);
        btnPaste.setText("Paste");
        btnPaste.setTextColor(Color.WHITE);
        btnPaste.setTextSize(12);
        btnPaste.setBackground(createRoundedDrawable("#334155", "#475569", 12));
        LinearLayout.LayoutParams pasteParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT
        );
        pasteParams.leftMargin = 16;
        btnPaste.setLayoutParams(pasteParams);
        btnPaste.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                pasteCodeFromClipboard();
            }
        });
        codeRow.addView(btnPaste);

        pairCard.addView(codeRow);

        // Action Button: Link & Start
        btnAction = new Button(this);
        btnAction.setText("CONNECT AND START WORKER");
        btnAction.setTextColor(Color.WHITE);
        btnAction.setTextSize(14);
        btnAction.setTypeface(null, Typeface.BOLD);
        btnAction.setBackground(createRoundedDrawable("#059669", "#047857", 14));
        btnAction.setPadding(30, 32, 30, 32);
        LinearLayout.LayoutParams btnParams = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT
        );
        btnParams.topMargin = 30;
        btnAction.setLayoutParams(btnParams);
        btnAction.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                toggleWorker();
            }
        });
        pairCard.addView(btnAction);

        root.addView(pairCard);

        // Telemetry Card
        LinearLayout telemCard = new LinearLayout(this);
        telemCard.setOrientation(LinearLayout.VERTICAL);
        telemCard.setPadding(36, 36, 36, 36);
        telemCard.setLayoutParams(cardParams);
        telemCard.setBackground(createRoundedDrawable("#1e293b", "#334155", 20));

        TextView tvTelemHeader = new TextView(this);
        tvTelemHeader.setText("LIVE NODE TELEMETRY");
        tvTelemHeader.setTextColor(Color.parseColor("#94a3b8"));
        tvTelemHeader.setTextSize(11);
        tvTelemHeader.setTypeface(null, Typeface.BOLD);
        telemCard.addView(tvTelemHeader);

        tvStatusBadge = new TextView(this);
        tvStatusBadge.setText("Status: Stopped");
        tvStatusBadge.setTextColor(Color.parseColor("#f87171"));
        tvStatusBadge.setTextSize(15);
        tvStatusBadge.setTypeface(null, Typeface.BOLD);
        tvStatusBadge.setPadding(0, 16, 0, 8);
        telemCard.addView(tvStatusBadge);

        tvNodeId = new TextView(this);
        tvNodeId.setText("Node ID: Not assigned");
        tvNodeId.setTextColor(Color.parseColor("#cbd5e1"));
        tvNodeId.setTextSize(13);
        tvNodeId.setTypeface(Typeface.MONOSPACE);
        telemCard.addView(tvNodeId);

        tvRamInfo = new TextView(this);
        tvRamInfo.setText("Physical RAM: Scanning...");
        tvRamInfo.setTextColor(Color.parseColor("#94a3b8"));
        tvRamInfo.setTextSize(12);
        tvRamInfo.setPadding(0, 8, 0, 0);
        telemCard.addView(tvRamInfo);

        tvBatteryTemp = new TextView(this);
        tvBatteryTemp.setText("Battery Temperature: Normal");
        tvBatteryTemp.setTextColor(Color.parseColor("#94a3b8"));
        tvBatteryTemp.setTextSize(12);
        tvBatteryTemp.setPadding(0, 4, 0, 0);
        telemCard.addView(tvBatteryTemp);

        root.addView(telemCard);

        // Collapsible Advanced Server URL Setting
        final TextView tvToggleAdvanced = new TextView(this);
        tvToggleAdvanced.setText("Advanced: Server Connection Settings");
        tvToggleAdvanced.setTextColor(Color.parseColor("#64748b"));
        tvToggleAdvanced.setTextSize(12);
        tvToggleAdvanced.setGravity(Gravity.CENTER);
        tvToggleAdvanced.setPadding(0, 36, 0, 10);
        tvToggleAdvanced.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                if (llAdvancedServer.getVisibility() == View.VISIBLE) {
                    llAdvancedServer.setVisibility(View.GONE);
                } else {
                    llAdvancedServer.setVisibility(View.VISIBLE);
                }
            }
        });
        root.addView(tvToggleAdvanced);

        llAdvancedServer = new LinearLayout(this);
        llAdvancedServer.setOrientation(LinearLayout.VERTICAL);
        llAdvancedServer.setVisibility(View.GONE);
        llAdvancedServer.setPadding(30, 20, 30, 20);
        llAdvancedServer.setBackground(createRoundedDrawable("#0f172a", "#1e293b", 12));

        TextView tvServerLbl = new TextView(this);
        tvServerLbl.setText("API Server Origin URL:");
        tvServerLbl.setTextColor(Color.parseColor("#94a3b8"));
        tvServerLbl.setTextSize(12);
        llAdvancedServer.addView(tvServerLbl);

        etServerUrl = new EditText(this);
        etServerUrl.setHint("https://api.yourdomain.com");
        etServerUrl.setTextColor(Color.WHITE);
        etServerUrl.setTextSize(14);
        etServerUrl.setBackground(createRoundedDrawable("#1e293b", "#334155", 10));
        etServerUrl.setPadding(16, 20, 16, 20);
        llAdvancedServer.addView(etServerUrl);

        root.addView(llAdvancedServer);

        scrollView.addView(root);
        setContentView(scrollView);
    }

    private GradientDrawable createRoundedDrawable(String bgColor, String strokeColor, int radiusDp) {
        GradientDrawable gd = new GradientDrawable();
        gd.setColor(Color.parseColor(bgColor));
        gd.setCornerRadius(radiusDp * getResources().getDisplayMetrics().density);
        gd.setStroke((int)(1 * getResources().getDisplayMetrics().density), Color.parseColor(strokeColor));
        return gd;
    }

    private void loadSavedConfig() {
        SharedPreferences prefs = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        String server = prefs.getString("server_url", "http://localhost:3000");
        String code = prefs.getString("pairing_code", "");
        etServerUrl.setText(server);
        if (!code.isEmpty()) etPairingCode.setText(code);
    }

    private void startQrScan() {
        if (checkSelfPermission(Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.CAMERA}, RC_CAMERA);
        } else {
            Intent intent = new Intent(this, QRScanActivity.class);
            startActivityForResult(intent, RC_QR_SCAN);
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == RC_QR_SCAN && resultCode == RESULT_OK && data != null) {
            String scanned = data.getStringExtra("scanned_code");
            if (scanned != null && !scanned.isEmpty()) {
                parseAndApplyScannedPayload(scanned);
            }
        }
    }

    private void parseAndApplyScannedPayload(String payload) {
        try {
            // Check if JSON payload {"server_url":"...","pairing_code":"..."}
            if (payload.startsWith("{")) {
                JSONObject json = new JSONObject(payload);
                String server = json.optString("server_url", json.optString("server", ""));
                String code = json.optString("pairing_code", json.optString("code", ""));
                if (!server.isEmpty()) etServerUrl.setText(server);
                if (!code.isEmpty()) etPairingCode.setText(code.toUpperCase());
            } else if (payload.startsWith("byog://")) {
                Uri uri = Uri.parse(payload);
                String server = uri.getQueryParameter("server");
                String code = uri.getQueryParameter("code");
                if (server != null && !server.isEmpty()) etServerUrl.setText(server);
                if (code != null && !code.isEmpty()) etPairingCode.setText(code.toUpperCase());
            } else {
                // Raw pairing code
                etPairingCode.setText(payload.trim().toUpperCase());
            }
            Toast.makeText(this, "QR Code scanned successfully! Connecting...", Toast.LENGTH_SHORT).show();
            // Automatically trigger worker start!
            startWorker();
        } catch (Exception e) {
            etPairingCode.setText(payload.trim().toUpperCase());
        }
    }

    private void pasteCodeFromClipboard() {
        try {
            ClipboardManager clipboard = (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
            if (clipboard != null && clipboard.hasPrimaryClip() && clipboard.getPrimaryClip().getItemCount() > 0) {
                ClipData.Item item = clipboard.getPrimaryClip().getItemAt(0);
                String text = item.getText().toString().trim().toUpperCase();
                etPairingCode.setText(text);
                Toast.makeText(this, "Pasted from clipboard", Toast.LENGTH_SHORT).show();
            }
        } catch (Exception e) {
            Toast.makeText(this, "Could not paste: " + e.getMessage(), Toast.LENGTH_SHORT).show();
        }
    }

    private void toggleWorker() {
        if (WorkerService.isRunning) {
            stopWorker();
        } else {
            startWorker();
        }
    }

    private void startWorker() {
        String code = etPairingCode.getText().toString().trim().toUpperCase();
        String server = etServerUrl.getText().toString().trim();

        if (code.isEmpty()) {
            Toast.makeText(this, "Please enter or scan a pairing code", Toast.LENGTH_SHORT).show();
            return;
        }

        if (server.isEmpty()) {
            server = "http://localhost:3000";
        }

        SharedPreferences prefs = getSharedPreferences("byog_prefs", MODE_PRIVATE);
        prefs.edit()
                .putString("server_url", server)
                .putString("pairing_code", code)
                .apply();

        Intent serviceIntent = new Intent(this, WorkerService.class);
        serviceIntent.putExtra("server", server);
        serviceIntent.putExtra("code", code);

        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                startForegroundService(serviceIntent);
            } else {
                startService(serviceIntent);
            }
            updateUiState();
            Toast.makeText(this, "BYOG Worker Started", Toast.LENGTH_SHORT).show();
        } catch (Exception e) {
            Toast.makeText(this, "Error starting worker: " + e.getMessage(), Toast.LENGTH_LONG).show();
        }
    }

    private void stopWorker() {
        Intent serviceIntent = new Intent(this, WorkerService.class);
        stopService(serviceIntent);
        WorkerService.isRunning = false;
        updateUiState();
        Toast.makeText(this, "BYOG Worker Stopped", Toast.LENGTH_SHORT).show();
    }

    private void updateUiState() {
        if (WorkerService.isRunning) {
            btnAction.setText("STOP WORKER");
            btnAction.setBackground(createRoundedDrawable("#dc2626", "#b91c1c", 14));
            tvStatusBadge.setText("Status: " + WorkerService.currentStatus);
            tvStatusBadge.setTextColor(Color.parseColor("#34d399")); // Emerald green
        } else {
            btnAction.setText("CONNECT AND START WORKER");
            btnAction.setBackground(createRoundedDrawable("#059669", "#047857", 14));
            tvStatusBadge.setText("Status: Stopped");
            tvStatusBadge.setTextColor(Color.parseColor("#f87171"));
        }

        if (!WorkerService.nodeId.isEmpty()) {
            tvNodeId.setText("Node ID: " + WorkerService.nodeId);
        } else {
            tvNodeId.setText("Node ID: Not assigned");
        }

        tvRamInfo.setText("Physical RAM: " + (WorkerService.physicalRamMB / 1024.0) + " GB (ZRAM Filtered) | Safe Budget: " + WorkerService.safeAllocatableHeapMB + " MB");
        if (WorkerService.batteryTemp > 0) {
            tvBatteryTemp.setText("Battery Temperature: " + WorkerService.batteryTemp + "°C " + (WorkerService.isThrottled ? "(Throttled)" : "(Safe)"));
        }
    }
}
''')

# 3. AAPT2 Compile Resources
compiled_res = os.path.join(WORKDIR, "compiled.flata")
subprocess.check_call([AAPT2, "compile", "--dir", res_dir, "-o", compiled_res])

# 4. AAPT2 Link
unaligned_apk = os.path.join(WORKDIR, "unaligned.apk")
subprocess.check_call([
    AAPT2, "link",
    "-I", PLATFORM_JAR,
    "--manifest", manifest_path,
    "-o", unaligned_apk,
    compiled_res,
    "--auto-add-overlay"
])

# 5. Compile Java Sources with ZXing jar on classpath
classes_dir = os.path.join(WORKDIR, "classes")
os.makedirs(classes_dir, exist_ok=True)
java_files = [
    os.path.join(src_dir, f) for f in os.listdir(src_dir) if f.endswith(".java")
]

subprocess.check_call([
    "javac",
    "-cp", f"{PLATFORM_JAR};{ZXING_JAR}",
    "-d", classes_dir,
    "-g:none",
    "--release", "8"
] + java_files)

# 6. D8: Convert compiled classes + ZXing jar to classes.dex
all_classes = []
for root, dirs, files in os.walk(classes_dir):
    for f in files:
        if f.endswith(".class"):
            all_classes.append(os.path.join(root, f))

subprocess.check_call([
    D8,
    "--output", WORKDIR,
    "--min-api", "24",
    "--lib", PLATFORM_JAR
] + all_classes + [ZXING_JAR])

dex_file = os.path.join(WORKDIR, "classes.dex")

# 7. Add classes.dex and embedded worker binary into unaligned APK
with zipfile.ZipFile(unaligned_apk, "a", compression=zipfile.ZIP_DEFLATED) as zf:
    zf.write(dex_file, "classes.dex")
    if os.path.exists(ARM64_BIN):
        print(f"Bundling native Go worker binary: {ARM64_BIN}")
        zf.write(ARM64_BIN, "assets/byog-worker-android-arm64")

# 8. Zipalign
aligned_apk = os.path.join(WORKDIR, "aligned.apk")
subprocess.check_call([ZIPALIGN, "-p", "-f", "-v", "4", unaligned_apk, aligned_apk])

# 9. Keytool Keystore
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
    "-dname", "CN=BYOG Worker, OU=Platform, O=Compute, L=City, ST=State, C=US"
])

# 10. Apksigner (v1 + v2 + v3)
subprocess.check_call([
    APKSIGNER, "sign",
    "--ks", keystore_path,
    "--ks-pass", "pass:android",
    "--key-pass", "pass:android",
    "--out", OUTPUT_APK,
    aligned_apk
])

# 11. Verify APK
subprocess.check_call([APKSIGNER, "verify", "--verbose", OUTPUT_APK])

size_mb = os.path.getsize(OUTPUT_APK) / (1024 * 1024)
print(f"SUCCESS: Generated modern, signed Android APK: {OUTPUT_APK} ({size_mb:.2f} MB)")
