// nav-pilot hooks for opencode. Loaded only by a nav-pilot launch, which names
// this file in OPENCODE_CONFIG_CONTENT and hands it the hooks to run in
// NAV_PILOT_OPENCODE_HOOKS. Design: cli/nav-pilot/docs/opencode-hooks.md in
// navikt/copilot.
//
// It decides nothing itself. It runs the same commands Copilot CLI runs for the
// same hooks, with the payload Copilot sends, and applies the answer:
//
//   - after a tool call, `nav-pilot hook loop-guard` and then
//     `nav-pilot hook redact` (postToolUse). A modifiedResult replaces what the
//     model reads. Redaction runs last, so nothing a hook adds goes around it.
//   - before a tool call, each gate nav-pilot installed (preToolUse). A
//     permissionDecision of "deny" is thrown, and opencode hands the reason to
//     the model as the tool's result.
//
// Every session, subagents included: tool.execute.before and after fire in a
// child session too, and a secret a subagent reads reaches its model the same
// way. The loop guard skips sessions on the local provider, which have the
// guard proxy in front of the model already.
//
// Failure: redaction fails closed (the output is withheld), everything else
// fails open (the call and its result go through), as under Copilot.
import { spawn } from "node:child_process"

const WITHHELD =
  "[nav-pilot: this tool output was withheld because the redaction hook failed (%s). " +
  "Nothing was redacted, so nothing was shown. Run `nav-pilot doctor`, or turn redaction off with " +
  "`nav-pilot config set hook_redact_secrets false` (and hook_redact_fnr, hook_injection_note).]"

// Not exported: opencode calls every export of a plugin module as a plugin.
// opencode's tools under the names and argument keys Copilot CLI uses, which
// are what the hooks and their matchers are written against.
function toCopilot(tool, args) {
  const a = args ?? {}
  switch (tool) {
    case "bash":
      return ["bash", { command: a.command, description: a.description }]
    case "edit":
      return ["edit", { path: a.filePath, old_str: a.oldString, new_str: a.newString }]
    case "write":
      return ["create", { path: a.filePath, file_text: a.content }]
    case "read":
      return ["view", { path: a.filePath }]
    case "apply_patch":
      return ["apply_patch", { input: a.patchText }]
    default:
      return [tool, a]
  }
}

// run hands payload to a command and resolves with its stdout, or with null
// when it could not start, exited non-zero or outlived its deadline.
function run(argv, payload, ms, cwd, env) {
  return new Promise((resolve) => {
    let out = ""
    let child
    try {
      child = spawn(argv[0], argv.slice(1), { cwd, env: { ...process.env, ...env }, stdio: ["pipe", "pipe", "ignore"] })
    } catch {
      return resolve(null)
    }
    const timer = setTimeout(() => {
      child.kill("SIGKILL")
      resolve(null)
    }, ms)
    child.on("error", () => {
      clearTimeout(timer)
      resolve(null)
    })
    child.stdout.on("data", (d) => (out += d))
    child.on("close", (code) => {
      clearTimeout(timer)
      resolve(code === 0 ? out : null)
    })
    child.stdin.on("error", () => {})
    child.stdin.end(JSON.stringify(payload))
  })
}

// The text the model reads, and how to put a rewrite back: `output` for
// opencode's own tools and task, `content` for an MCP tool.
function texts(output) {
  if (typeof output?.output === "string") return [[output.output, (t) => (output.output = t)]]
  if (Array.isArray(output?.content)) {
    const out = []
    for (const item of output.content) {
      if (item?.type === "text" && typeof item.text === "string") out.push([item.text, (t) => (item.text = t)])
      if (item?.type === "resource" && typeof item.resource?.text === "string")
        out.push([item.resource.text, (t) => (item.resource.text = t)])
    }
    return out
  }
  return []
}

export const NavPilotHooks = async ({ directory, worktree }) => {
  let cfg
  try {
    cfg = JSON.parse(process.env.NAV_PILOT_OPENCODE_HOOKS ?? "")
  } catch {
    return {}
  }
  const post = Array.isArray(cfg?.post) ? cfg.post : []
  const pre = (Array.isArray(cfg?.pre) ? cfg.pre : []).flatMap((h) => {
    try {
      return [{ ...h, re: new RegExp(`^(?:${h.matcher || ".*"})$`) }]
    } catch {
      return []
    }
  })
  const cwd = worktree && worktree !== "/" ? worktree : directory
  const providers = new Map()

  return {
    "chat.params": async (input) => {
      if (input?.sessionID && input?.model?.providerID) providers.set(input.sessionID, input.model.providerID)
    },
    "tool.execute.before": async (input, output) => {
      if (!pre.length) return
      const [toolName, toolArgs] = toCopilot(input.tool, output?.args)
      for (const h of pre) {
        if (!h.re.test(toolName)) continue
        const payload = { sessionId: input.sessionID, toolName, toolArgs, cwd, timestamp: Date.now() }
        const out = await run(["/bin/sh", "-c", h.command], payload, (h.timeout || 5) * 1000, cwd)
        let answer
        try {
          answer = JSON.parse(out || "{}")
        } catch {
          continue
        }
        const decision = answer?.permissionDecision ?? answer?.hookSpecificOutput?.permissionDecision
        if (decision === "deny") {
          throw new Error(
            answer.permissionDecisionReason ?? answer.hookSpecificOutput?.permissionDecisionReason ?? `denied by ${h.name}`,
          )
        }
      }
    },
    "tool.execute.after": async (input, output) => {
      if (!post.length) return
      const [toolName, toolArgs] = toCopilot(input.tool, input.args)
      for (const [text, put] of texts(output)) {
        let current = text
        for (const h of post) {
          if (h.skipLocal && cfg.localProvider && providers.get(input.sessionID) === cfg.localProvider) continue
          const payload = {
            sessionId: input.sessionID,
            toolName,
            toolArgs,
            toolResult: { resultType: "success", textResultForLlm: current },
          }
          const env = h.failClosed ? { NAV_PILOT_HOOK_FAIL_CLOSED: "1" } : {}
          const out = await run(h.argv, payload, (h.timeout || 5) * 1000, cwd, env)
          let answer
          try {
            answer = out === null ? null : JSON.parse(out)
          } catch {
            answer = null
          }
          const rewrite = answer?.modifiedResult?.textResultForLlm
          const ok = answer !== null && typeof answer === "object" && !answer.error
          if (!ok || (answer.modifiedResult && typeof rewrite !== "string")) {
            if (h.failClosed) {
              current = WITHHELD.replace("%s", answer?.error ? String(answer.error) : `${h.name} did not answer`)
              break
            }
            continue
          }
          if (typeof rewrite === "string") current = rewrite
        }
        if (current !== text) put(current)
      }
    },
  }
}
