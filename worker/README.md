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
