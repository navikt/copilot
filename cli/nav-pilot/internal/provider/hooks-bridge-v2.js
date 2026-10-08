// nav-pilot hooks for opencode 2. hooks-bridge.js is the same bridge for
// opencode 1; this one speaks opencode 2's plugin API (a default export with an
// id and a setup function, hooks registered per domain). Loaded only by a
// nav-pilot launch, which names this file's directory in
// OPENCODE_CONFIG_CONTENT and hands it the hooks to run in
// NAV_PILOT_OPENCODE_HOOKS. Design: cli/nav-pilot/docs/opencode-hooks.md.
//
// It decides nothing itself. It runs the same commands Copilot CLI runs for the
// same hooks, with the payload Copilot sends, and applies the answer:
//
//   - after a tool call (tool execute.after), `nav-pilot hook loop-guard` and
//     then `nav-pilot hook redact`. A modifiedResult replaces what the model
//     reads. Redaction runs last, so nothing a hook adds goes around it.
//   - before a tool call (tool execute.before), each gate nav-pilot installed.
//     A permissionDecision of "deny" is thrown, and opencode hands the reason
//     to the model as the tool's result.
//
// opencode 2 runs both hooks for a tool a Code Mode program (`execute`) calls,
// under that tool's own name, and for a subagent's session.
//
// With NAV_PILOT_DISPATCH_GATE set, it also runs the local dispatch gate
// (internal/local/gate.go), as dispatch-gate.js does on opencode 1: it asks
// the gate before the orchestrator's edit, write, shell or subagent call and
// throws a refusal, appends what the gate answers to the worker's result and
// to a check after it, and adds the gate's reminder, once the orchestrator has
// written text, to the session as a synthetic message it reads before the turn
// ends.
//
// Failure: redaction fails closed (the output is withheld), everything else
// fails open (the call and its result go through), as under Copilot.
import { spawn } from "node:child_process"
import { isAbsolute, join } from "node:path"

const WITHHELD =
  "[nav-pilot: this tool output was withheld because the redaction hook failed (%s). " +
  "Nothing was redacted, so nothing was shown. Tell the user, and suggest `nav-pilot doctor`.]"

// opencode 2's tools under the names and argument keys Copilot CLI uses.
function toCopilot(tool, args) {
  const a = args ?? {}
  switch (tool) {
    case "shell":
      return ["bash", { command: a.command, description: a.description }]
    case "edit":
      return ["edit", { path: a.path, old_str: a.oldString, new_str: a.newString }]
    case "write":
      return ["create", { path: a.path, file_text: a.content }]
    case "read":
      return ["view", { path: a.path }]
    case "patch":
      return ["apply_patch", { input: a.patchText }]
    default:
      return [tool, a]
  }
}

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
    child.stdout.setEncoding("utf8")
    child.stdout.on("data", (d) => (out += d))
    child.on("close", (code) => {
      clearTimeout(timer)
      resolve(code === 0 ? out : null)
    })
    child.stdin.on("error", () => {})
    child.stdin.end(JSON.stringify(payload))
  })
}

// The text items of a result's content, each with a way to put a rewrite back.
function texts(result) {
  if (typeof result.content === "string") return [[result.content, (t) => (result.content = t)]]
  const out = []
  for (const item of Array.isArray(result.content) ? result.content : []) {
    if (item?.type === "text" && typeof item.text === "string") out.push([item.text, (t) => (item.text = t)])
    if (item?.type === "resource" && typeof item.resource?.text === "string")
      out.push([item.resource.text, (t) => (item.resource.text = t)])
  }
  return out
}

