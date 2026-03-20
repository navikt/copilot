# Client Support Matrix

Tracking document for customization install mechanisms and client compatibility across GitHub Copilot environments.

**Last verified**: 2026-03-19

---

## References

Sources used to compile this matrix. Re-check these when updating.

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

---

## Support Matrix

### Legend

| Symbol | Meaning                               |
| ------ | ------------------------------------- |
| ✅      | Full support                          |
| ⚠️      | Partial / preview support (see notes) |
| ❌      | Not supported                         |

### Customization Types × Clients

| Type                              | VS Code              | JetBrains            | GitHub.com              | Copilot CLI        | Visual Studio    | Eclipse        | Xcode          |
| --------------------------------- | -------------------- | -------------------- | ----------------------- | ------------------ | ---------------- | -------------- | -------------- |
| **copilot-instructions.md**       | ✅ Chat + Agent       | ✅ Chat + Agent       | ✅ Chat + Agent + Review | ✅                  | ✅ Chat           | ✅ Chat + Agent | ✅ Chat + Agent |
| **\*.instructions.md**            | ✅ Chat + Agent       | ✅ Chat + Agent       | ✅ Agent + Review        | ✅                  | ✅ Chat           | ✅ Agent only   | ✅ Chat + Agent |
| **AGENTS.md**                     | ✅ Chat + Agent       | ✅ Agent              | ✅ Agent                 | ✅                  | ❌                | ✅ Agent only   | ✅ Agent        |
| **Custom Agents (.agent.md)**     | ✅ Chat + Agent       | ✅ Chat + Agent       | ✅ Agent                 | ✅ `/agent`         | ❌                | ⚠️ Agent only   | ⚠️ Agent only   |
| **Reusable Prompts (.prompt.md)** | ✅ `/prompt-name`     | ✅ `/prompt-name`     | ❌                       | ❌                  | ✅ `/prompt-name` | ❌              | ❌              |
| **Agent Skills (SKILL.md)**       | ✅ Auto-discovery     | ⚠️ Agent Mode preview | ✅ Coding agent          | ✅ `/skills`        | ❌                | ❌              | ❌              |
| **MCP Servers**                   | ✅ `.vscode/mcp.json` | ✅ `.idea/mcp.json`   | ✅ Org config            | ✅ `gh copilot mcp` | ✅                | ❌              | ❌              |
| **Organization instructions**     | ❌                    | ❌                    | ✅ Chat + Agent + Review | ❌                  | ❌                | ❌              | ❌              |
| **Personal instructions**         | ✅ Settings           | ✅ Settings           | ✅ Chat                  | ❌                  | ✅ Settings       | ❌              | ❌              |

> **Note**: "Agent" refers to Copilot coding agent (autonomous mode). "Chat" refers to interactive Copilot Chat.

---

## Install Mechanisms per Type

### Instructions (.instructions.md)

| Method                                 | Client  | Notes                                            |
| -------------------------------------- | ------- | ------------------------------------------------ |
| One-click install button               | VS Code | Via `vscode:chat-instructions/install?url=...`   |
| Manual copy to `.github/instructions/` | All     | Works everywhere — universal format              |
| curl from GitHub raw                   | All     | `curl -sO --output-dir .github/instructions ...` |

**Status**: Most portable customization type. Works in all clients.

### Custom Agents (.agent.md)

| Method                           | Client      | Notes                                       |
| -------------------------------- | ----------- | ------------------------------------------- |
| One-click install button         | VS Code     | Via `vscode:chat-agent/install?url=...`     |
| Manual copy to `.github/agents/` | All         | File must exist in repo for coding agent    |
| GitHub.com agents tab            | GitHub.com  | Create/select agents directly on github.com |
| `/agent` command                 | Copilot CLI | Select agent in CLI session                 |
| Configure Custom Agents menu     | JetBrains   | Create/select in JetBrains Chat UI          |

**Status**: Now supported across all major clients. JetBrains added full Chat support (was previously coding agent only).

### Reusable Prompts (.prompt.md)

| Method                            | Client                            | Notes                                    |
| --------------------------------- | --------------------------------- | ---------------------------------------- |
| One-click install button          | VS Code                           | Via `vscode:chat-prompt/install?url=...` |
| Manual copy to `.github/prompts/` | VS Code, JetBrains, Visual Studio | Invoke with `/prompt-name` in Chat       |

**Status**: IDE-only feature. Not supported in CLI or GitHub.com.

### Agent Skills (SKILL.md folders)

