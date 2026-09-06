#!/usr/bin/env bash
set -euo pipefail

readonly GREMLINS_VERSION="v0.6.0"
readonly TOOL_DIR="${XDG_CACHE_HOME:-${HOME}/.cache}/traicr/gremlins/${GREMLINS_VERSION}"
readonly GREMLINS="${TOOL_DIR}/gremlins"

usage() {
  cat >&2 <<'EOF'
usage:
  scripts/mutation.sh changed <git-ref> [report.json]
  scripts/mutation.sh full [report.json]
  scripts/mutation.sh scope <directory> [report.json]
EOF
  exit 2
}

if [[ ! -x "${GREMLINS}" ]]; then
  mkdir -p "${TOOL_DIR}"
  GOBIN="${TOOL_DIR}" go install "github.com/go-gremlins/gremlins/cmd/gremlins@${GREMLINS_VERSION}"
fi

mode="${1:-}"
case "${mode}" in
  changed)
    [[ $# -ge 2 && $# -le 3 ]] || usage
    base_ref="$2"
    report="${3:-artifacts/mutation-changed.json}"
    target="."
    extra=(--diff "${base_ref}")
    ;;
  full)
    [[ $# -le 2 ]] || usage
    report="${2:-artifacts/mutation-full.json}"
    target="."
    extra=()
    ;;
  scope)
    [[ $# -ge 2 && $# -le 3 ]] || usage
    target="$2"
    report="${3:-artifacts/mutation-scope.json}"
    extra=()
    ;;
  *)
    usage
    ;;
esac

mkdir -p "$(dirname "${report}")"
log="$(mktemp)"
trap 'rm -f "${log}"' EXIT
"${GREMLINS}" unleash "${target}" \
  --workers "${GREMLINS_WORKERS:-2}" \
  --timeout-coefficient "${GREMLINS_TIMEOUT_COEFFICIENT:-20}" \
  --output "${report}" \
  --output-statuses lctv \
  "${extra[@]}" 2>&1 | tee "${log}"

if grep -q '^ERROR:' "${log}"; then
  exit 1
fi

if [[ -f "${report}" ]]; then
  printf 'mutation report: %s\n' "${report}"
else
  printf 'no mutable expressions found; no report written\n'
fi
