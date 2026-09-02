#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${script_dir}/common.sh"

if [[ "$(uname -s)" != Linux ]]; then
    printf '%s\n' 'Linux AppImages must be built on Linux.' >&2
    exit 2
fi

case "$(uname -m)" in
    x86_64|amd64)
        go_arch=amd64
        appimage_arch=x86_64
        ;;
    aarch64|arm64)
        go_arch=arm64
        appimage_arch=aarch64
        ;;
    *)
        printf 'Unsupported Linux architecture: %s\n' "$(uname -m)" >&2
        exit 2
        ;;
esac

require_command file go install
version=$(app_version)
build_root=${BUILD_ROOT:-"${release_root_dir}/build/linux-${appimage_arch}"}
dist_root=${1:-"${release_root_dir}/dist/linux"}

appimagetool_candidate=${APPIMAGETOOL:-appimagetool}
if [[ "${appimagetool_candidate}" == */* ]]; then
    [[ -x "${appimagetool_candidate}" ]] || {
        printf 'appimagetool is not executable: %s\n' "${appimagetool_candidate}" >&2
        exit 2
    }
    appimagetool_bin=${appimagetool_candidate}
else
    appimagetool_bin=$(command -v "${appimagetool_candidate}" || true)
    [[ -n "${appimagetool_bin}" ]] || {
        printf '%s\n' 'AppImage packaging requires appimagetool.' >&2
        printf '%s\n' 'Install it or set APPIMAGETOOL=/absolute/path/to/appimagetool.' >&2
        exit 2
    }
fi

appimagetool_args=()
if [[ -n "${APPIMAGE_RUNTIME_FILE:-}" ]]; then
    [[ -f "${APPIMAGE_RUNTIME_FILE}" ]] || {
        printf 'AppImage runtime does not exist: %s\n' "${APPIMAGE_RUNTIME_FILE}" >&2
        exit 2
    }
    appimagetool_args+=(--runtime-file "${APPIMAGE_RUNTIME_FILE}")
fi

[[ -f "${WDTMON4_ICON_PNG}" ]] || {
    printf 'Application icon was not found: %s\n' "${WDTMON4_ICON_PNG}" >&2
    exit 1
}

if [[ "${SKIP_WEB_BUILD:-0}" != 1 ]]; then
    printf '%s\n' '==> Building the web interface'
    build_web
fi

mkdir -p "${build_root}" "${dist_root}"
binary="${build_root}/${WDTMON4_NAME}"
printf '==> Building %s %s for linux/%s\n' \
    "${WDTMON4_NAME}" "${version}" "${go_arch}"
(
    cd "${release_root_dir}"
    CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
        go build -trimpath -buildvcs=false \
        -ldflags "-s -w -X main.VERSION=${version}" \
        -o "${binary}" .
)

[[ -x "${binary}" ]] || {
    printf 'Linux binary was not created: %s\n' "${binary}" >&2
    exit 1
}
expected_version="${WDTMON4_NAME} v${version}"
actual_version=$("${binary}" --version)
[[ "${actual_version}" == "${expected_version}" ]] || {
    printf 'Unexpected version output: %s\n' "${actual_version}" >&2
    exit 1
}

package_dir=$(mktemp -d "${TMPDIR:-/tmp}/wdtmon4-appdir.XXXXXX")
image_tmp=
cleanup()
{
    if [[ -n "${package_dir:-}" && "${package_dir}" == *wdtmon4-appdir.* ]]; then
        rm -rf -- "${package_dir}"
    fi
    if [[ -n "${image_tmp:-}" && -f "${image_tmp}" ]]; then
        rm -f -- "${image_tmp}"
    fi
}
trap cleanup EXIT

mkdir -p \
    "${package_dir}/usr/bin" \
    "${package_dir}/usr/share/applications" \
    "${package_dir}/usr/share/doc/${WDTMON4_NAME}" \
    "${package_dir}/usr/share/icons/hicolor/512x512/apps"
install -m 0755 "${binary}" "${package_dir}/usr/bin/${WDTMON4_NAME}"
install -m 0644 "${WDTMON4_ICON_PNG}" "${package_dir}/${WDTMON4_NAME}.png"
install -m 0644 "${WDTMON4_ICON_PNG}" \
    "${package_dir}/usr/share/icons/hicolor/512x512/apps/${WDTMON4_NAME}.png"
install -m 0644 "${release_root_dir}/LICENSE" \
    "${package_dir}/usr/share/doc/${WDTMON4_NAME}/LICENSE"
ln -s "${WDTMON4_NAME}.png" "${package_dir}/.DirIcon"

{
    printf '%s\n' '#!/bin/sh'
    printf '%s\n' 'set -e'
    printf '%s\n' 'app_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)'
    printf 'exec "${app_dir}/usr/bin/%s" "$@"\n' "${WDTMON4_NAME}"
} > "${package_dir}/AppRun"
chmod 0755 "${package_dir}/AppRun"

desktop_file="${package_dir}/${WDTMON4_NAME}.desktop"
{
    printf '%s\n' '[Desktop Entry]'
    printf '%s\n' 'Type=Application'
    printf 'Name=%s\n' "${WDTMON4_DISPLAY_NAME}"
    printf 'Comment=%s\n' "${WDTMON4_DESCRIPTION}"
    printf 'Exec=%s\n' "${WDTMON4_NAME}"
    printf 'Icon=%s\n' "${WDTMON4_NAME}"
    printf '%s\n' 'Categories=Utility;'
    printf '%s\n' 'Terminal=false'
    printf '%s\n' 'StartupNotify=true'
} > "${desktop_file}"
chmod 0644 "${desktop_file}"
cp "${desktop_file}" "${package_dir}/usr/share/applications/"
if command -v desktop-file-validate >/dev/null 2>&1; then
    desktop-file-validate "${desktop_file}"
fi

image="${dist_root}/${WDTMON4_NAME}-${version}-linux-${appimage_arch}.AppImage"
image_tmp="${dist_root}/.${WDTMON4_NAME}-${version}-linux-${appimage_arch}.AppImage"
rm -f -- "${image_tmp}"
(
    export ARCH=${appimage_arch}
    export APPIMAGE_EXTRACT_AND_RUN=${APPIMAGE_EXTRACT_AND_RUN:-1}
    export VERSION=${version}
    "${appimagetool_bin}" "${appimagetool_args[@]}" \
        "${package_dir}" "${image_tmp}"
)
[[ -f "${image_tmp}" ]] || {
    printf 'appimagetool did not create %s\n' "${image_tmp}" >&2
    exit 1
}
chmod 0755 "${image_tmp}"
actual_version=$(APPIMAGE_EXTRACT_AND_RUN=1 "${image_tmp}" --version)
[[ "${actual_version}" == "${expected_version}" ]] || {
    printf 'Unexpected AppImage version output: %s\n' "${actual_version}" >&2
    exit 1
}
mv -f -- "${image_tmp}" "${image}"
image_tmp=

file "${image}"
printf 'Created %s\n' "${image}"
write_sha256 "${image}"

rm -rf -- "${package_dir}"
package_dir=
trap - EXIT
