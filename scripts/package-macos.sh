#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${script_dir}/common.sh"

if [[ "$(uname -s)" != Darwin ]]; then
    printf '%s\n' 'macOS DMGs must be built on macOS.' >&2
    exit 2
fi

require_command clang codesign ditto go hdiutil iconutil lipo npm node otool plutil shasum sips
version=$(app_version)
dist_root=${1:-"${release_root_dir}/dist/macos"}
mkdir -p "${dist_root}"

[[ -f "${WDTMON4_ICON_PNG}" ]] || {
    printf 'Application icon was not found: %s\n' "${WDTMON4_ICON_PNG}" >&2
    exit 1
}

if [[ "${SKIP_WEB_BUILD:-0}" != 1 ]]; then
    printf '%s\n' '==> Building the web interface'
    build_web
fi

build_dir=$(mktemp -d "${TMPDIR:-/tmp}/wdtmon4-macos.XXXXXX")
mount_dir="${build_dir}/mounted"
mounted=0
image_tmp=

detach_image()
{
    local attempt

    for attempt in 1 2 3; do
        if hdiutil detach "${mount_dir}" -quiet; then
            mounted=0
            return 0
        fi
        sleep 1
    done
    if hdiutil detach "${mount_dir}" -force -quiet; then
        mounted=0
        return 0
    fi
    return 1
}

cleanup()
{
    if [[ "${mounted}" == 1 ]]; then
        detach_image || true
    fi
    if [[ -n "${build_dir:-}" && "${build_dir}" == *wdtmon4-macos.* ]]; then
        rm -rf -- "${build_dir}"
    fi
    if [[ -n "${image_tmp:-}" && -f "${image_tmp}" ]]; then
        rm -f -- "${image_tmp}"
    fi
}
trap cleanup EXIT

build_arch()
{
    local go_arch=$1
    local clang_arch=$2
    local output=$3

    (
        cd "${release_root_dir}"
        env \
            MACOSX_DEPLOYMENT_TARGET="${WDTMON4_MACOS_MIN_VERSION}" \
            CGO_ENABLED=1 \
            GOOS=darwin \
            GOARCH="${go_arch}" \
            CC="clang -arch ${clang_arch}" \
            go build -trimpath -buildvcs=false \
            -ldflags "-s -w -X main.VERSION=${version}" \
            -o "${output}" .
    )
}

amd64_binary="${build_dir}/${WDTMON4_NAME}-amd64"
arm64_binary="${build_dir}/${WDTMON4_NAME}-arm64"
universal_binary="${build_dir}/${WDTMON4_NAME}"

printf '==> Building %s %s for Intel and Apple Silicon\n' \
    "${WDTMON4_NAME}" "${version}"
build_arch amd64 x86_64 "${amd64_binary}"
build_arch arm64 arm64 "${arm64_binary}"
lipo -create -output "${universal_binary}" "${amd64_binary}" "${arm64_binary}"

architectures=$(lipo -archs "${universal_binary}")
for required_arch in x86_64 arm64; do
    if [[ " ${architectures} " != *" ${required_arch} "* ]]; then
        printf 'Universal binary is missing %s: %s\n' \
            "${required_arch}" "${architectures}" >&2
        exit 1
    fi
done

bundle="${build_dir}/${WDTMON4_NAME}.app"
bundle_executable="${bundle}/Contents/MacOS/${WDTMON4_NAME}"
resources_dir="${bundle}/Contents/Resources"
mkdir -p "$(dirname -- "${bundle_executable}")" "${resources_dir}"
install -m 0755 "${universal_binary}" "${bundle_executable}"

