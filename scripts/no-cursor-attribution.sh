#!/usr/bin/env bash
# Shared Cursor-attribution helpers for .githooks and CI.
set -euo pipefail

REWRITE_AUTHOR_NAME="regutierrez"
REWRITE_AUTHOR_EMAIL="rpegutierrez@gmail.com"

# Lines stripped from commit messages. Keep in sync with CI text checks.
cursor_attribution_line_ere() {
  printf '%s' \
    'Co-authored-by:.*[Cc]ursor' \
    '|Made-with:[[:space:]]*Cursor' \
    '|Made with Cursor' \
    '|cursoragent@cursor.com'
}

strip_cursor_attribution_file() {
  local src="$1"
  local dest="${2:-$1}"
  local tmp
  tmp="$(mktemp)"
  # grep -v exits 1 when the result is empty; that is a successful strip.
  grep -vE "$(cursor_attribution_line_ere)" "$src" >"$tmp" || true
  if [[ "$dest" == "$src" ]]; then
    cat "$tmp" >"$src"
    rm -f "$tmp"
  else
    mv "$tmp" "$dest"
  fi
}

commit_message_has_subject() {
  local file="$1"
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^# ]] && continue
    [[ -z "${line//[[:space:]]/}" ]] && continue
    return 0
  done <"$file"
  return 1
}

text_has_cursor_attribution() {
  local text="$1"
  grep -qE "$(cursor_attribution_line_ere)" <<<"$text"
}

ident_is_cursor() {
  local name="${1:-}"
  local email="${2:-}"
  local ident="${3:-}"
  local email_lc
  email_lc="$(printf '%s' "$email" | tr '[:upper:]' '[:lower:]')"
  if [[ "$email_lc" == "cursoragent@cursor.com" ]]; then
    return 0
  fi
  if [[ "$name" == "Cursor Agent" ]]; then
    return 0
  fi
  if [[ "$ident" == *"cursoragent@cursor.com"* || "$ident" == *"Cursor Agent"* ]]; then
    return 0
  fi
  return 1
}

current_ident_is_cursor() {
  local author_name="${GIT_AUTHOR_NAME:-}"
  local author_email="${GIT_AUTHOR_EMAIL:-}"
  local committer_name="${GIT_COMMITTER_NAME:-}"
  local committer_email="${GIT_COMMITTER_EMAIL:-}"
  local author_ident="" committer_ident=""
  if git rev-parse --git-dir >/dev/null 2>&1; then
    author_ident="$(git var GIT_AUTHOR_IDENT 2>/dev/null || true)"
    committer_ident="$(git var GIT_COMMITTER_IDENT 2>/dev/null || true)"
  fi
  ident_is_cursor "$author_name" "$author_email" "$author_ident" \
    || ident_is_cursor "$committer_name" "$committer_email" "$committer_ident"
}

export_rafael_ident() {
  export GIT_AUTHOR_NAME="${REWRITE_AUTHOR_NAME}"
  export GIT_AUTHOR_EMAIL="${REWRITE_AUTHOR_EMAIL}"
  export GIT_COMMITTER_NAME="${REWRITE_AUTHOR_NAME}"
  export GIT_COMMITTER_EMAIL="${REWRITE_AUTHOR_EMAIL}"
}

persist_rafael_ident() {
  export_rafael_ident
  # Hook exports do not reach the parent git process. Local config is used
  # by later commits and by the post-commit amend.
  if git rev-parse --git-dir >/dev/null 2>&1; then
    git config --local user.name "${REWRITE_AUTHOR_NAME}"
    git config --local user.email "${REWRITE_AUTHOR_EMAIL}"
  fi
}

rewrite_cursor_ident() {
  if ! current_ident_is_cursor; then
    return 0
  fi
  persist_rafael_ident
}

# git resolves author before prepare-commit-msg/pre-commit, so those hooks
# cannot change THIS commit. Amend HEAD when it still carries Cursor identity.
amend_head_if_cursor_ident() {
  if [[ "${CURSOR_IDENT_REWRITE:-}" == "1" ]]; then
    return 0
  fi
  if [[ -d "$(git rev-parse --git-path rebase-merge)" || -d "$(git rev-parse --git-path rebase-apply)" ]]; then
    return 0
  fi
  local author_name author_email committer_name committer_email
  author_name="$(git log -1 --format='%an')"
  author_email="$(git log -1 --format='%ae')"
  committer_name="$(git log -1 --format='%cn')"
  committer_email="$(git log -1 --format='%ce')"
  if ! ident_is_cursor "$author_name" "$author_email" "" \
    && ! ident_is_cursor "$committer_name" "$committer_email" ""; then
    return 0
  fi
  persist_rafael_ident
  export CURSOR_IDENT_REWRITE=1
  git commit --amend --no-edit --reset-author
}

