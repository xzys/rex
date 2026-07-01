#!/bin/sh
# setup-worktree.sh — wire up worktree-local symlinks after `git worktree add`.
#
# Invoked from the post-checkout hook (CWD = worktree root). It links the
# gitignored, non-tracked paths that every worktree needs:
#
#   .env    -> main worktree's .env
#   notes/  -> sibling rex-notes repo  (../rex-notes from the main worktree)
#   web/    -> sibling rex-web repo    (../rex-web   from the main worktree)
#
# It only acts when ALL of these hold:
#   1. previous-HEAD ($1) is all zeros  -> initial checkout, not a switch
#   2. git-dir lives under git-common-dir/worktrees/  -> a linked worktree,
#      not the main worktree (which keeps its own .env / real symlinks)
#
# Idempotent: existing correct symlinks are left alone; a non-symlink at a
# target path is left in place with a warning (never clobbers real files).

set -eu

prev_head="${1:-}"
# Only run on an initial checkout (no prior HEAD). For `git worktree add`
# the new worktree has no previous HEAD, so $1 is 40 zeros.
case "$prev_head" in
  ''|0000000000000000000000000000000000000000) : ;;
  *) exit 0 ;;
esac

git_dir=$(git rev-parse --git-dir)
common_dir=$(git rev-parse --git-common-dir)
git_dir=$(cd "$git_dir" && pwd)
common_dir=$(cd "$common_dir" && pwd)

# Only act inside linked worktrees, never the main one.
case "$git_dir" in
  "$common_dir"/worktrees/*) : ;;
  *) exit 0 ;;
esac

main_root=$(dirname "$common_dir")   # parent of .git = main worktree root
wt_root=$(pwd)

make_link() {
  target="$1"   # absolute path to point at
  link="$2"     # path inside the worktree
  if [ -L "$link" ]; then
    cur=$(readlink "$link")
    [ "$cur" = "$target" ] && return 0
    rm -f "$link"
  elif [ -e "$link" ]; then
    printf 'setup-worktree: %s exists and is not a symlink; skipping\n' \
      "$link" >&2
    return 0
  fi
  ln -s "$target" "$link"
  printf 'setup-worktree: linked %s -> %s\n' "$link" "$target"
}

make_link "$main_root/.env"      "$wt_root/.env"
make_link "$main_root/../rex-notes" "$wt_root/notes"
make_link "$main_root/../rex-web"   "$wt_root/web"
