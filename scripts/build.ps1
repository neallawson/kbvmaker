# Build kbvmaker-gui on Windows. Linux and macOS use ./scripts/build.
# -SkipFFmpegCheck is for a build machine that will not run the program.
# The executable still needs ffmpeg and ffprobe on PATH when it renders.
param(
    [switch]$SkipFFmpegCheck
)

$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") {
    Write-Host "On Linux and macOS, run ./scripts/build"
    exit 1
}

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$missing = @()
$cmds = @("go", "pnpm")
if (-not $SkipFFmpegCheck) {
    $cmds += @("ffmpeg", "ffprobe")
}
foreach ($cmd in $cmds) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        $missing += $cmd
    }
}
if ($SkipFFmpegCheck) {
    Write-Host "ffmpeg was not checked. Rendering still needs ffmpeg and ffprobe on PATH."
}
if ($missing.Count -gt 0) {
    Write-Host ("Missing programs: " + ($missing -join ", ") + ".")
    Write-Host "Install them and run this script again."
    exit 1
}

if (-not (Test-Path "frontend/node_modules")) {
    Write-Host "Installing frontend dependencies"
    pnpm --dir frontend install --frozen-lockfile
}

Write-Host "Using the Windows WebView2 runtime"
Write-Host "Building the client"
pnpm --dir frontend build

$stage = "cmd/desktop/client"
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
Copy-Item -Recurse frontend/build $stage
try {
    Write-Host "Building kbvmaker-gui.exe"
    go build -tags production -o kbvmaker-gui.exe ./cmd/desktop
    go test -tags production -count=1 -run TestClientBuild ./cmd/desktop
    Write-Host "Built .\kbvmaker-gui.exe"
} finally {
    if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
}
