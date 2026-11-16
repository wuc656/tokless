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

$Syso = "cmd/tokless/rsrc_windows_amd64.syso"

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
    if ($goos -eq "windows") {
        $ldflags = "-X ${Pkg}.Version=${Version}"
        if ($extra) {
            $ldflags = "$extra $ldflags"
        }
        & {
            $prevEA = $ErrorActionPreference
            $ErrorActionPreference = "Continue"
            go run github.com/tc-hib/go-winres@v0.3.3 make --arch $goarch --in cmd/tokless/winres/winres.json --out cmd/tokless/rsrc --product-version $Version --file-version $Version 2>&1 | Out-Null
            $ErrorActionPreference = $prevEA
        }
    } elseif ($extra) {
        $ldflags = "$extra $ldflags"
    }
    go build -trimpath -ldflags $ldflags -o "$Out/$asset" ./cmd/tokless
    if (Test-Path $Syso) {
        Remove-Item -Force $Syso
    }
    Write-Host "ok"
}

# Generate SHA256SUMS
$lines = @()
Get-ChildItem -Path $Out -File | Where-Object { $_.Name -like "tokless*" } | Sort-Object Name | ForEach-Object {
    $hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    $lines += "$hash  $($_.Name)"
}
$lines | Out-File -FilePath "$Out/SHA256SUMS" -Encoding ascii

Write-Host "`nBuilt binaries for v${Version} in ${Out}/:"
Get-ChildItem $Out | Format-Table Name, Length, LastWriteTime
