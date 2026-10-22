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
go build -trimpath -ldflags $ldflags -o (Join-Path $binaries "BYOG Worker.exe") ./worker/desktop

# The scanners of the systems trust a program that is signed with the
# certificate of the installation. The signature is written when the
# certificate is on the computer, an unsigned build stays usable.
function Sign-Program([string]$Path) {
    if (-not $env:BYOG_SIGN_CERT -or -not (Test-Path $env:BYOG_SIGN_CERT)) {
        return
    }
    $signTool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($null -eq $signTool) {
        Write-Host "signtool bulunamadi, program imzalanmadi: $Path"
        return
    }
    $arguments = @("sign", "/fd", "SHA256", "/tr", "http://timestamp.digicert.com", "/td", "SHA256", "/f", $env:BYOG_SIGN_CERT)
    if ($env:BYOG_SIGN_PASSWORD) {
        $arguments += @("/p", $env:BYOG_SIGN_PASSWORD)
    }
    $arguments += $Path
    & $signTool.Source @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Program imzalanamadi: $Path"
    }
}

Sign-Program (Join-Path $binaries "BYOG Worker.exe")

# The icon of the installation is the picture of the logo.
$icon = Join-Path $root "worker\desktop\assets\byog.ico"
if (-not (Test-Path $icon)) {
    Write-Host "Ikon bulunamadi: $icon"
}

# The symbols of the program are kept in the clients of Linux. The stripped
# program of Go looks like packed malware to some scanners of the systems, the
# symbols give the traces of a crash as well.
Write-Host "BYOG Worker (Linux) derleniyor..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -trimpath -o (Join-Path $binaries "byog-worker-linux-amd64") ./worker/desktop

# The small computers of the Raspberry family use the processors of ARM. The
# two images of the systems cover the 64-bit systems and the older 32-bit
# systems.
Write-Host "BYOG Worker (Linux ARM64) derleniyor..."
$env:GOARCH = "arm64"
go build -trimpath -o (Join-Path $binaries "byog-worker-linux-arm64") ./worker/desktop

Write-Host "BYOG Worker (Linux ARM 32-bit) derleniyor..."
$env:GOARCH = "arm"
$env:GOARM = "7"
go build -trimpath -o (Join-Path $binaries "byog-worker-linux-arm") ./worker/desktop
Remove-Item Env:GOARM

Write-Host "BYOG Worker (macOS) derleniyor..."
$env:GOOS = "darwin"
$env:GOARCH = "arm64"
go build -trimpath -ldflags "-s -w" -o (Join-Path $binaries "byog-worker-darwin-arm64") ./worker/desktop

Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED

if (-not $SkipMobile) {
    Write-Host "BYOG Worker (Android / Termux) derleniyor..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    go build -trimpath -ldflags "-s -w" -o (Join-Path $binaries "byog-worker-android-arm64") ./worker/mobile
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
        Sign-Program (Join-Path $binaries "BYOG Worker Kurulumu.exe")
    }
}

Write-Host ""
Write-Host "İstemciler derlendi:"
Get-ChildItem $binaries | Select-Object Name, Length | Format-Table -AutoSize
