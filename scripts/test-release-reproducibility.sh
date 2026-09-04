#!/usr/bin/env bash
set -euo pipefail

command -v goreleaser >/dev/null 2>&1 || {
    echo "goreleaser is required" >&2
    exit 1
}
goreleaser_version="$(goreleaser --version | awk '/^GitVersion:/ { sub(/^v/, "", $2); print $2 }')"
[[ "$goreleaser_version" == 2.18.0 ]] || {
    echo "goreleaser v2.18.0 is required" >&2
    exit 1
}
go_version="$(awk '$1 == "go" { print $2; exit }' go.mod)"
export GOTOOLCHAIN="go${go_version}"
[[ "$(go env GOVERSION)" == "go${go_version}" ]] || {
    echo "Go ${go_version} is required" >&2
    exit 1
}

go mod tidy -diff

goreleaser check

args=(release --clean)
tag="${HETKI_RELEASE_TAG:-}"
if [[ -n "$tag" ]]; then
    ./scripts/validate-release-tag.sh "$tag"
    [[ "$(git rev-list -n 1 "$tag")" == "$(git rev-parse HEAD)" ]] || {
        echo "release tag does not point at HEAD: $tag" >&2
        exit 1
    }
    export GORELEASER_CURRENT_TAG="$tag"
    args+=(--skip=publish)
else
    args+=(--snapshot)
fi

first="$(mktemp -d)"
trap 'rm -rf "$first"' EXIT

verify_checksums() {
    local names expected
    names="$(awk '{print $2}' dist/checksums.txt | sort)"
    expected=$'hetki-darwin-amd64\nhetki-darwin-arm64\nhetki-linux-amd64\nhetki-linux-arm64'
    [[ "$names" == "$expected" ]] || {
        echo "checksums.txt does not name exactly the four release artifacts" >&2
        exit 1
    }
    if command -v sha256sum >/dev/null 2>&1; then
        (cd dist && sha256sum --check checksums.txt)
    else
        (cd dist && shasum -a 256 --check checksums.txt)
    fi
}

materialize_binaries() {
    local os arch matches source
    for os in darwin linux; do
        for arch in amd64 arm64; do
            # shellcheck disable=SC2206 # intentional glob; GoReleaser paths contain no spaces
            matches=(dist/hetki_${os}_${arch}_*/hetki)
            [[ "${#matches[@]}" -eq 1 && -f "${matches[0]}" ]] || {
                echo "missing unique GoReleaser output for ${os}/${arch}" >&2
                exit 1
            }
            source="${matches[0]}"
            cp "$source" "dist/hetki-${os}-${arch}"
        done
    done
}

artifacts=(
    dist/hetki-darwin-amd64
    dist/hetki-darwin-arm64
    dist/hetki-linux-amd64
    dist/hetki-linux-arm64
    dist/checksums.txt
)

goreleaser "${args[@]}"
materialize_binaries
verify_checksums
cp "${artifacts[@]}" "$first/"
goreleaser "${args[@]}"
materialize_binaries
verify_checksums

for artifact in "$first"/*; do
    current="dist/${artifact##*/}"
    [[ -f "$current" ]] || {
        echo "second build omitted ${artifact##*/}" >&2
        exit 1
    }
    cmp -s "$artifact" "$current" || {
        echo "release artifact is not reproducible: ${artifact##*/}" >&2
        exit 1
    }
done

[[ "$(find "$first" -type f | wc -l)" -eq 5 ]] || {
    echo "expected four binaries and checksums.txt" >&2
    exit 1
}

echo "release artifacts are reproducible"
