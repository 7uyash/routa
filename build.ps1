# Routa Multi-Platform PowerShell Build Script
param (
    [string]$OutputDir = "bin"
)

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

$targets = @(
    @{ GOOS = "darwin";  GOARCH = "arm64"; Out = "routa-darwin-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; Out = "routa-darwin-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Out = "routa-linux-arm64" },
    @{ GOOS = "linux";   GOARCH = "amd64"; Out = "routa-linux-amd64" },
    @{ GOOS = "windows"; GOARCH = "amd64"; Out = "routa-windows-amd64.exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Out = "routa-windows-arm64.exe" }
)

Write-Host "Building Routa cross-platform binaries..." -ForegroundColor Cyan

foreach ($target in $targets) {
    $outPath = Join-Path $OutputDir $target.Out
    Write-Host "-> Building $($target.GOOS)/$($target.GOARCH) => $outPath" -ForegroundColor Yellow
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    go build -ldflags="-s -w" -o $outPath ./cmd/routa
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed building for $($target.GOOS)/$($target.GOARCH)" -ForegroundColor Red
        exit 1
    }
}

# Reset env vars to local defaults
$env:GOOS = ""
$env:GOARCH = ""

Write-Host "All multi-platform binaries built successfully in ./$OutputDir!" -ForegroundColor Green
