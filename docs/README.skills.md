# 🎯 Agent Skills

Agent Skills are self-contained folders with instructions and bundled resources that enhance AI capabilities for specialized NAV development tasks.

Based on the [Agent Skills specification](https://agentskills.io/specification), each skill contains a `SKILL.md` file with detailed instructions that agents load on-demand.

Skills differ from other primitives by supporting bundled assets (scripts, code samples, reference data) that agents can utilize when performing specialized tasks.

### How to Use Agent Skills

**What's Included:**
- Each skill is a folder containing a `SKILL.md` instruction file
- Skills may include helper scripts, code templates, or reference data
- Skills follow the Agent Skills specification for maximum compatibility

**When to Use:**
- Skills are ideal for complex, repeatable workflows that benefit from bundled resources
- Use skills when you need code templates, helper utilities, or reference data alongside instructions
- Skills provide progressive disclosure - loaded only when needed for specific tasks

**Usage:**
- Browse the skills table below to find relevant capabilities
- Copy the skill folder to your local skills directory (`.github/skills/`)
- Reference skills in your prompts or let the agent discover them automatically

## Available Skills

| Name                    | Description                                                                                      | Location                                                                                |
| ----------------------- | ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------- |
| **aksel-spacing**       | Responsive layout patterns using Aksel spacing tokens with Box, VStack, HStack, and HGrid        | [`.github/skills/aksel-spacing/`](../.github/skills/aksel-spacing/SKILL.md)             |
| **flyway-migration**    | Database migration patterns using Flyway with versioned SQL scripts                              | [`.github/skills/flyway-migration/`](../.github/skills/flyway-migration/SKILL.md)       |
| **kotlin-app-config**   | Sealed class configuration pattern for Kotlin applications with environment-specific settings    | [`.github/skills/kotlin-app-config/`](../.github/skills/kotlin-app-config/SKILL.md)     |
| **observability-setup** | Setting up Prometheus metrics, OpenTelemetry tracing, and health endpoints for Nais applications | [`.github/skills/observability-setup/`](../.github/skills/observability-setup/SKILL.md) |
| **security-review**     | Pre-commit/PR security checks — use when about to commit, push, or open a pull request           | [`.github/skills/security-review/`](../.github/skills/security-review/SKILL.md)         |
| **tokenx-auth**         | Service-to-service authentication using TokenX token exchange in Nais                            | [`.github/skills/tokenx-auth/`](../.github/skills/tokenx-auth/SKILL.md)                 |

## Creating NAV Skills

When creating agent skills for NAV projects:

1. **Follow Specification**: Adhere to the [Agent Skills specification](https://agentskills.io/specification)
2. **Bundle Resources**: Include templates, scripts, and reference data
3. **NAV Context**: Include NAV-specific patterns and configurations
4. **Self-Contained**: Skills should be independent and reusable
5. **Progressive Disclosure**: Load only when needed for specific tasks

## Skill Structure

```
.github/skills/
└── skill-name/
    ├── SKILL.md              # Main instruction file
    ├── templates/            # Code templates
    ├── scripts/              # Helper scripts
    ├── examples/             # Example implementations
    └── reference/            # Reference documentation
```

## Best Practices

- Keep skills focused on specific domains
- Include practical examples from NAV projects
- Provide clear usage instructions
- Bundle only necessary resources
- Test skills in various NAV contexts
- Document dependencies and requirements
