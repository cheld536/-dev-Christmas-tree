$ErrorActionPreference = 'Stop'

function Invoke-Gh {
    & gh @args
    if ($LASTEXITCODE -ne 0) { throw 'GitHub CLI 명령에 실패했습니다.' }
}

function Get-Release {
    $id = & gh release view $tag --repo $repo --json databaseId --jq .databaseId 2>&1
    if ($LASTEXITCODE -eq 0) {
        return (Invoke-Gh api "repos/$repo/releases/$id" | ConvertFrom-Json)
    }
    if ("$id" -notmatch '^(release not found|HTTP 404)') { throw "$id" }
}

function Assert-Asset($asset, [string]$path) {
    $file = Get-Item -LiteralPath $path
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash.ToLowerInvariant()
    if ($asset.state -ne 'uploaded' -or $asset.size -ne $file.Length -or $asset.digest -cne "sha256:$hash") {
        throw "GitHub 첨부 파일의 크기 또는 SHA256이 일치하지 않습니다. $($file.Name)"
    }
}

$repo = $env:GITHUB_REPOSITORY
$tag = $env:RELEASE_TAG
if ($tag -cnotmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
    throw '릴리스 태그는 vMAJOR.MINOR.PATCH 형식이어야 합니다.'
}
$restored = @{
    'v1.0.0' = 'DeveloperChristmasTree_v1.0.exe'
    'v2.0.0' = 'DeveloperChristmasTree_v2.0_Setup.exe'
    'v2.1.0' = 'DeveloperChristmasTree_v2.1_Setup.exe'
    'v2.2.0' = 'DeveloperChristmasTree_v2.2_Setup.exe'
}
$isRestored = -not [string]::IsNullOrEmpty($env:RESTORED_TAG)
$checksumName = "$tag-SHA256SUMS.txt"
$release = Get-Release
$directory = 'release-files'
if ($isRestored) {
    if ($env:RESTORED_TAG -cne $tag -or -not $restored.ContainsKey($tag) -or -not $release) {
        throw '복원 버전의 초안 릴리스를 찾을 수 없습니다.'
    }
    $exeName = $restored[$tag]
    Invoke-Gh release download $tag --repo $repo --dir $directory --pattern $exeName --pattern $checksumName --skip-existing
} else {
    $version = $tag.Substring(1) -replace '\.0$', ''
    $exeName = "DeveloperChristmasTree_v${version}_Setup.exe"
}
$tagRef = Invoke-Gh api "repos/$repo/git/ref/tags/$tag" | ConvertFrom-Json
$prerelease = $false
$notes = @('--generate-notes')
if (-not $isRestored -and $tagRef.object.type -ceq 'tag') {
    $annotation = Invoke-Gh api "repos/$repo/git/tags/$($tagRef.object.sha)" --jq .message
    $prerelease = $annotation -ccontains 'Release-Channel: prerelease'
    $body = ($annotation | Where-Object { $_ -cne 'Release-Channel: prerelease' }) -join "`n"
    if ($body.Trim()) { $notes = @('--notes', $body.Trim()) }
}
$exePath = Join-Path $directory $exeName
$checksumPath = Join-Path $directory $checksumName
$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $exePath).Hash.ToLowerInvariant()
if ((Get-Content -Raw -LiteralPath $checksumPath).TrimEnd("`r", "`n") -cne "$hash  $exeName") {
    throw 'EXE의 SHA256이 검사값 파일과 일치하지 않습니다.'
}
if ($isRestored -and "$hash  $exeName" -cnotin (Get-Content -LiteralPath docs/release-checksums.txt)) {
    throw '복원 EXE의 SHA256이 저장소에 기록된 검사값과 일치하지 않습니다.'
}

if (-not $release) {
    $options = @()
    if ($prerelease) { $options += '--prerelease' }
    Invoke-Gh release create $tag --repo $repo --verify-tag --draft --title "Developer Christmas Tree $tag" @options @notes
    $release = Get-Release
}
if ($release.tag_name -cne $tag -or @($release.assets | Where-Object { $_.name -cnotin @($exeName, $checksumName) }).Count -ne 0) {
    throw '릴리스 태그 또는 첨부 파일 목록이 예상과 다릅니다.'
}
foreach ($path in @($exePath, $checksumPath)) {
    $name = Split-Path -Leaf $path
    $asset = $release.assets | Where-Object name -CEQ $name
    if (-not $asset -and $release.draft -and -not $isRestored) {
        Invoke-Gh release upload $tag $path --repo $repo
        $release = Get-Release
        $asset = $release.assets | Where-Object name -CEQ $name
    }
    Assert-Asset $asset $path
}
if ($release.draft) {
    $latest = (-not $prerelease -and (-not $isRestored -or $tag -ceq 'v2.2.0')).ToString().ToLowerInvariant()
    $options = @()
    if (-not $isRestored) { $options += "--prerelease=$($prerelease.ToString().ToLowerInvariant())" }
    Invoke-Gh release edit $tag --repo $repo --draft=false "--latest=$latest" @options
    $release = Get-Release
    if ($release.draft) { throw '릴리스가 아직 초안 상태입니다.' }
}
Write-Output $release.html_url
