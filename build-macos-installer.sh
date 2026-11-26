#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
    echo 'Apple Silicon과 Intel 앱 ZIP을 순서대로 지정하세요.' >&2
    exit 2
fi
arm_zip="$1"
intel_zip="$2"
repo="$(cd "$(dirname "$0")" && pwd)"
version="$(tr -d '\r\n' < "$repo/internal/buildinfo/VERSION")"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo '올바른 버전이 필요합니다.' >&2; exit 1; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
app_name='Developer Christmas Tree.app'
ditto -x -k "$arm_zip" "$work/arm"
ditto -x -k "$intel_zip" "$work/intel"
arm="$work/arm/$app_name"
intel="$work/intel/$app_name"
for bundle in "$arm" "$intel"; do
    codesign --verify --deep --strict "$bundle"
    actual="$(/usr/libexec/PlistBuddy -c 'Print CFBundleShortVersionString' "$bundle/Contents/Info.plist")"
    [[ "$version" == "$actual" ]] || { echo 'macOS 앱과 저장소의 버전이 다릅니다.' >&2; exit 1; }
done

app="$work/$app_name"
ditto "$arm" "$app"
for binary in MacOS/DeveloperChristmasTree Helpers/tree-renderer; do
    lipo -verify_arch arm64 "$arm/Contents/$binary"
    lipo -verify_arch x86_64 "$intel/Contents/$binary"
    lipo -create "$arm/Contents/$binary" "$intel/Contents/$binary" -output "$app/Contents/$binary"
    chmod 755 "$app/Contents/$binary"
    lipo -verify_arch arm64 x86_64 "$app/Contents/$binary"
done

app_identity="${DCT_MAC_APP_IDENTITY:-}"
installer_identity="${DCT_MAC_INSTALLER_IDENTITY:-}"
notary_profile="${DCT_MAC_NOTARY_PROFILE:-}"
sign=(--force --sign -)
product_sign=()
label=preview
if [[ -n "$app_identity$installer_identity$notary_profile" ]]; then
    if [[ -z "$app_identity" || -z "$installer_identity" || -z "$notary_profile" ]]; then
        echo '배포 서명에는 Developer ID Application, Installer, Apple 공증 프로필이 모두 필요합니다.' >&2
        exit 2
    fi
    sign=(--force --options runtime --timestamp --sign "$app_identity")
    product_sign=(--sign "$installer_identity")
    label=notarized
fi
codesign "${sign[@]}" "$app/Contents/Helpers/tree-renderer"
codesign "${sign[@]}" "$app"
codesign --verify --deep --strict "$app"

name="DeveloperChristmasTree_v${version}_macOS-universal-${label}"
pkg="$work/$name.pkg"
if [[ "$label" == notarized ]]; then
    productbuild "${product_sign[@]}" --component "$app" /Applications "$pkg"
    xcrun notarytool submit "$pkg" --keychain-profile "$notary_profile" --wait --output-format json > "$work/notary.json"
    status="$(plutil -extract status raw -o - "$work/notary.json")"
    [[ "$status" == Accepted ]] || { echo "Apple 공증 상태: $status" >&2; exit 1; }
    xcrun stapler staple "$pkg"
    xcrun stapler validate "$pkg"
    pkgutil --check-signature "$pkg"
else
    productbuild --component "$app" /Applications "$pkg"
fi

mkdir -p "$repo/dist"
mv "$pkg" "$repo/dist/"
(cd "$repo/dist" && shasum -a 256 "$name.pkg" > "$name-SHA256SUMS.txt")
printf '%s\n' "dist/$name.pkg"
