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
an agent reading `~/.aws` or posting your environment to a host you never
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

cplt is our sandbox for coding agents. It runs Copilot CLI, OpenCode,
Antigravity CLI, Pi, Claude Code, goose or a plain shell inside Seatbelt on
macOS or Landlock plus seccomp-BPF on Linux. That part is unremarkable, and
several other tools do it well.

Two things sit above the kernel layer.

**Guards on `git` and `gh`.** The agent commits, branches and rebases as much
as it likes. `gh pr merge`, `gh repo delete`, `gh release create` and
`gh workflow run` are blocked, along with about sixty other subcommands that
need a human, and anything `gh` grows later that the policy table has not
heard of. `git push` warns by default and can be set to block, optionally only
on the default branch so feature branches stay free. Force push is blocked
even there.

We should be precise about what the guard is. It is a shim on the PATH. The
real `/usr/bin/git` is untouched, and an agent that calls it by absolute path,
or resets the PATH, or shells out from Python with the full path, walks past
the guard. Our security doc says so in as many words. The guard stops an agent
that does what it is told, including one that a prompt injection told to be
helpful and merge. It does not stop one that is trying to get out. For that,
the boundary is branch protection on the server, and the guard is there so
you find out on the laptop instead of in the audit log.

nono, another sandbox in this space, reaches a similar end from the other
direction: it scopes the GitHub token through a credential proxy so the API
itself refuses. Both beat hoping the model behaves.

**Policy committed to the repository.** cplt reads `.cplt.toml` from the
repository, and it reads the version at `git HEAD`, not the working tree, so
the agent cannot rewrite its own rules mid-session. Writing the file is denied
by the kernel inside the sandbox. The file has two halves. A `[deny]` section
lists paths and environment variables the agent never sees. It takes effect on
every clone with no approval, because it can only tighten. A `[propose]`
section asks for permissions, such as Docker or a localhost port. Nothing in
it takes effect until the developer runs `cplt trust accept`, and the approval
is pinned to a hash of the file, so a later edit invalidates it.

That split is the property we built around. Sandbox policy usually lives in a
home directory, where it is one developer's private arrangement, or in an MDM,
where it belongs to whoever administers laptops. Neither travels with the
project. A rule that says *the agent in this repository never sees
`VAULT_TOKEN`* is a fact about the repository. It should be written down there,
reviewed in a pull request like any other change, and follow the code to
whoever clones it.

The guards themselves are the exception. A repository can ask for them to be
turned on, but how they behave, warn or block, is a setting on the developer's
machine, not in the repo. We are not sure that is the right split yet.

## The honest part

None of this is a sandbox escape story. We have not caught an agent deleting a
repository. The reasoning is duller: the tools hold credentials that can do
irreversible things, the cost of preventing that is a wrapper and a config
file, and we would rather write the rule down than trust that the next model
release stays polite.

cplt does not run on Windows outside WSL2. macOS has the strongest enforcement
today, and the Linux coverage is not identical. `git push` warns rather than
blocks until you configure it, which is a deliberate default for adoption and
an odd one for security, and we argue about it internally.

It is MIT, it is a single binary, and the code is at
[github.com/navikt/cplt](https://github.com/navikt/cplt).
