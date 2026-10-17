import os
import sys
import subprocess
import shutil
import zipfile

BUILD_TOOLS = r"C:\Users\simse\AppData\Local\Android\Sdk\build-tools\37.0.0"
PLATFORM_JAR = r"C:\Users\simse\AppData\Local\Android\Sdk\platforms\android-34\android.jar"
KEYTOOL = r"C:\Program Files\Java\jdk-21.0.10\bin\keytool.exe"
AAPT2 = os.path.join(BUILD_TOOLS, "aapt2.exe")
D8 = os.path.join(BUILD_TOOLS, "d8.bat")
ZIPALIGN = os.path.join(BUILD_TOOLS, "zipalign.exe")
APKSIGNER = os.path.join(BUILD_TOOLS, "apksigner.bat")

WORKDIR = os.path.abspath(r"Z:\better-new-api\worker\mobile\build_tmp")
OUTPUT_APK = os.path.abspath(r"Z:\better-new-api\worker\binaries\byog-worker.apk")

if os.path.exists(WORKDIR):
    shutil.rmtree(WORKDIR, ignore_errors=True)
os.makedirs(WORKDIR, exist_ok=True)

# 1. Project structure
res_dir = os.path.join(WORKDIR, "res")
res_values = os.path.join(res_dir, "values")
res_mipmap = os.path.join(res_dir, "mipmap")
os.makedirs(res_values, exist_ok=True)
os.makedirs(res_mipmap, exist_ok=True)

# strings.xml
with open(os.path.join(res_values, "strings.xml"), "w", encoding="utf-8") as f:
    f.write('<?xml version="1.0" encoding="utf-8"?>\n<resources>\n    <string name="app_name">BYOG Worker</string>\n</resources>\n')

shutil.copyfile(r"Z:\better-new-api\web\public\logo.png", os.path.join(res_mipmap, "ic_launcher.png"))

# AndroidManifest.xml
manifest_content = '''<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="org.byog.worker"
    android:versionCode="1"
    android:versionName="1.0.0">

    <uses-sdk android:minSdkVersion="24" android:targetSdkVersion="34" />

    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <uses-permission android:name="android.permission.WAKE_LOCK" />
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />

    <application
        android:label="@string/app_name"
        android:icon="@mipmap/ic_launcher"
        android:theme="@android:style/Theme.DeviceDefault.NoActionBar">
        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
        <service
            android:name=".WorkerService"
            android:exported="false" />
    </application>
</manifest>
'''
manifest_path = os.path.join(WORKDIR, "AndroidManifest.xml")
with open(manifest_path, "w", encoding="utf-8") as f:
    f.write(manifest_content)

# 2. Java Sources
src_dir = os.path.join(WORKDIR, "src", "org", "byog", "worker")
os.makedirs(src_dir, exist_ok=True)

main_activity_content = '''package org.byog.worker;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;
import android.widget.Toast;

public class MainActivity extends Activity implements View.OnClickListener {
    private EditText etServerUrl;
    private EditText etPairingCode;
    private EditText etDeviceName;
    private TextView tvStatus;
    private Button btnToggle;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Simple programmatic UI layout
        android.widget.LinearLayout layout = new android.widget.LinearLayout(this);
        layout.setOrientation(android.widget.LinearLayout.VERTICAL);
        layout.setPadding(40, 60, 40, 40);

        TextView title = new TextView(this);
        title.setText("BYOG Mobile Worker");
        title.setTextSize(22);
        title.setTypeface(null, android.graphics.Typeface.BOLD);
        title.setPadding(0, 0, 0, 30);
        layout.addView(title);

        TextView lblServer = new TextView(this);
        lblServer.setText("Server URL:");
        layout.addView(lblServer);

        etServerUrl = new EditText(this);
        etServerUrl.setHint("https://api.example.com");
        layout.addView(etServerUrl);

        TextView lblCode = new TextView(this);
        lblCode.setText("Pairing Code:");
        layout.addView(lblCode);

        etPairingCode = new EditText(this);
        etPairingCode.setHint("6-digit Pairing Code");
        layout.addView(etPairingCode);

        TextView lblDevice = new TextView(this);
        lblDevice.setText("Device Name:");
        layout.addView(lblDevice);

        etDeviceName = new EditText(this);
        etDeviceName.setText(android.os.Build.MODEL);
        layout.addView(etDeviceName);

        tvStatus = new TextView(this);
        tvStatus.setText("Status: Ready to connect");
        tvStatus.setPadding(0, 30, 0, 30);
        layout.addView(tvStatus);

        btnToggle = new Button(this);
        btnToggle.setText("Start Worker");
        btnToggle.setOnClickListener(this);
        layout.addView(btnToggle);

        setContentView(layout);
    }

    @Override
    public void onClick(View v) {
        toggleWorker();
    }

    private void toggleWorker() {
        if (WorkerService.isRunning) {
            Intent stopIntent = new Intent(this, WorkerService.class);
            stopService(stopIntent);
            btnToggle.setText("Start Worker");
            tvStatus.setText("Status: Stopped");
            Toast.makeText(this, "BYOG Worker Stopped", Toast.LENGTH_SHORT).show();
        } else {
            String server = etServerUrl.getText().toString().trim();
            String code = etPairingCode.getText().toString().trim();
            String name = etDeviceName.getText().toString().trim();

            Intent startIntent = new Intent(this, WorkerService.class);
            startIntent.putExtra("server", server);
            startIntent.putExtra("code", code);
            startIntent.putExtra("name", name);
            if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.O) {
                startForegroundService(startIntent);
            } else {
                startService(startIntent);
            }
            btnToggle.setText("Stop Worker");
            tvStatus.setText("Status: Running in background");
            Toast.makeText(this, "BYOG Worker Started", Toast.LENGTH_SHORT).show();
        }
    }
}
'''
with open(os.path.join(src_dir, "MainActivity.java"), "w", encoding="utf-8") as f:
    f.write(main_activity_content)

