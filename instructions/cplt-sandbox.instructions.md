---
applyTo: "**"
---

# cplt Sandbox

Applies only when `$__CPLT_WRAPPED` is set. Otherwise ignore this file.

- Create worktrees only when `$CPLT_WORKTREE_SOCKET` is set; otherwise ask the user to prepare a worktree outside cplt. Never fall back to scratch.
- `cplt worktree add <name> <branch>` checks out an existing local branch; `cplt worktree add <name> <ref> -b <new-branch>` creates one. No `git switch` step.
- Never run plain `git worktree add`. It aborts mid-checkout on protected files and can land in scratch.
- Scratch and `$TMPDIR` are deleted when the session ends. Use them for helpers only, never for worktrees or unfinished work.
- When cplt refuses a command, read the message, report what was refused and the fix it names. Do not route around it.
