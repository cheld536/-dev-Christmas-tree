#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

version="$(tr -d '\r\n' < internal/buildinfo/VERSION)"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo '올바른 버전이 필요합니다.' >&2; exit 1; }
cpu="$(uname -m)"
case "$cpu" in
    arm64) goarch=arm64 ;;
    x86_64) goarch=amd64 ;;
    *) echo "지원하지 않는 macOS 아키텍처입니다. $cpu" >&2; exit 1 ;;
esac

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
app="$work/Developer Christmas Tree.app"
mkdir -p "$app/Contents/"{MacOS,Helpers,Resources} "$work/Tree.iconset" dist
sed "s/@VERSION@/$version/g" macos/Info.plist > "$app/Contents/Info.plist"

GOOS=darwin GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -o "$app/Contents/Helpers/tree-renderer" .
xcrun swiftc -swift-version 5 -O -target "$cpu-apple-macos12.0" \
    macos/ChristmasTreeApp.swift macos/TreeProcess.swift -o "$app/Contents/MacOS/DeveloperChristmasTree"

# 기존 트리 아이콘의 256픽셀 PNG를 macOS 아이콘으로 변환합니다.
python3 - "$work/Tree.png" <<'PY'
from pathlib import Path
import struct
import sys

icon = Path("assets/tree.ico").read_bytes()
count = struct.unpack_from("<H", icon, 4)[0]
for index in range(count):
    width, height, _, _, _, _, length, offset = struct.unpack_from("<BBBBHHII", icon, 6 + 16 * index)
    if width == height == 0 and icon[offset:offset + 8] == b"\x89PNG\r\n\x1a\n":
        Path(sys.argv[1]).write_bytes(icon[offset:offset + length])
        break
else:
    raise SystemExit("트리 아이콘에서 PNG를 찾을 수 없습니다.")
PY
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" "$work/Tree.png" --out "$work/Tree.iconset/icon_${size}x${size}.png" >/dev/null
    double=$((size * 2))
    sips -z "$double" "$double" "$work/Tree.png" --out "$work/Tree.iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$work/Tree.iconset" -o "$app/Contents/Resources/Tree.icns"

codesign --force --sign - "$app/Contents/Helpers/tree-renderer"
codesign --force --sign - "$app"
codesign --verify --deep --strict "$app"
zip="DeveloperChristmasTree_v${version}_macOS-${cpu}.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "dist/$zip"
(cd dist && shasum -a 256 "$zip" > "${zip%.zip}-SHA256SUMS.txt")
printf '%s\n' "dist/$zip"
