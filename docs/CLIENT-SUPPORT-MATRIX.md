# Client Support Matrix

Which customization types work in which GitHub Copilot clients, and how each one gets installed.

## References

Re-check these when you update the matrix.

| #   | Topic                                            | URL                                                                                                      |
| --- | ------------------------------------------------ | -------------------------------------------------------------------------------------------------------- |
| 1   | **Custom instructions support matrix** (primary) | <https://docs.github.com/en/copilot/reference/custom-instructions-support>                               |
| 2   | Prompt files overview (VS Code, VS, JetBrains)   | <https://docs.github.com/en/copilot/tutorials/customization-library/prompt-files>                        |
| 3   | Creating custom agents (all clients)             | <https://docs.github.com/en/copilot/how-tos/copilot-chat/creating-custom-agents>                         |
| 4   | Creating agent skills (VS Code)                  | <https://docs.github.com/en/copilot/how-tos/copilot-chat/using-agent-skills>                             |
| 5   | Creating agent skills (Copilot CLI)              | <https://docs.github.com/en/copilot/how-tos/copilot-cli/using-agent-skills-in-copilot-cli>               |
| 6   | Extending Copilot with MCP                       | <https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-chat-with-mcp>                 |
| 7   | Adding repository custom instructions            | <https://docs.github.com/en/copilot/how-tos/configure-custom-instructions/add-repository-instructions>   |
| 8   | Adding organization custom instructions          | <https://docs.github.com/en/copilot/how-tos/configure-custom-instructions/add-organization-instructions> |
| 9   | JetBrains Copilot changelog (skills preview)     | <https://github.blog/changelog/label/copilot/>                                                           |
| 10  | Blog: Instructions, Prompts, Agents and Skills   | <https://devopsjournal.io/blog/2025/12/22/GitHub-Copilot-Custom-Instructions>                            |
| 11  | Agent Skills specification                       | <https://agentskills.io/specification>                                                                   |

## Support matrix

### Legend

| Symbol | Meaning                               |
| ------ | ------------------------------------- |
| ✅      | Full support                          |
| ⚠️      | Partial / preview support (see notes) |
| ❌      | Not supported                         |

### Customization types × clients

| Type                              | VS Code              | JetBrains            | GitHub.com              | Copilot CLI        | Visual Studio    | Eclipse        | Xcode          |
| --------------------------------- | -------------------- | -------------------- | ----------------------- | ------------------ | ---------------- | -------------- | -------------- |
| **copilot-instructions.md**       | ✅ Chat + Agent       | ✅ Chat + Agent       | ✅ Chat + Agent + Review | ✅                  | ✅ Chat           | ✅ Chat + Agent | ✅ Chat + Agent |
| **\*.instructions.md**            | ✅ Chat + Agent       | ✅ Chat + Agent       | ✅ Agent + Review        | ✅                  | ✅ Chat           | ✅ Agent only   | ✅ Chat + Agent |
| **AGENTS.md**                     | ✅ Chat + Agent       | ✅ Agent              | ✅ Agent                 | ✅                  | ❌                | ✅ Agent only   | ✅ Agent        |
| **Custom Agents (.agent.md)**     | ✅ Chat + Agent       | ✅ Chat + Agent       | ✅ Agent                 | ✅ `/agent`         | ❌                | ⚠️ Agent only   | ⚠️ Agent only   |
| **Reusable Prompts (.prompt.md)** | ✅ `/prompt-name`     | ✅ `/prompt-name`     | ❌                       | ❌                  | ✅ `/prompt-name` | ❌              | ❌              |
| **Agent Skills (SKILL.md)**       | ✅ Auto-discovery     | ⚠️ Agent Mode preview | ✅ Coding agent          | ✅ `/skills`        | ❌                | ❌              | ❌              |
| **MCP Servers**                   | ✅ `.vscode/mcp.json` | ✅ `.idea/mcp.json`   | ✅ Org config            | ✅ `gh copilot mcp` | ✅                | ❌              | ❌              |
| **Organization instructions**     | ❌                    | ❌                    | ✅ Chat + Agent + Review | ❌                  | ✅                | ❌              | ❌              |
| **Personal instructions**         | ✅ Settings           | ✅ Settings           | ✅ Chat                  | ❌                  | ✅ Settings       | ❌              | ❌              |
| **Copilot / Agent Memory**        | ✅ (User/Repo/Session) | ❌                    | ✅ (Repo scope)           | ✅ (Repo scope)      | ❌                | ❌              | ❌              |

> **Note**: "Agent" refers to Copilot coding agent (autonomous mode). "Chat" refers to interactive Copilot Chat.

## Install mechanisms per type

### Instructions (.instructions.md)

| Method                                 | Client  | Notes                                            |
| -------------------------------------- | ------- | ------------------------------------------------ |
| One-click install button               | VS Code | Via `vscode:chat-instructions/install?url=...`   |
| Manual copy to `.github/instructions/` | All     | Universal format, works everywhere               |
| curl from GitHub raw                   | All     | `curl -sO --output-dir .github/instructions ...` |

**Status**: the most portable type. Every client reads it.

