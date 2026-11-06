# PowerShell release build script for Windows/cross-platform.
# Usage: .\scripts\build-release.ps1 [version]
param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

if (-not $Version) {
    $Version = git describe --tags --abbrev=0 2>$null
    if (-not $Version) {
        $Version = "0.0.0"
    }
}
$Version = $Version -replace '^v', ''

$Pkg = "github.com/HoangP8/tokless/internal/util"
$Out = "dist/release"

if (Test-Path $Out) {
    Remove-Item -Recurse -Force $Out
}
New-Item -ItemType Directory -Force -Path $Out | Out-Null

$Targets = @(
    @{ Asset = "tokless-linux-x64";        OS = "linux";   Arch = "amd64"; Extra = "" },
    @{ Asset = "tokless-linux-arm64";      OS = "linux";   Arch = "arm64"; Extra = "" },
    @{ Asset = "tokless-darwin-x64";       OS = "darwin";  Arch = "amd64"; Extra = "" },
    @{ Asset = "tokless-darwin-arm64";     OS = "darwin";  Arch = "arm64"; Extra = "" },
    @{ Asset = "tokless-windows-x64.exe";  OS = "windows"; Arch = "amd64"; Extra = "" },
    @{ Asset = "toklessw-windows-x64.exe"; OS = "windows"; Arch = "amd64"; Extra = "-H windowsgui" }
)

foreach ($t in $Targets) {
    $asset = $t.Asset
    $goos = $t.OS
    $goarch = $t.Arch
    $extra = $t.Extra
    Write-Host "compiling $asset ($goos/$goarch) ... " -NoNewline
    $env:CGO_ENABLED = "0"
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $ldflags = "-s -w -X ${Pkg}.Version=${Version}"
    if ($extra) {
        $ldflags = "$extra $ldflags"
    }
    go build -trimpath -ldflags $ldflags -o "$Out/$asset" ./cmd/tokless
    Write-Host "ok"
}

Write-Host "`nBuilt binaries for v${Version} in ${Out}/:"
Get-ChildItem $Out | Format-Table Name, Length, LastWriteTime
