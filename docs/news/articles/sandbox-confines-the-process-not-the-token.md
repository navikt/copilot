---
title: "Your sandbox confines the process, not the token"
date: 2026-09-06
author: starefossen
category: praksis
lang: en
excerpt: "A coding agent inside a perfect kernel sandbox can still push to main and merge its own pull request. Nothing it does breaks a sandbox rule."
tags:
  - security
  - coding-agents
---

Nav is Norway's labour and welfare administration. We pay out roughly a third
of the national budget, and a few hundred developers keep that running. In
March we asked them how they work with AI coding tools. 163 answered. Twelve
said they use none. Three quarters use two or more.

That is the context for what follows. The agents are here, they run on
developer laptops with real credentials, and the question is no longer whether
to allow them.

## The gap

Put a coding agent in a kernel sandbox. Seatbelt on macOS, Landlock and
seccomp on Linux. Deny it your SSH keys, your cloud credentials, everything
outside the project directory. Give it an egress allowlist. Good. That stops
an agent reading `~/.aws` or posting your environment to a host you have never
heard of.

Now watch it push to main.

Nothing in that sequence breaks a sandbox rule. The agent runs `git push`, or
`gh pr merge`, or `gh repo delete`. Those are well-formed API calls from a
client holding a valid token, over a connection you allowed, using a binary
you put on the PATH. A filesystem rule has no opinion about them. Neither does
a syscall filter. The sandbox was asked whether this process may open a socket,
not whether this agent should be merging its own work.

The blast radius of a confined agent is therefore not the machine. It is
everything the token can reach: the default branch, the release, the pull
request that no human read.

## What we do about it

cplt is our sandbox for coding agents. It runs Copilot CLI, OpenCode, Gemini
CLI, Antigravity CLI, Pi, Claude Code, goose or a plain shell inside Seatbelt
on macOS or Landlock plus seccomp-BPF on Linux. That part is unremarkable, and
several other tools do it well.

Two things sit above the kernel layer.

**Guards on `git` and `gh`.** The agent commits, branches and rebases as much
as it likes. `gh pr merge`, `gh repo delete`, `gh release create` and
`gh workflow run` are blocked, along with about sixty other subcommands that
need a human, and anything `gh` grows later that the policy table has not
heard of. `git push` to the default branch is blocked. A push to a feature
branch goes through, so the review gate is the pull request rather than the
push. Force push is blocked everywhere. All of that is the default; you can
tighten it to refuse every push, or loosen it to a warning, but you do not
have to configure anything to get it.

We should be precise about what the guard is. It is a shim on the PATH. The
real `/usr/bin/git` is untouched, and an agent that calls it by absolute path,
or resets the PATH, or shells out from Python with the full path, walks past
the guard. Our security doc says so in as many words: a best-effort command
filter, not a kernel boundary. The guard stops an agent that does what it is
told, including one that a prompt injection told to be helpful and merge. It
does not stop one that is trying to get out. For that, the boundary is branch
protection on the server, and the guard is there so you find out on the laptop
instead of in the audit log.

nono, another sandbox in this space, reaches a similar end from the other
direction: it scopes the GitHub token through a credential proxy so the API
itself refuses. Both beat hoping the model behaves.

**Policy committed to the repository.** cplt reads `.cplt.toml` from the
repository, and it reads the version at `git HEAD`, not the working tree, so
the agent cannot rewrite its own rules mid-session. Inside the sandbox the
file is write-denied, by Seatbelt on macOS and by a read-only bind mount on
Linux when bubblewrap is available. The file has two halves. A `[deny]`
section lists paths and environment variables the agent never sees. It takes
effect on every clone with no approval, because it can only tighten. A
`[propose]` section asks for permissions, such as Docker or a localhost port.
Nothing in it takes effect until the developer runs `cplt trust accept`, and
the approval is pinned to a hash of the file, so a later edit invalidates it.

That split is the property we built around. Sandbox policy usually lives in a
home directory, where it is one developer's private arrangement, or in an MDM,
where it belongs to whoever administers laptops. Neither travels with the
project. A rule that says *the agent in this repository never sees
`VAULT_TOKEN`* is a fact about the repository. It should be written down there,
reviewed in a pull request like any other change, and follow the code to
whoever clones it.

We are not alone in thinking so. fence looks for `fence.jsonc` in the working
directory before falling back to the home directory, and agentcontainers puts
agent policy beside `devcontainer.json`. What we have not seen elsewhere is
reading that file from `HEAD` rather than from disk. A policy file on disk is
one the agent can edit before the next run. A policy file from the last commit,
in a sandbox where writing it is denied, is one the agent has no way to reach.

The guards themselves are the exception. A repository can ask for them to be
turned on, but how they behave, warn or block, is a setting on the developer's
machine, not in the repo. We are not sure that is the right split yet.

## The honest part

None of this is a sandbox escape story. We have not caught an agent deleting a
repository. The reasoning is duller: the tools hold credentials that can do
irreversible things, the cost of preventing that is a wrapper and a config
file, and we would rather write the rule down than trust that the next model
release stays polite.

Until recently the two guards had opposite defaults. The `gh` guard blocked
and the `git` guard warned, so the weaker default sat on the operation with
the worse blast radius. We picked warn for adoption and then argued about it
internally. Both block now.

cplt runs on macOS and Linux. There is no Windows backend; WSL2 works as an
ordinary Linux install. macOS still has the strongest file-level enforcement.
Landlock cannot deny a path inside a tree it has allowed, so on Linux
`.git/hooks` and `.cplt.toml` stay writable unless bubblewrap is present to
bind them read-only, and `.git/config` stays writable either way. We are
closing the differences that are code gaps rather than kernel limits, and the
security doc lists what remains.

It is MIT, it is a single binary, and the code is at
[github.com/navikt/cplt](https://github.com/navikt/cplt).