### Custom Agents (.agent.md)

| Method                           | Client      | Notes                                       |
| -------------------------------- | ----------- | ------------------------------------------- |
| One-click install button         | VS Code     | Via `vscode:chat-agent/install?url=...`     |
| Manual copy to `.github/agents/` | All         | File must exist in repo for coding agent    |
| GitHub.com agents tab            | GitHub.com  | Create/select agents directly on github.com |
| `/agent` command                 | Copilot CLI | Select agent in CLI session                 |
| Configure Custom Agents menu     | JetBrains   | Create/select in JetBrains Chat UI          |

**Status**: supported in every major client.

### Reusable Prompts (.prompt.md)

| Method                            | Client                            | Notes                                    |
| --------------------------------- | --------------------------------- | ---------------------------------------- |
| One-click install button          | VS Code                           | Via `vscode:chat-prompt/install?url=...` |
| Manual copy to `.github/prompts/` | VS Code, JetBrains, Visual Studio | Invoke with `/prompt-name` in Chat       |

**Status**: IDE only.

### Agent Skills (SKILL.md folders)

| Method                                             | Client                  | Notes                          |
| -------------------------------------------------- | ----------------------- | ------------------------------ |
| Manual copy folder to `.github/skills/`            | VS Code, JetBrains, CLI | Auto-discovered by agents      |
| Personal skills in `~/.copilot/skills/`            | VS Code, CLI            | Cross-project personal skills  |
| `/skills list`, `/skills add`                      | Copilot CLI             | Full skill management commands |
| Enable in Settings > GitHub Copilot > Chat > Agent | JetBrains               | Public preview, must opt in    |

**Status**: the CLI has the deepest support. JetBrains is still preview.

### MCP Servers

| Method                      | Client      | Notes                                       |
| --------------------------- | ----------- | ------------------------------------------- |
| VS Code MCP Registry panel  | VS Code     | Extensions panel → filter → MCP Registry    |
| `.vscode/mcp.json` in repo  | VS Code     | Shared with team                            |
| `.idea/mcp.json` in project | JetBrains   | Shared with team                            |
| `gh copilot mcp add`        | Copilot CLI | Or edit `~/.config/github-copilot/mcp.json` |
| Organization MCP config     | GitHub.com  | Org-level server configuration              |

**Status**: works in every major client.

## Nav customization inventory

Per-file status. The client columns follow the type rows in the matrix above.

### Agents

| Agent             | File                         | JetBrains | CLI |
| ----------------- | ---------------------------- | --------- | --- |
| Accessibility     | `accessibility.agent.md`     | ✅         | ✅   |
| Aksel             | `aksel.agent.md`             | ✅         | ✅   |
| Code Review       | `code-review.agent.md`       | ✅         | ✅   |
| Forfatter         | `forfatter.agent.md`         | ✅         | ✅   |
| Kafka             | `kafka.agent.md`             | ✅         | ✅   |
| Research          | `research.agent.md`          | ✅         | ✅   |
| Rust              | `rust.agent.md`              | ✅         | ✅   |
| Security Champion | `security-champion.agent.md` | ✅         | ✅   |

### Instructions

| Instruction    | File                             | JetBrains | CLI |
| -------------- | -------------------------------- | --------- | --- |
| Accessibility  | `accessibility.instructions.md`  | ✅         | ✅   |
| Database       | `database.instructions.md`       | ✅         | ✅   |
| Docker         | `docker.instructions.md`         | ✅         | ✅   |
| GitHub Actions | `github-actions.instructions.md` | ✅         | ✅   |
| Kotlin/Ktor    | `kotlin-ktor.instructions.md`    | ✅         | ✅   |
| Kotlin/Spring  | `kotlin-spring.instructions.md`  | ✅         | ✅   |
| Next.js/Aksel  | `nextjs-aksel.instructions.md`   | ✅         | ✅   |
| Testing        | `testing.instructions.md`        | ✅         | ✅   |

### Prompts

| Prompt               | File                             | JetBrains | CLI |
| -------------------- | -------------------------------- | --------- | --- |
| Aksel Component      | `aksel-component.prompt.md`      | ✅         | ❌   |
| Kafka Topic          | `kafka-topic.prompt.md`          | ✅         | ❌   |
| Nais Manifest        | `nais-manifest.prompt.md`        | ✅         | ❌   |
| Next.js API Route    | `nextjs-api-route.prompt.md`     | ✅         | ❌   |
| Spring Boot Endpoint | `spring-boot-endpoint.prompt.md` | ✅         | ❌   |

### Skills