// The local dispatch gate. Its questions and answers are dispatch-gate.js's,
// with opencode 2's tool names and arguments mapped to opencode 1's, which the
// gate matches on. Every hook is a no-op without the gate's address.
function dispatchGate(ctx, directory) {
  const url = process.env.NAV_PILOT_DISPATCH_GATE
  if (!url) return undefined
  const turns = new Map()
  const topLevel = new Map()
  const str = (v) => (typeof v === "string" ? v : "")
  const abs = (p) => (p ? (isAbsolute(p) ? p : join(directory, p)) : "")
  // Only the session the developer talks to; see dispatch-gate.js. Unknown
  // is not top-level, and only a definite answer is kept.
  const isTopLevel = async (id) => {
    if (!topLevel.has(id)) {
      try {
        const s = await ctx.session.get({ sessionID: id })
        if (s && typeof s === "object") topLevel.set(id, !s.parentID)
      } catch {}
    }
    return topLevel.get(id) ?? false
  }
  const orchestrator = async (sessionID, agent) =>
    agent && agent !== "local-worker" && (await isTopLevel(sessionID)) ? agent : undefined
  const ask = async (body) => {
    const res = await fetch(url, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(2000),
    })
    return res.ok ? ((await res.json()) ?? {}) : {}
  }
  const agents = new Map()
  // The orchestrator has written text. If the worker's work is still
  // unchecked, the gate answers with a reminder once per turn, added as a
  // synthetic steer: the running session takes it at its next step, and a
  // session that has gone idle wakes for it, so the model reads it before the
  // turn ends, as dispatch-gate.js does on opencode 1.
  ;(async () => {
    try {
      for await (const e of ctx.event.subscribe()) {
        if (e?.type !== "session.text.ended" || !e.data?.sessionID) continue
        const id = e.data.sessionID
        ;(async () => {
          const agent = await orchestrator(id, agents.get(id))
          if (!agent) return
          const text = (await ask({ session: id, turn: turns.get(id) ?? 0, agent, phase: "text" })).nudge
          if (typeof text === "string" && text) await ctx.session.synthetic({ sessionID: id, text, delivery: "steer" })
        })().catch(() => {})
      }
    } catch {}
  })()
  return {
    // One user prompt is one turn: the gate counts per turn.
    prompt: (event) => {
      turns.set(event.sessionID, (turns.get(event.sessionID) ?? 0) + 1)
    },
    context: (event) => {
      if (event.agent) agents.set(event.sessionID, event.agent)
    },
    before: async (event) => {
      const tool = event.tool
      if (!["edit", "write", "shell", "subagent"].includes(tool)) return
      const args = event.input ?? {}
      if (event.agent) agents.set(event.sessionID, event.agent)
      const create = tool === "write" || (tool === "edit" && args.oldString === "")
      let deny = ""
      try {
        const agent = await orchestrator(event.sessionID, event.agent)
        if (!agent) {
          // The worker's new files: see dispatch-gate.js. Never a refusal.
          if (event.agent === "local-worker" && create)
            await ask({ session: event.sessionID, agent: "local-worker", tool, create: true, path: abs(str(args.path)), phase: "worker" })
          return
        }
        const replaceAll = tool === "edit" && args.replaceAll === true
        const toWorker = tool === "subagent" && args.agent === "local-worker"
        deny = (
          await ask({
            session: event.sessionID,
            turn: turns.get(event.sessionID) ?? 0,
            agent,
            tool: tool === "shell" ? "bash" : tool === "subagent" ? "task" : tool,
            path: abs(str(args.path)),
            create,
            command: tool === "shell" ? str(args.command) : "",
            subagent: tool === "subagent" ? str(args.agent) : "",
            prompt: toWorker ? str(args.prompt) : "",
            replaceAll,
            old: replaceAll ? str(args.oldString) : "",
          })
        ).deny
      } catch {
        return
      }
      if (typeof deny === "string" && deny) throw new Error(deny)
    },
    // The text to append to the worker's result or to a check after it.
    after: async (event) => {
      const toWorker = event.tool === "subagent" && event.input?.agent === "local-worker" && event.input?.background !== true
      if (!toWorker && event.tool !== "shell") return ""
      try {
        const agent = await orchestrator(event.sessionID, event.agent)
        if (!agent) return ""
        const turn = turns.get(event.sessionID) ?? 0
        const exit = event.result?.metadata?.exit
        const body = toWorker
          ? { session: event.sessionID, turn, agent, tool: "task", subagent: "local-worker", worker: str(event.result?.metadata?.sessionID), phase: "after" }
          : { session: event.sessionID, turn, agent, tool: "bash", command: str(event.input?.command), exit: typeof exit === "number" ? exit : null, phase: "after" }
        const text = (await ask(body)).append
        return typeof text === "string" ? text : ""
      } catch {
        return ""
      }
    },
  }
}

