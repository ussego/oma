#!/usr/bin/env bash
# Runnable check for the oma skills commands. No frameworks, just grep.
# grep -q against a pipe would SIGPIPE the writer once it exits at the first
# match (and `set -o pipefail` would then fail the check), so every grep runs
# against a captured variable instead.
set -euo pipefail

cd "$(dirname "$0")"

BIN="$(mktemp /tmp/oma-skills.XXXXXX)"
trap 'rm -f "$BIN"' EXIT

# Build the CLI and run it against the repo's embedded skill-data.
( cd cli && go build -o "$BIN" . )

fail() { echo "FAIL: $1" >&2; exit 1; }

# list: name<TAB>description, includes every skill
list_out="$("$BIN" skills list)"
for s in oma qml qt quickshell qs-ui omarchy-shell; do
	grep -q $'^'"$s"$'\t' <<< "$list_out" || fail "list missing $s"
done

# get: frontmatter + body verbatim
get_oma="$("$BIN" skills get oma)"
grep -q '^---$' <<< "$get_oma" || fail "get oma has no frontmatter"
grep -q '^# oma' <<< "$get_oma" || fail "get oma missing body"

# get --full: inlines references with provenance headers
full_oma="$("$BIN" skills get oma --full)"
grep -q '# references/bridge.md' <<< "$full_oma" || fail "get --full missing reference header"

# get --full on a skill without references/ must not error
full_qt="$("$BIN" skills get qt --full)"
grep -q '^# Qt / QJSEngine' <<< "$full_qt" || fail "get qt --full failed"

# get --all: "---" separated, includes more than one skill
all_out="$("$BIN" skills get --all)"
grep -q '^---$' <<< "$all_out" || fail "get --all missing separator"
grep -q '^name: qml$' <<< "$all_out" || fail "get --all missing qml skill"

# get --all --full: everything with inlined references
allfull_out="$("$BIN" skills get --all --full)"
grep -q '# references/bridge.md' <<< "$allfull_out" || fail "get --all --full missing reference header"

# unknown name: stderr message + non-zero exit
if "$BIN" skills get does-not-exist >/dev/null 2>&1; then
	fail "unknown skill should exit non-zero"
fi
err_out="$("$BIN" skills get does-not-exist 2>&1 >/dev/null || true)"
[ "$err_out" = "Unknown skill: does-not-exist" ] || fail "unknown skill message was: $err_out"

echo "OK"
