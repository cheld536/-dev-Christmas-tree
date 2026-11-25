[CmdletBinding()]
param(
    [ValidatePattern('^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Version = '2.2.0'
)

$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    $flags = "-s -w -H windowsgui -X main.version=$Version"
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