export default {
  // Per launch (NAV_PILOT_OPENCODE_PLUGIN_ID): opencode 2 drops a plugin
  // whose id another plugin loaded first.
  id: process.env.NAV_PILOT_OPENCODE_PLUGIN_ID || "nav-pilot-hooks",
  setup: async (ctx) => {
    let cfg = {}
    try {
      cfg = JSON.parse(process.env.NAV_PILOT_OPENCODE_HOOKS ?? "") ?? {}
    } catch {}
    // MCP servers Nav's registry does not list (#1027); see hooks-bridge.js.
    // opencode 2 names an MCP tool <server>_<tool> as opencode 1 does.
    const prefix = (n) => String(n).replace(/[^a-zA-Z0-9_-]/g, "_") + "_"
    let servers = []
    try {
      const m = JSON.parse(process.env.NAV_PILOT_MCP_BLOCKED ?? "{}")
      servers = [
        ...(m.blocked ?? []).map((n) => ({ p: prefix(n), name: n, blocked: true })),
        ...(m.listed ?? []).map((n) => ({ p: prefix(n), name: n, blocked: false })),
      ].sort((a, b) => b.p.length - a.p.length)
    } catch {}
    // ponytail: opencode 2.0.24's own tools, by hand, as in hooks-bridge.js.
    const builtin = new Set(["shell", "read", "edit", "write", "patch", "glob", "grep", "subagent", "webfetch", "websearch", "skill", "question", "execute", "invalid", "opencode_session_rename", "opencode_session_move", "opencode_models"])
    const resourceTools = new Set(["list_mcp_resources", "list_mcp_resource_templates", "read_mcp_resource"])
    const blockedServer = (tool, args) => {
      if (resourceTools.has(tool)) return servers.find((s) => s.blocked && s.name === args?.server)?.name
      if (builtin.has(tool)) return undefined
      const hit = servers.find((s) => String(tool).startsWith(s.p))
      return hit?.blocked ? hit.name : undefined
    }
    const post = Array.isArray(cfg?.post) ? cfg.post : []
    const pre = (Array.isArray(cfg?.pre) ? cfg.pre : []).flatMap((h) => {
      try {
        return [{ ...h, re: new RegExp(`^(?:${h.matcher || ".*"})$`) }]
      } catch {
        return []
      }
    })
    const loc = ctx.location ?? {}
    const cwd = loc.worktree && loc.worktree !== "/" ? loc.worktree : (loc.directory ?? process.cwd())
    const providers = new Map()
    const redact = post.filter((h) => h.failClosed)
    const seen = new Map()
    const done = new Set()
    const gate = dispatchGate(ctx, loc.directory ?? cwd)
    if (gate) {
      await ctx.session.hook("prompt", gate.prompt)
      await ctx.session.hook("context", gate.context)
    }

    const postHooks = async (sessionID, tool, args, text, hooks = post) => {
      const [toolName, toolArgs] = toCopilot(tool, args)
      let current = text
      for (const h of hooks) {
        if (h.skipLocal && cfg.localProvider && providers.get(sessionID) === cfg.localProvider) continue
        const payload = { sessionId: sessionID, toolName, toolArgs, toolResult: { resultType: "success", textResultForLlm: current } }
        const env = h.failClosed ? { NAV_PILOT_HOOK_FAIL_CLOSED: "1" } : {}
        const out = await run(h.argv, payload, (h.timeout || 5) * 1000, cwd, env)
        let answer
        try {
          answer = out === null ? null : JSON.parse(out)
        } catch {
          answer = null
        }
        const rewrite = answer?.modifiedResult?.textResultForLlm
        const ok = answer !== null && typeof answer === "object" && !Array.isArray(answer) && !answer.error
        if (!ok || (answer.modifiedResult && typeof rewrite !== "string")) {
          if (h.failClosed) return WITHHELD.replace("%s", answer?.error ? String(answer.error) : `${h.name} did not answer`)
          continue
        }
        if (typeof rewrite === "string") current = rewrite
      }
      return current
    }

    const redactOnce = async (sessionID, id, tool, text) => {
      const key = id + "\u0000" + text
      if (!seen.has(key)) {
        const out = await postHooks(sessionID, tool, undefined, text, redact)
        seen.set(key, out)
        seen.set(id + "\u0000" + out, out)
      }
      return seen.get(key)
    }

    await ctx.session.hook("model.request", async (event) => {
      if (event?.sessionID && event?.model?.providerID) providers.set(event.sessionID, event.model.providerID)
    })

    await ctx.tool.hook("execute.before", async (event) => {
      const server = blockedServer(event.tool, event.input)
      if (server)
        throw new Error(
          `nav-pilot: the MCP server ${server} is not in Nav's MCP registry, so its tools are turned off in this session. Tell the user.`,
        )
      if (gate) await gate.before(event)
      if (!pre.length) return
      const [toolName, toolArgs] = toCopilot(event.tool, event.input)
      for (const h of pre) {
        if (!h.re.test(toolName)) continue
        const payload = { sessionId: event.sessionID, toolName, toolArgs, cwd, timestamp: Date.now() }
        const out = await run(["/bin/sh", "-c", h.command], payload, (h.timeout || 5) * 1000, cwd)
        // A gate marked failClosed denies when it cannot answer. The command
        // prints its own deny when the script is killed or fails, so null
        // here means sh itself did not start or finish in time. The others
        // allow.
        if (out === null && h.failClosed) throw new Error(`${h.name} feilet eller svarte ikke innen fristen, så kallet er stoppet`)
        let answer
        try {
          answer = JSON.parse(out || "{}")
        } catch {
          if (h.failClosed) throw new Error(`${h.name} feilet, så kallet er stoppet`)
          continue
        }
        const decision = answer?.permissionDecision ?? answer?.hookSpecificOutput?.permissionDecision
        if (decision === "deny" || decision === "ask") {
          throw new Error(
            answer.permissionDecisionReason ?? answer.hookSpecificOutput?.permissionDecisionReason ?? `denied by ${h.name}`,
          )
        }
      }
    })

    await ctx.tool.hook("execute.after", async (event) => {
      // Before redaction, so nothing goes around it.
      if (gate && event.status !== "error" && event.result) {
        const add = await gate.after(event)
        const items = add ? texts(event.result) : []
        if (items.length) items.at(-1)[1](`${items.at(-1)[0]}\n\n${add}`)
        else if (add) event.result.content = [...(Array.isArray(event.result.content) ? event.result.content : []), { type: "text", text: add }]
      }
      if (!post.length) return
      if (event.status === "error") {
        // A tool that failed: its message reaches the model. Redaction only,
        // as the loop guard counts results, not failures.
        if (redact.length && typeof event.error?.message === "string")
          event.error.message = await redactOnce(event.sessionID, event.id, event.tool, event.error.message)
        return
      }
      const result = event.result
      if (!result) return
      done.add(event.id)
      // Once per tool call, as Copilot sends one result: the text items are
      // joined, and a rewrite goes into the first item and empties the rest.
      const items = texts(result)
      const text = items.map(([t]) => t).join("\n\n")
      let next = text
      if (items.length) {
        next = await postHooks(event.sessionID, event.tool, event.input, text)
        if (next !== text) items.forEach(([, put], i) => put(i === 0 ? next : ""))
      }
      // The structured output is what a Code Mode program reads from the
      // call, and what the model reads when there is no text content.
      if (result.output === undefined || !redact.length) return
      if (result.output === text) return void (result.output = next)
      const raw = typeof result.output === "string" ? result.output : JSON.stringify(result.output)
      if (typeof raw !== "string") return
      const clean = await postHooks(event.sessionID, event.tool, event.input, raw, redact)
      if (clean === raw) return
      if (typeof result.output === "string") return void (result.output = clean)
      try {
        result.output = JSON.parse(clean)
      } catch {
        result.output = clean
      }
    })

    // The last hop before every model call. A tool result execute.after did
    // not see reaches the model here: an error a hook or the runtime raised,
    // a call the user interrupted. Redaction runs over it, once per text.
    const scrub = async (event) => {
      if (!redact.length) return
      for (const msg of event?.messages ?? []) {
        if (msg?.role !== "tool" || !Array.isArray(msg.content)) continue
        for (const part of msg.content) {
          if (part?.type !== "tool-result" || done.has(part.id) || !part.result) continue
          const r = part.result
          if (typeof r.value === "string") r.value = await redactOnce(event.sessionID, part.id, part.name, r.value)
          else if (typeof r.value?.error?.message === "string")
            r.value.error.message = await redactOnce(event.sessionID, part.id, part.name, r.value.error.message)
        }
      }
    }
    // Every model call: the agent's turn, compaction, a generate call, and
    // the title (core/src/session/model-request.ts at v2.0.24).
    for (const name of ["context", "compaction", "generate", "title"]) await ctx.session.hook(name, scrub)
  },
}
