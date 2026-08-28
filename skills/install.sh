#!/usr/bin/env bash
# Install ray's skills for every agent harness on this machine.
#
# SKILL.md is a cross-agent open standard: Claude Code, Codex, Cursor,
# Gemini CLI and others load the same folder unchanged. Each harness reads
# its own directory, and `.agents/skills/` is the vendor-neutral convention
# the others are converging on, so a skill is installed to all of them.
#
# Symlinks rather than copies, so a skill edited in the repository takes
# effect immediately in every harness and can never drift from the compiler
# it documents.
#
#   ./skills/install.sh                install every skill for every harness
#   ./skills/install.sh spec           install one skill
#   ./skills/install.sh --user         home directories only (default)
#   ./skills/install.sh --project DIR  install into DIR/.agents, DIR/.claude, ...
#   ./skills/install.sh --list
#   ./skills/install.sh --uninstall
set -euo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Vendor-neutral first; the rest are the per-harness paths each one reads.
HARNESS_DIRS=(".agents/skills" ".claude/skills" ".codex/skills" ".cursor/skills")

ROOT="$HOME"
MODE=user
ARGS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --user)      MODE=user; ROOT="$HOME"; shift ;;
    --project)   MODE=project; ROOT="${2:?--project needs a directory}"; shift 2 ;;
    --list|--uninstall) ACTION="$1"; shift ;;
    -*) echo "unknown flag: $1" >&2; exit 2 ;;
    *) ARGS+=("$1"); shift ;;
  esac
done

available() {
  for d in "$SRC"/*/; do
    [ -f "$d/SKILL.md" ] && basename "$d"
  done
}

targets() {
  for rel in "${HARNESS_DIRS[@]}"; do
    printf '%s/%s\n' "$ROOT" "$rel"
  done
}

case "${ACTION:-}" in
  --list)
    printf '%-12s %-26s %s\n' SKILL DIRECTORY STATE
    for name in $(available); do
      for dir in $(targets); do
        link="$dir/$name"
        if [ -L "$link" ] && [ "$(readlink "$link")" = "$SRC/$name" ]; then state=installed
        elif [ -e "$link" ]; then state=occupied
        else state=absent; fi
        printf '%-12s %-26s %s\n' "$name" "${dir/#$HOME/~}" "$state"
      done
    done
    exit 0
    ;;
  --uninstall)
    for name in $(available); do
      for dir in $(targets); do
        link="$dir/$name"
        # Only remove links this script made; never touch a hand-authored dir.
        if [ -L "$link" ] && [ "$(readlink "$link")" = "$SRC/$name" ]; then
          rm "$link"
          echo "removed ${dir/#$HOME/~}/$name"
        fi
      done
    done
    exit 0
    ;;
esac

requested=("${ARGS[@]}")
if [ ${#requested[@]} -eq 0 ]; then
  while IFS= read -r line; do requested+=("$line"); done < <(available)
fi

for name in "${requested[@]}"; do
  if [ ! -f "$SRC/$name/SKILL.md" ]; then
    echo "no such skill: $name" >&2
    exit 1
  fi
  for dir in $(targets); do
    link="$dir/$name"
    if [ -e "$link" ] && [ ! -L "$link" ]; then
      echo "skipped ${dir/#$HOME/~}/$name — exists and is not a symlink" >&2
      continue
    fi
    mkdir -p "$dir"
    ln -sfn "$SRC/$name" "$link"
    echo "installed ${dir/#$HOME/~}/$name"
  done
done

cat <<'NOTE'

Installed for every harness that reads these paths — Claude Code, Codex,
Cursor, Gemini CLI and others sharing the SKILL.md standard. Restart or
resume the session to pick up newly installed skills.
NOTE
