# BYOG Worker (BYOG Worker)

The clients of the local computing network of the installation. Every client
runs on a computer of a user, it serves the models of the installation on the
device and it takes the tasks of the coordinator.

## The clients

| Dosya | Sistem | Arayüz |
| --- | --- | --- |
| `BYOG Worker.exe` | Windows | Kendi penceresi, bildirim alanında simge |
| `BYOG Worker Kurulumu.exe` | Windows | Dosya, istemciyi kendisi kurar |
| `byog-worker-linux-amd64` | Linux | Tarayıcıda açılan sayfa |
| `byog-worker-darwin-arm64` | macOS | Tarayıcıda açılan sayfa |
| `byog-worker-android-arm64` | Android / Termux | Termux arayüzü (`termux-gui`) |
| `byog-worker.apk` | Android | Uygulamanın kendi arayüzü |

## The installation on Windows

The client of Windows is a program of the graphical user interface, it has no
terminal. The program of the installation is a copy of the client:

1. The user downloads `BYOG Worker Kurulumu.exe` from the dashboard.
2. The program copies itself into `%LOCALAPPDATA%\Programs\BYOG Worker`.
3. The entries of the control panel are written, the client appears in the
   applications of the system and in the search of Windows.
4. The shortcut of the start menu is written, the client starts.

The client is removed with the applications of the control panel. The entry of
the system is `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\BYOGWorker`,
the removal calls the client with `--uninstall`.

The client installs itself when it is started outside of the program directory
of the user, the file of the installation is only a copy with a name that the
user understands.

## The configuration of the client

The files of the client are in the profile of the user:

| Dosya | İçerik |
| --- | --- |
| `~/.byog/config.json` | Sunucu adresi, eşleştirme kodu, cihaz adı |
| `~/.byog/worker.log` | Kayıtlar |
| `~/.byog/models/` | Modeller |

The server address and the pairing code are also written in the interface of
the client.

## The build of the clients

```
powershell -ExecutionPolicy Bypass -File worker\build-clients.ps1
```

The script builds the clients of all systems, the client of Windows is built
with the subsystem of the graphical user interface, it has no terminal.
