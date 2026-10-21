# Builds the clients of the local computing network.
#
# The Windows client is a program of the graphical user interface, it is shown
# in the applications of the control panel and in the search of the system. The
# program of the installation copies the client into the computer and writes the
# entries of the system.

param(
    [switch]$SkipMobile
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$binaries = Join-Path $root "worker\binaries"

New-Item -ItemType Directory -Force -Path $binaries | Out-Null

Write-Host "BYOG Worker (Windows) derleniyor..."
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# The client has no terminal, the graphical interface of the client is the
# program itself.
$ldflags = "-s -w -H windowsgui"
$workerExe = Join-Path $binaries "byog-worker.exe"
go build -ldflags $ldflags -o $workerExe ./worker/desktop
Copy-Item $workerExe (Join-Path $binaries "BYOG Worker.exe") -Force

# The file of the installation is a copy of the client, the client installs
# itself when it is started for the first time.
Write-Host "BYOG Worker Kurulumu hazirlaniyor..."
Copy-Item (Join-Path $binaries "BYOG Worker.exe") (Join-Path $binaries "BYOG Worker Kurulumu.exe") -Force

# The program of the client is only kept in the program directory of the
# installation.
Remove-Item $workerExe -Force -ErrorAction SilentlyContinue

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

Write-Host ""
Write-Host "İstemciler derlendi:"
Get-ChildItem $binaries | Select-Object Name, Length | Format-Table -AutoSize
