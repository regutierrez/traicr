#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=no-cursor-attribution.sh
source "${root}/scripts/no-cursor-attribution.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok: $*"
}

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

cat >"$tmp" <<'EOF'
feat: example

Body stays.

Co-authored-by: Cursor Agent <cursoragent@cursor.com>
Co-authored-by: Rafael <rpegutierrez@gmail.com>
Made-with: Cursor
Made with Cursor
Signed-off-by: someone@example.com
EOF

strip_cursor_attribution_file "$tmp"
grep -q 'feat: example' "$tmp" || fail "subject removed"
grep -q 'Body stays.' "$tmp" || fail "body removed"
grep -q 'Co-authored-by: Rafael' "$tmp" || fail "non-Cursor co-author removed"
! grep -qE 'Co-authored-by:.*[Cc]ursor' "$tmp" || fail "Cursor co-author remained"
! grep -q 'Made-with:' "$tmp" || fail "Made-with remained"
! grep -q 'Made with Cursor' "$tmp" || fail "Made with Cursor remained"
! grep -q 'cursoragent@cursor.com' "$tmp" || fail "mailbox remained"
grep -q 'Signed-off-by: someone@example.com' "$tmp" || fail "unrelated trailer removed"
commit_message_has_subject "$tmp" || fail "valid subject rejected"
pass "strip keeps subject/body and drops Cursor trailers"

cat >"$tmp" <<'EOF'
Co-authored-by: Cursor Agent <cursoragent@cursor.com>
Made-with: Cursor
Made with Cursor
EOF
strip_cursor_attribution_file "$tmp"
if commit_message_has_subject "$tmp"; then
  fail "empty message after strip was accepted"
fi
pass "empty message after strip is rejected"

cat >"$tmp" <<'EOF'
# please enter a commit message
EOF
if commit_message_has_subject "$tmp"; then
  fail "comment-only message was accepted"
fi
pass "comment-only message has no subject"

text_has_cursor_attribution "normal message" && fail "false positive on clean text"
text_has_cursor_attribution $'feat: ok\n\nMade with Cursor' || fail "missed Made with Cursor"
text_has_cursor_attribution "Co-authored-by: Cursor" || fail "missed Co-authored-by"
text_has_cursor_attribution "Made-with: Cursor" || fail "missed Made-with"
text_has_cursor_attribution "thanks cursoragent@cursor.com" || fail "missed mailbox"
pass "text detection"

ident_is_cursor "Cursor Agent" "cursoragent@cursor.com" "" || fail "ident miss"
ident_is_cursor "regutierrez" "rpegutierrez@gmail.com" "" && fail "Rafael flagged"
ident_is_cursor "" "" "Cursor Agent <cursoragent@cursor.com> 1 +0000" || fail "ident string miss"
ident_is_cursor "Other" "CURSORAGENT@CURSOR.COM" "" || fail "mailbox case miss"
pass "ident detection"

echo "All no-cursor-attribution tests passed."
