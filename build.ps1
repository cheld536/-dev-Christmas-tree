[CmdletBinding()]
param(
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    if (-not $PSBoundParameters.ContainsKey('Version')) {
        $Version = (Get-Content -Raw -LiteralPath internal/buildinfo/VERSION).Trim()
    }
    if ($Version -cnotmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
        throw '버전은 MAJOR.MINOR.PATCH 형식이어야 합니다.'
    }
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -ico assets/tree.ico -o branding_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw 'Windows 아이콘 리소스를 만들 수 없습니다.' }
    Copy-Item -LiteralPath branding_windows_amd64.syso -Destination setup/branding_windows_amd64.syso -Force
    $flags = "-s -w -H windowsgui -X christmas/internal/buildinfo.override=$Version"
    go build -trimpath -ldflags $flags -o setup/app.exe .
    if ($LASTEXITCODE -ne 0) { throw 'App build failed' }
    go test -v -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    New-Item -ItemType Directory -Force dist | Out-Null
    $label = $Version -replace '\.0$', ''
    $filename = "DeveloperChristmasTree_v${label}_Setup.exe"
    $path = Join-Path dist $filename
    go build -trimpath -ldflags $flags -o $path ./setup
    if ($LASTEXITCODE -ne 0) { throw 'Setup build failed' }
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash.ToLowerInvariant()
    Set-Content -Encoding ascii -LiteralPath (Join-Path dist "v$Version-SHA256SUMS.txt") -Value "$hash  $filename"
    Write-Output "$hash  $filename"
} finally { Pop-Location }