cat > "${bundle}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>ru</string>
    <key>CFBundleDisplayName</key>
    <string>${WDTMON4_DISPLAY_NAME}</string>
    <key>CFBundleExecutable</key>
    <string>${WDTMON4_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>${WDTMON4_NAME}.icns</string>
    <key>CFBundleIdentifier</key>
    <string>${WDTMON4_BUNDLE_ID}</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>${WDTMON4_DISPLAY_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>${version}</string>
    <key>CFBundleVersion</key>
    <string>${version}</string>
    <key>LSApplicationCategoryType</key>
    <string>public.app-category.utilities</string>
    <key>LSMinimumSystemVersion</key>
    <string>${WDTMON4_MACOS_MIN_VERSION}</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSHumanReadableCopyright</key>
    <string>UnitX</string>
</dict>
</plist>
EOF
plutil -lint "${bundle}/Contents/Info.plist" >/dev/null

iconset="${build_dir}/${WDTMON4_NAME}.iconset"
mkdir -p "${iconset}"
sips -z 16 16 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_16x16.png" >/dev/null
sips -z 32 32 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_16x16@2x.png" >/dev/null
sips -z 32 32 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_32x32.png" >/dev/null
sips -z 64 64 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_32x32@2x.png" >/dev/null
sips -z 128 128 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_128x128.png" >/dev/null
sips -z 256 256 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_256x256.png" >/dev/null
sips -z 512 512 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_256x256@2x.png" >/dev/null
sips -z 512 512 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_512x512.png" >/dev/null
sips -z 1024 1024 "${WDTMON4_ICON_PNG}" --out "${iconset}/icon_512x512@2x.png" >/dev/null
iconutil -c icns "${iconset}" -o "${resources_dir}/${WDTMON4_NAME}.icns"

for arch in x86_64 arm64; do
    actual_min_version=$(otool -l -arch "${arch}" "${bundle_executable}" | \
        awk '$1 == "minos" {print $2; exit}')
    if [[ "${actual_min_version}" != "${WDTMON4_MACOS_MIN_VERSION}" ]]; then
        printf 'Minimum macOS version for %s is %s; expected %s.\n' \
            "${arch}" "${actual_min_version:-<empty>}" \
            "${WDTMON4_MACOS_MIN_VERSION}" >&2
        exit 1
    fi
done

if otool -L "${bundle_executable}" | grep -E '/(usr/local|opt/homebrew)/' >/dev/null; then
    printf '%s\n' 'The application depends on an unbundled Homebrew library.' >&2
    exit 1
fi

codesign_identity=${MACOS_CODESIGN_IDENTITY:--}
codesign_args=(--force --deep --sign "${codesign_identity}")
if [[ "${codesign_identity}" != - ]]; then
    codesign_args+=(--options runtime --timestamp)
fi
codesign "${codesign_args[@]}" "${bundle}"
codesign --verify --deep --strict --verbose=2 "${bundle}"

dmg_root="${build_dir}/dmg-root"
mkdir -p "${dmg_root}"
ditto "${bundle}" "${dmg_root}/${WDTMON4_NAME}.app"
ln -s /Applications "${dmg_root}/Applications"

image_name="${WDTMON4_NAME}-${version}-macos-universal.dmg"
image="${dist_root}/${image_name}"
image_tmp="${dist_root}/.${image_name}"
rm -f -- "${image_tmp}"
hdiutil create -quiet -ov -format UDZO \
    -volname "${WDTMON4_DISPLAY_NAME} ${version}" \
    -srcfolder "${dmg_root}" "${image_tmp}"

dmg_codesign_args=(--force --sign "${codesign_identity}")
if [[ "${codesign_identity}" != - ]]; then
    dmg_codesign_args+=(--timestamp)
fi
codesign "${dmg_codesign_args[@]}" "${image_tmp}"
codesign --verify --verbose=2 "${image_tmp}"
hdiutil verify -quiet "${image_tmp}"

mkdir -p "${mount_dir}"
hdiutil attach "${image_tmp}" -readonly -nobrowse -mountpoint "${mount_dir}" -quiet
mounted=1
mounted_app="${mount_dir}/${WDTMON4_NAME}.app"
codesign --verify --deep --strict --verbose=2 "${mounted_app}"
actual_version=$("${mounted_app}/Contents/MacOS/${WDTMON4_NAME}" --version)
expected_version="${WDTMON4_NAME} v${version}"
[[ "${actual_version}" == "${expected_version}" ]] || {
    printf 'Unexpected version output: %s\n' "${actual_version}" >&2
    exit 1
}
detach_image

mv -f -- "${image_tmp}" "${image}"
image_tmp=
printf 'Created %s (%s)\n' "${image}" "${architectures}"
write_sha256 "${image}"

rm -rf -- "${build_dir}"
build_dir=
trap - EXIT
