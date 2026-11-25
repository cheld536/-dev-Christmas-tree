$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    go build -trimpath -ldflags '-s -w -H windowsgui' -o setup/app.exe .
    if ($LASTEXITCODE -ne 0) { throw 'App build failed' }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    New-Item -ItemType Directory -Force dist | Out-Null
    go build -trimpath -ldflags '-s -w -H windowsgui' -o dist/DeveloperChristmasTree_v2.2_Setup.exe ./setup
    if ($LASTEXITCODE -ne 0) { throw 'Setup build failed' }
    Get-FileHash dist/DeveloperChristmasTree_v2.2_Setup.exe -Algorithm SHA256
} finally { Pop-Location }
