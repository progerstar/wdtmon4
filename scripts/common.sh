#!/usr/bin/env bash

if ((BASH_VERSINFO[0] < 3)); then
    printf '%s\n' 'wdtmon4 packaging scripts require Bash 3 or newer.' >&2
    if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
        return 2
    fi
    exit 2
fi

release_script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
release_root_dir=$(cd -- "${release_script_dir}/.." && pwd)

readonly WDTMON4_NAME=wdtmon4
readonly WDTMON4_DISPLAY_NAME='wdtmon4'
readonly WDTMON4_DESCRIPTION='Monitor and configure UnitX USB watchdogs'
readonly WDTMON4_BUNDLE_ID='ru.unitx.wdtmon4'
readonly WDTMON4_MACOS_MIN_VERSION='12.0'
readonly WDTMON4_ICON_PNG="${release_root_dir}/web/public/android-chrome-512x512.png"

require_command()
{
    local command_name

    for command_name in "$@"; do
        if ! command -v "${command_name}" >/dev/null 2>&1; then
            printf 'Required command was not found: %s\n' "${command_name}" >&2
            return 2
        fi
    done
}

app_version()
{
    local version

    version=$(sed -nE \
        's/^[[:space:]]*var[[:space:]]+VERSION[[:space:]]*=[[:space:]]*"([^"]+)".*/\1/p' \
        "${release_root_dir}/main.go" | head -n 1)
    if [[ ! "${version}" =~ ^[0-9]+(\.[0-9]+){1,2}$ ]]; then
        printf 'Invalid application version in main.go: %s\n' \
            "${version:-<empty>}" >&2
        return 1
    fi

    printf '%s\n' "${version}"
}

build_web()
{
    require_command node npm
    (
        cd "${release_root_dir}/web"
        if [[ "${SKIP_NPM_CI:-0}" != 1 ]]; then
            npm ci --no-audit --no-fund
        fi
        npm run build
    )

    [[ -f "${release_root_dir}/web/build/index.html" ]] || {
        printf '%s\n' 'Vite did not create web/build/index.html.' >&2
        return 1
    }
}

write_sha256()
{
    local artifact=$1
    local artifact_dir artifact_name

    [[ -f "${artifact}" ]] || {
        printf 'Cannot checksum missing artifact: %s\n' "${artifact}" >&2
        return 1
    }

    artifact_dir=$(cd -- "$(dirname -- "${artifact}")" && pwd)
    artifact_name=$(basename -- "${artifact}")
    (
        cd "${artifact_dir}"
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum "${artifact_name}" > "${artifact_name}.sha256"
        elif command -v shasum >/dev/null 2>&1; then
            shasum -a 256 "${artifact_name}" > "${artifact_name}.sha256"
        else
            printf '%s\n' 'Neither sha256sum nor shasum is available.' >&2
            return 2
        fi
    )
    printf 'Created %s.sha256\n' "${artifact}"
}