| Method                                             | Client                  | Notes                          |
| -------------------------------------------------- | ----------------------- | ------------------------------ |
| Manual copy folder to `.github/skills/`            | VS Code, JetBrains, CLI | Auto-discovered by agents      |
| Personal skills in `~/.copilot/skills/`            | VS Code, CLI            | Cross-project personal skills  |
| `/skills list`, `/skills add`                      | Copilot CLI             | Full skill management commands |
| Enable in Settings > GitHub Copilot > Chat > Agent | JetBrains               | Public preview, must opt in    |

**Status**: Major expansion — CLI now has full skill support including `/skills list`, `/skills info`, `/skills add`, `/skills reload`, `/skills remove`. JetBrains Agent Mode preview added.

### MCP Servers

| Method                      | Client      | Notes                                       |
| --------------------------- | ----------- | ------------------------------------------- |
| VS Code MCP Registry panel  | VS Code     | Extensions panel → filter → MCP Registry    |
| `.vscode/mcp.json` in repo  | VS Code     | Shared with team                            |
| `.idea/mcp.json` in project | JetBrains   | Shared with team                            |
| `gh copilot mcp add`        | Copilot CLI | Or edit `~/.config/github-copilot/mcp.json` |
| Organization MCP config     | GitHub.com  | Org-level server configuration              |

**Status**: Broad support across all major clients.

---

## Nav Customization Inventory

### Agents (11 files)

| Agent             | File                         | JetBrains | CLI |
| ----------------- | ---------------------------- | --------- | --- |
| Accessibility     | `accessibility.agent.md`     | ✅         | ✅   |
| Aksel             | `aksel.agent.md`             | ✅         | ✅   |
| Auth              | `auth.agent.md`              | ✅         | ✅   |
| Code Review       | `code-review.agent.md`       | ✅         | ✅   |
| Forfatter         | `forfatter.agent.md`         | ✅         | ✅   |
| Kafka             | `kafka.agent.md`             | ✅         | ✅   |
| Nais              | `nais.agent.md`              | ✅         | ✅   |
| Observability     | `observability.agent.md`     | ✅         | ✅   |
| Research          | `research.agent.md`          | ✅         | ✅   |
| Rust              | `rust.agent.md`              | ✅         | ✅   |
| Security Champion | `security-champion.agent.md` | ✅         | ✅   |

### Instructions (8 files)

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

### Prompts (5 files)

| Prompt               | File                             | JetBrains | CLI |
| -------------------- | -------------------------------- | --------- | --- |
| Aksel Component      | `aksel-component.prompt.md`      | ✅         | ❌   |
| Kafka Topic          | `kafka-topic.prompt.md`          | ✅         | ❌   |
| Nais Manifest        | `nais-manifest.prompt.md`        | ✅         | ❌   |
| Next.js API Route    | `nextjs-api-route.prompt.md`     | ✅         | ❌   |
| Spring Boot Endpoint | `spring-boot-endpoint.prompt.md` | ✅         | ❌   |

### Skills (14 folders)

| Skill                | Folder                                 | JetBrains | CLI |
| -------------------- | -------------------------------------- | --------- | --- |
| ai-news-research     | `.github/skills/ai-news-research/`     | ⚠️ Preview | ✅   |
| aksel-spacing        | `.github/skills/aksel-spacing/`        | ⚠️ Preview | ✅   |
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

---

## Changes Since Last Review

### 2026-03-19: Major client support expansions

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

---

## VS Code Tasks Issues

The workspace task definitions (shown in Command Palette) reference old agent filenames that no longer exist:

| Task Label                               | References               | Actual File               |
| ---------------------------------------- | ------------------------ | ------------------------- |
| Install Individual - Nais Platform Agent | `nais-platform.agent.md` | `nais.agent.md`           |
| Install Individual - Kafka Events Agent  | `kafka-events.agent.md`  | `kafka.agent.md`          |
| Install Individual - Aksel Design Agent  | `aksel-design.agent.md`  | `aksel.agent.md`          |
| Install All Agents                       | "6 agent files"          | 11 agent files exist      |
| Install All Instructions                 | "4 instruction files"    | 8 instruction files exist |
| Install All Prompts                      | "3 prompt files"         | 5 prompt files exist      |

These tasks are local-only (`.vscode/tasks.json`) and not committed to the repo, so they only affect this workspace.

---

## Metadata Schema

Current metadata files (`.metadata.json`) contain:

```json
{
  "domain": "platform",
  "tags": ["nais", "kubernetes"],
  "examples": [{ "prompt": "...", "scenario": "..." }]
}
```

Missing fields that could improve tracking:
- `version` — for changelog tracking
- `supportedClients` — explicit client compatibility
- `minCopilotVersion` — minimum required Copilot version
- `lastUpdated` — timestamp for staleness detection