print_fix_guidance() {
  cat >&2 <<'EOF'

Cursor attribution is not allowed on this pull request.

Fix:
  1. Strip Cursor co-author / made-with trailers and the Cursor agent mailbox
     from commit messages and the PR description.
  2. Squash-merge so history shows Rafael (regutierrez / rpegutierrez@gmail.com).

EOF
}

commit_authors_include_cursor() {
  local base="$1"
  local head="$2"
  local line name email
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    name="${line%% <*}"
    email="${line#*<}"
    email="${email%>}"
    if ident_is_cursor "$name" "$email" "$line"; then
      echo "Cursor author/committer: ${line}" >&2
      return 0
    fi
  done < <(
    git log --format='%an <%ae>' "${base}..${head}"
    git log --format='%cn <%ce>' "${base}..${head}"
  )
  return 1
}

cmd_strip() {
  local file="${1:?usage: strip <file>}"
  strip_cursor_attribution_file "$file"
  if ! commit_message_has_subject "$file"; then
    echo "commit-msg: message is empty after stripping Cursor attribution" >&2
    return 1
  fi
}

cmd_rewrite_ident() {
  rewrite_cursor_ident
  printf 'GIT_AUTHOR_NAME=%s\n' "${GIT_AUTHOR_NAME:-}"
  printf 'GIT_AUTHOR_EMAIL=%s\n' "${GIT_AUTHOR_EMAIL:-}"
  printf 'GIT_COMMITTER_NAME=%s\n' "${GIT_COMMITTER_NAME:-}"
  printf 'GIT_COMMITTER_EMAIL=%s\n' "${GIT_COMMITTER_EMAIL:-}"
}

cmd_check_pr() {
  local base="${1:-${BASE_SHA:-}}"
  local head="${2:-${HEAD_SHA:-}}"
  local failed=0

  if [[ -z "$base" || -z "$head" ]]; then
    echo "usage: check-pr <base-sha> <head-sha> (or set BASE_SHA and HEAD_SHA)" >&2
    return 2
  fi

  if ! git cat-file -e "${base}^{commit}" 2>/dev/null; then
    git fetch --no-tags origin "$base"
  fi
  if ! git cat-file -e "${head}^{commit}" 2>/dev/null; then
    git fetch --no-tags origin "$head"
  fi

  echo "Checking PR title and body"
  if text_has_cursor_attribution "${PR_TITLE:-}"$'\n'"${PR_BODY:-}"; then
    echo "PR title or body contains Cursor attribution." >&2
    failed=1
  fi

  echo "Checking commit messages ${base}..${head}"
  local messages
  messages="$(git log --format='%s%n%b' "${base}..${head}")"
  if text_has_cursor_attribution "$messages"; then
    echo "A commit message contains Cursor attribution." >&2
    git log --format='--- %h %s ---%n%b' "${base}..${head}" \
      | grep -nE "$(cursor_attribution_line_ere)" || true
    failed=1
  fi

  echo "Checking commit authors ${base}..${head}"
  if commit_authors_include_cursor "$base" "$head"; then
    echo "A commit author or committer is Cursor Agent or the Cursor agent mailbox." >&2
    failed=1
  fi

  if ((failed)); then
    print_fix_guidance
    return 1
  fi
  echo "No Cursor attribution found."
}

cmd_check_text() {
  local text
  text="$(cat)"
  if text_has_cursor_attribution "$text"; then
    echo "Cursor attribution found." >&2
    return 1
  fi
  echo "No Cursor attribution found."
}

usage() {
  cat >&2 <<'EOF'
usage:
  scripts/no-cursor-attribution.sh strip <file>
  scripts/no-cursor-attribution.sh rewrite-ident
  scripts/no-cursor-attribution.sh check-pr [base-sha] [head-sha]
  scripts/no-cursor-attribution.sh check-text
EOF
  exit 2
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  cmd="${1:-}"
  shift || true
  case "$cmd" in
    strip) cmd_strip "$@" ;;
    rewrite-ident) cmd_rewrite_ident "$@" ;;
    check-pr) cmd_check_pr "$@" ;;
    check-text) cmd_check_text "$@" ;;
    *) usage ;;
  esac
fi
