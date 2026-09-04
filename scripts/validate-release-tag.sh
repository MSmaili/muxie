#!/usr/bin/env bash
set -euo pipefail

valid_release_tag() {
    local tag="$1" prerelease identifier
    [[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$ ]] || return 1
    prerelease="${BASH_REMATCH[5]:-}"
    [[ -z "$prerelease" ]] && return 0
    while IFS= read -r identifier; do
        if [[ "$identifier" =~ ^[0-9]+$ && ${#identifier} -gt 1 && "$identifier" == 0* ]]; then
            return 1
        fi
    done < <(printf '%s\n' "$prerelease" | tr '.' '\n')
}

self_test() {
    local tag
    for tag in v0.0.0 v1.2.3 v1.2.3-rc.1 v1.2.3-alpha-beta; do
        valid_release_tag "$tag" || return 1
    done
    for tag in 1.2.3 v1.2 v01.2.3 v1.2.3-01 v1.2.3-a..b v1.2.3-. v1.2.3-a. v1.2.3+build; do
        ! valid_release_tag "$tag" || return 1
    done
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    if [[ "${1:-}" == --self-test ]]; then
        self_test
    elif [[ $# -eq 1 ]] && valid_release_tag "$1"; then
        exit 0
    else
        echo "invalid release tag: ${1:-<missing>}" >&2
        exit 1
    fi
fi
