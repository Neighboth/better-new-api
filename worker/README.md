# BYOG Worker

The clients of the local computing network of the installation. Every client
runs on a computer of a user, it serves the models of the installation on the
device and it takes the tasks of the coordinator.

## The clients

| Dosya | Sistem | Arayüz |
| --- | --- | --- |
| `BYOG Worker.exe` | Windows | Inno Setup kurulum programı (panodan indirilen dosya) |
| `BYOG Worker Kurulumu.exe` | Windows | Inno Setup derlemesinin kaynak dosyası |
| `byog-worker-linux-amd64` | Linux (x86-64) | Tarayıcıda açılan sayfa |
| `byog-worker-linux-arm64` | Linux (ARM64, Raspberry Pi 4/5) | Tarayıcıda açılan sayfa |
| `byog-worker-linux-arm` | Linux (ARMv7) | Tarayıcıda açılan sayfa |
| `byog-worker-darwin-arm64` | macOS (Apple Silicon) | Tarayıcıda açılan sayfa |
| `byog-worker-android-arm64` | Android / Termux | Termux arayüzü (`termux-gui`) |
| `byog-worker.apk` | Android | Uygulamanın kendi arayüzü |

## The installation on Windows

The dashboard hands out one file, `BYOG Worker.exe`; it is the installer of Inno
Setup. The name, the address of the installation, the favicon and the icon are
written into the installer when it is downloaded, the installer and the client
do not ask the user for them.

1. The installer offers the language and the directory of the installation, the
   default directory is `%LOCALAPPDATA%\Programs\<sunucu adı> BYOG Worker`.
2. It shows the policies of the installation and requires the acceptance before
   the client is installed.
3. It writes the language and the acceptance into `~/.byog/config.json`, the
   client starts with the same language and it does not show the page of the
   first start again.
4. It writes the settings of the installation at the end of the installed
   `BYOG Worker.exe`, the client reads the name, the address, the favicon and
   the icon from its own file.
5. It writes the entry of the control panel, the entry of `App Paths` and the
   entry that starts the client with the session of the user. It does not write
   a shortcut on the desktop or in the menu of the programs.
6. The page of the client is shown in a window of WebView2. The window runs in
   a process of its own. When the runtime of WebView2 cannot be started, the
   window of the program shows the state of the worker; the client never opens
   a browser.

The program of the client is still served without `?format=installer` for the
self-updater of the installed clients. The dashboard asks for the installer with
`?format=installer`; the response keeps the file name `BYOG Worker.exe`.

The remove entry is
`HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\{5B4C0A1E-6C1B-4C0E-9E4F-7A3D2B5C8E11}_is1`.

## The configuration of the client

The files of the client are in the profile of the user:

| Dosya | İçerik |
| --- | --- |
| `~/.byog/config.json` | Sunucu adresi, eşleştirme kodu, cihaz adı, dil |
| `~/.byog/installer.json` | Kurulum programının seçtiği dil |
| `~/.byog/worker.log` | Kayıtlar |
| `~/.byog/webview.log` | Kayıtlar of the window of the client (Windows) |
| `~/.byog/models/` | Modeller |

The server address and the pairing code are also written in the page of the
client. The field of the server address is hidden when the address is known from
the dashboard.

## The build of the clients

```
powershell -ExecutionPolicy Bypass -File worker\build-clients.ps1
```

The script builds the clients of all systems, the client of Windows is built
with the subsystem of the graphical user interface, it has no terminal. The
program of the installation is built with Inno Setup, it is skipped when Inno
Setup is not on the computer.

The application of Android is built with the tools of the Android SDK:

```
python worker\mobile\build_apk.py
```

The version of the application follows `WorkerVersion` in
`worker/desktop/main.go`, the update check of the application compares the
version of the server with the version of the application.

### Security and Android signing

Worker connections require HTTPS (HTTP is allowed only for loopback development;
Android emulators may use `10.0.2.2`). The private per-device token is issued
once at pairing; save and restore the worker's private configuration securely.
Deploy a trusted TLS certificate. Existing workers must be paired again.

The server re-signs each downloadable APK with an installation-specific private
key. Set `BYOG_APK_SIGNING_KEY` to a protected path on persistent storage; all
server instances serving the same installation must share that key. Keep secure
backups: changing or losing it requires Android clients to be reinstalled. The
old repository-shared APK signing key was public and must not be reused. The
build script's local keystore signs only the intermediate base APK; it must not
be published or committed. If branding or signing fails, the server refuses to
serve the APK rather than returning an APK with the wrong signer.

Do not point `BYOG_APK_SIGNING_KEY` at the old public repository key or any
copy of it. Key rotation changes Android's signing identity and requires an
uninstall/reinstall; do not lose the new key after deployment.

Pairing sessions use the shared database, but the live worker WebSocket hub is
process-local. For a multi-instance installation, route all BYOG worker and
BYOG inference/control traffic to the same coordinator process; shared SQL and
signing keys alone do not provide cross-instance tunnel dispatch. A distributed
hub and failover have not been implemented or validated.

## Ücretli BYOG kullanım sınırları

Başka bir kullanıcının cihazında çalışan ve kalıcı cüzdan hold'u kullanan
ücretli, toplu-güncelleme dışı isteklerde sunucu işe başlamadan önce cüzdan
ve API anahtarı kotasından azami tutarı ayırır; kullanılmayan kısım işlem
sonunda iade edilir. `max_tokens` belirtilmezse 4096 kullanılır; 32768'in
üzerindeki değerler reddedilir. İleti ve promptun birleşik boyutu 8 MiB ile
sınırlıdır. İstenen azami tutar için bakiye yoksa daha küçük bir `max_tokens`
ile tekrar denenebilir. Kullanıcının kendi cihazındaki ücretsiz kullanım bu
sınırlandırılmış ücretli yolun dışındadır. Ağ kesilmesi veya geçersiz worker
kullanım bildirimi belirsiz işlem olarak yöneticinin incelemesini gerektirebilir.
Bu sınırlar worker'ın model token sayısını dürüst bildirdiğinin kanıtı değildir.