| Skill                | Folder                                 | JetBrains | CLI |
| -------------------- | -------------------------------------- | --------- | --- |
| ai-news-research     | `.github/skills/ai-news-research/`     | ⚠️ Preview | ✅   |
| aksel-builder        | `.github/skills/aksel-builder/`        | ⚠️ Preview | ✅   |
| api-design           | `.github/skills/api-design/`           | ⚠️ Preview | ✅   |
| conventional-commit  | `.github/skills/conventional-commit/`  | ⚠️ Preview | ✅   |
| flyway-migration     | `.github/skills/flyway-migration/`     | ⚠️ Preview | ✅   |
| kotlin-app-config    | `.github/skills/kotlin-app-config/`    | ⚠️ Preview | ✅   |
| observability-setup  | `.github/skills/observability-setup/`  | ⚠️ Preview | ✅   |
| playwright-testing   | `.github/skills/playwright-testing/`   | ⚠️ Preview | ✅   |
| postgresql-review    | `.github/skills/postgresql-review/`    | ⚠️ Preview | ✅   |
| rust-development     | `.github/skills/rust-development/`     | ⚠️ Preview | ✅   |
| security-review      | `.github/skills/security-review/`      | ⚠️ Preview | ✅   |
| spring-boot-scaffold | `.github/skills/spring-boot-scaffold/` | ⚠️ Preview | ✅   |
| tokenx-auth          | `.github/skills/tokenx-auth/`          | ⚠️ Preview | ✅   |
| web-design-reviewer  | `.github/skills/web-design-reviewer/`  | ⚠️ Preview | ✅   |


## Changes since last review

### 2026-03-19: client support expansions

| Change                                                       | Impact                                                                                      |
| ------------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| **JetBrains: Custom agents (.agent.md) now in Copilot Chat** | Agents no longer coding-agent-only. Users can `@agent-name` in JetBrains Chat.              |
| **Copilot CLI: Custom agents supported**                     | Agents selectable via `/agent` command in CLI sessions.                                     |
| **Copilot CLI: Full skills support**                         | `/skills list`, `/skills info`, `/skills add`, `/skills reload`, `/skills remove` commands. |
| **JetBrains: Skills in Agent Mode (public preview)**         | Enable via Settings > GitHub Copilot > Chat > Agent. Must opt in.                           |
| **Personal skills location**                                 | `~/.copilot/skills/` for cross-project personal skills (VS Code + CLI).                     |
| **Prompt files: Visual Studio support added**                | Prompts now work in VS Code + JetBrains + Visual Studio (3 IDEs).                           |
| **GitHub.com agents tab**                                    | Create and manage custom agents directly on github.com/copilot/agents.                      |
| **rust.agent.md added**                                      | New agent without metadata.json yet.                                                        |

### 2026-06-30: Microsoft Build and IDE releases

| Change | Impact |
| ------ | ------ |
| **Visual Studio: Organization instructions supported** | Organizations can define custom instructions across all repositories, available in Visual Studio. |
| **Copilot Memory / Agent Memory support** | Scoped memories (User, Repo, Session) introduced in VS Code, and Enterprise controls/policies introduced. |
| **VS Code 1.124: Autopilot & Background sessions** | Autopilot is on by default to determine completion; sessions can run in the background. |

### Docs corrections needed

| File                     | Issue                                                 | Fix                                              |
| ------------------------ | ----------------------------------------------------- | ------------------------------------------------ |
| `docs/README.agents.md`  | JetBrains listed as "Not supported for Copilot Chat"  | Update to ✅ full support                         |
| `docs/README.agents.md`  | CLI listed as "Not supported"                         | Update to ✅ supported via `/agent`               |
| `docs/README.skills.md`  | CLI listed as "Not supported"                         | Update to ✅ full support with `/skills` commands |
| `docs/README.skills.md`  | JetBrains listed as "Works with Copilot coding agent" | Update to ⚠️ Agent Mode preview                   |
| `docs/README.skills.md`  | No mention of personal skills path                    | Add `~/.copilot/skills/`                         |
| `docs/README.prompts.md` | Missing Visual Studio support                         | Add Visual Studio row                            |
| `docs/README.agents.md`  | Missing `rust.agent.md` in table                      | Add Rust Agent row                               |
| `docs/README.skills.md`  | Missing `rust-development` skill in table             | Add (already commented out `ai-news-research`)   |

## VS Code tasks issues

The workspace task definitions in the Command Palette point at agent filenames that no longer exist, and their file counts are out of date:

| Task Label                               | References               | Actual File               |
| ---------------------------------------- | ------------------------ | ------------------------- |
| Install Individual - Nais Platform Agent | `nais-platform.agent.md` | removed, use the `nais` skill |
| Install Individual - Kafka Events Agent  | `kafka-events.agent.md`  | `kafka.agent.md`          |
| Install Individual - Aksel Design Agent  | `aksel-design.agent.md`  | `aksel.agent.md`          |
| Install All Agents                       | "6 agent files"          | 10 agent files exist      |
| Install All Instructions                 | "4 instruction files"    | 8 instruction files exist |
| Install All Prompts                      | "3 prompt files"         | 5 prompt files exist      |

These tasks live in `.vscode/tasks.json`, which is not committed, so they only affect this workspace.

## Metadata schema

Current metadata files (`.metadata.json`) contain:

```json
{
  "domain": "platform",
  "tags": ["nais", "kubernetes"],
  "examples": [{ "prompt": "...", "scenario": "..." }]
}
```

Fields that would improve tracking but are missing:

- `version` for changelog tracking
- `supportedClients` for explicit client compatibility
- `minCopilotVersion` for the minimum required Copilot version
- `lastUpdated` for staleness detection
