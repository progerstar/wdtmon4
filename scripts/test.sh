#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${script_dir}/common.sh"

require_command go node npm

printf '%s\n' '==> Installing and testing the web interface'
(
    cd "${release_root_dir}/web"
    if [[ "${SKIP_NPM_CI:-0}" != 1 ]]; then
        npm ci --no-audit --no-fund
    fi
    npm test
    npm run build
)

printf '%s\n' '==> Testing and vetting the Go application'
(
    cd "${release_root_dir}"
    go test -race ./...
    go vet ./...
)
