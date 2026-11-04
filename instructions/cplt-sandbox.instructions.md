---
applyTo: "**"
---

# cplt Sandbox

Applies only when `$__CPLT_WRAPPED` is set. Otherwise ignore this file.

- Create worktrees only with `cplt worktree add <name> <ref>`. Then check out the branch: `git -C "$CPLT_WORKTREE_ROOT/<name>" switch <branch>` for an existing branch, `switch -c <branch>` for a new one.
- Never run plain `git worktree add`. It aborts mid-checkout on protected files and can land in scratch.
- Scratch and `$TMPDIR` are deleted when the session ends. Use them for helpers only, never for worktrees or unfinished work.
- When cplt refuses a command, read the message, report what was refused and the fix it names. Do not route around it.