worker_service_content = '''package org.byog.worker;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;

public class WorkerService extends Service {
    public static boolean isRunning = false;
    private PowerManager.WakeLock wakeLock;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        isRunning = true;
        PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
        if (pm != null) {
            wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "BYOG:WorkerWakeLock");
            wakeLock.acquire(24 * 60 * 60 * 1000L);
        }

        Notification notification;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder builder = new Notification.Builder(this, "byog_worker_channel")
                    .setContentTitle("BYOG Worker Active")
                    .setContentText("Participating in decentralized compute network")
                    .setSmallIcon(android.R.drawable.stat_notify_sync);
            notification = builder.build();
        } else {
            notification = new Notification();
        }

        startForeground(1001, notification);
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        isRunning = false;
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                    "byog_worker_channel",
                    "BYOG Worker Service",
                    NotificationManager.IMPORTANCE_LOW
            );
            NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (manager != null) {
                manager.createNotificationChannel(channel);
            }
        }
    }
}
'''
with open(os.path.join(src_dir, "WorkerService.java"), "w", encoding="utf-8") as f:
    f.write(worker_service_content)

# 3. AAPT2 Compile
compiled_res = os.path.join(WORKDIR, "compiled.flata")
subprocess.check_call([AAPT2, "compile", "--dir", res_dir, "-o", compiled_res])

# 4. AAPT2 Link (creates unaligned unsigned APK with compiled resources & AndroidManifest.xml)
unaligned_apk = os.path.join(WORKDIR, "unaligned.apk")
subprocess.check_call([
    AAPT2, "link",
    "-I", PLATFORM_JAR,
    "--manifest", manifest_path,
    "-o", unaligned_apk,
    compiled_res,
    "--auto-add-overlay"
])

# 5. Compile Java
classes_dir = os.path.join(WORKDIR, "classes")
os.makedirs(classes_dir, exist_ok=True)
java_files = [
    os.path.join(src_dir, "MainActivity.java"),
    os.path.join(src_dir, "WorkerService.java")
]
subprocess.check_call([
    "javac",
    "-cp", PLATFORM_JAR,
    "-d", classes_dir,
    "-g:none",
    "--release", "8"
] + java_files)

# 6. D8: Convert classes to classes.dex
subprocess.check_call([
    D8,
    "--output", WORKDIR,
    "--min-api", "24",
    "--lib", PLATFORM_JAR
] + [os.path.join(classes_dir, "org", "byog", "worker", f) for f in os.listdir(os.path.join(classes_dir, "org", "byog", "worker")) if f.endswith(".class")])

dex_file = os.path.join(WORKDIR, "classes.dex")

# 7. Add classes.dex to unaligned APK
with zipfile.ZipFile(unaligned_apk, "a", compression=zipfile.ZIP_DEFLATED) as zf:
    zf.write(dex_file, "classes.dex")

# 8. Zipalign
aligned_apk = os.path.join(WORKDIR, "aligned.apk")
subprocess.check_call([ZIPALIGN, "-p", "-f", "-v", "4", unaligned_apk, aligned_apk])

# 9. Generate keystore if not exists
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

# 10. Apksigner sign
subprocess.check_call([
    APKSIGNER, "sign",
    "--ks", keystore_path,
    "--ks-pass", "pass:android",
    "--key-pass", "pass:android",
    "--out", OUTPUT_APK,
    aligned_apk
])

# 11. Verify APK
subprocess.check_call([APKSIGNER, "verify", OUTPUT_APK])

print(f"SUCCESS: Generated signed, valid Android APK at: {OUTPUT_APK} (Size: {os.path.getsize(OUTPUT_APK)} bytes)")
