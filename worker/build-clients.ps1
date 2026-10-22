# Builds the clients of the local computing network.
#
# The Windows client is a program of the graphical user interface, it is shown
# in the applications of the control panel and in the search of the system. The
# program of the installation is built with Inno Setup, the user chooses the
# directory of the installation and decides whether the client gets a shortcut
# on the desktop.

param(
    [switch]$SkipMobile,
    [switch]$SkipInstaller
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binaries = Join-Path $root "worker\binaries"

New-Item -ItemType Directory -Force -Path $binaries | Out-Null

# The temporary folder of the compiler is the folder of the build.
#
# The antivirus of the computer scans the temporary folder of the system and it
# blocks the program of the compiler with a wrong warning, the build fails with
# it. The folder of the build is not scanned.
$tempFolder = Join-Path $root "worker\.buildtmp"
New-Item -ItemType Directory -Force -Path $tempFolder | Out-Null
$env:GOTMPDIR = $tempFolder

Write-Host "BYOG Worker (Windows) derleniyor..."
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# The client has no terminal, the graphical interface of the client is the
# program itself.
$ldflags = "-s -w -H windowsgui"
go build -ldflags $ldflags -o (Join-Path $binaries "BYOG Worker.exe") ./worker/desktop

# The icon of the installation is the picture of the logo.
$icon = Join-Path $root "worker\desktop\assets\byog.ico"
if (-not (Test-Path $icon)) {
    Write-Host "Ikon bulunamadi: $icon"
}

Write-Host "BYOG Worker (Linux) derleniyor..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags "-s -w" -o (Join-Path $binaries "byog-worker-linux-amd64") ./worker/desktop

Write-Host "BYOG Worker (macOS) derleniyor..."
$env:GOOS = "darwin"
$env:GOARCH = "arm64"
go build -ldflags "-s -w" -o (Join-Path $binaries "byog-worker-darwin-arm64") ./worker/desktop

Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED

if (-not $SkipMobile) {
    Write-Host "BYOG Worker (Android / Termux) derleniyor..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    go build -ldflags "-s -w" -o (Join-Path $binaries "byog-worker-android-arm64") ./worker/mobile
    Remove-Item Env:GOOS
    Remove-Item Env:GOARCH
    Remove-Item Env:CGO_ENABLED
}

if (-not $SkipInstaller) {
    # The program of the installation is built with Inno Setup, the standard
    # tool of Windows.
    $compiler = $null
    foreach ($candidate in @(
            "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
            "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
            "C:\Program Files\Inno Setup 6\ISCC.exe")) {
        if (Test-Path $candidate) {
            $compiler = $candidate
            break
        }
    }
    if ($null -eq $compiler) {
        Write-Host "Inno Setup bulunamadi, kurulum programi derlenmedi."
    } else {
        Write-Host "BYOG Worker Kurulumu derleniyor..."
        & $compiler (Join-Path $root "worker\installer\byog-worker.iss")
        if ($LASTEXITCODE -ne 0) {
            throw "Kurulum programi derlenemedi."
        }
    }
}

Write-Host ""
Write-Host "İstemciler derlendi:"
Get-ChildItem $binaries | Select-Object Name, Length | Format-Table -AutoSize
