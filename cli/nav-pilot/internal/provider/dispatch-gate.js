// nav-pilot local dispatch gate. Written by nav-pilot with its local dispatch
// policy and removed by `nav-pilot alpha local off`. It does nothing unless a
// nav-pilot launch that enforces local_dispatch sets NAV_PILOT_DISPATCH_GATE.
// Design: cli/nav-pilot/docs/local-dispatch.md in navikt/copilot.
//
// It decides nothing. It asks the nav-pilot process that started this session
// and throws the refusal it gets back, which opencode hands to the model as the
// tool's result. After local-worker returns, it appends what the answer says to
// the worker's result, and adds the reminder it may get back when the
// orchestrator writes text. Anything that goes wrong lets the call through.
import { isAbsolute, join } from "node:path"

export const NavPilotDispatchGate = async ({ client, directory }) => {
  const url = process.env.NAV_PILOT_DISPATCH_GATE
  if (!url) return {}
  const agents = new Map()
  const models = new Map()
  const turns = new Map()
  const topLevel = new Map()
  // Only the session the developer talks to. A refusal inside a subagent's
  // session fails that whole task, and the worker's own edits must never be
  // gated. Unknown counts as not top-level for this call, and only a
  // definite answer is kept, so one failed lookup does not end the gate.
  const isTopLevel = async (id) => {
    if (!topLevel.has(id)) {
      try {
        const res = await client.session.get({ path: { id } })
        if (res?.data) topLevel.set(id, !res.data.parentID)
      } catch {}
    }
    return topLevel.get(id) ?? false
  }
  const ask = async (body) => {
    const res = await fetch(url, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(2000),
    })
    return res.ok ? ((await res.json()) ?? {}) : {}
  }
  // The orchestrator's session, or undefined for any other.
  const orchestrator = async (id) => {
    const agent = agents.get(id)
    if (!agent || agent === "local-worker") return undefined
    if (!(await isTopLevel(id))) return undefined
    return agent
  }
  const str = (v) => (typeof v === "string" ? v : "")
  return {
    // One user message is one turn: the gate counts per turn.
    "chat.message": async (input, output) => {
      // A background task's result arrives as a synthetic message: not a turn.
      const parts = output?.parts ?? []
      if (parts.length && parts.every((p) => p?.synthetic)) return
      if (input?.agent) agents.set(input.sessionID, input.agent)
      if (input?.model) models.set(input.sessionID, { model: input.model, variant: input.variant })
      turns.set(input.sessionID, (turns.get(input.sessionID) ?? 0) + 1)
    },
    // Only when chat.message did not name the agent: compaction runs its own
    // agent through chat.params on the same session.
    "chat.params": async (input) => {
      if (input?.agent && !agents.has(input.sessionID)) agents.set(input.sessionID, input.agent)
    },
    "tool.execute.before": async (input, output) => {
      const agent = await orchestrator(input.sessionID)
      if (!agent) return
      const args = output?.args ?? {}
      const replaceAll = input.tool === "edit" && args.replaceAll === true
      const toWorker = input.tool === "task" && args.subagent_type === "local-worker"
      const body = {
        session: input.sessionID,
        turn: turns.get(input.sessionID) ?? 0,
        agent,
        tool: input.tool,
        path: args.filePath ? (isAbsolute(str(args.filePath)) ? str(args.filePath) : join(directory ?? "", str(args.filePath))) : "",
        create: input.tool === "write" || (input.tool === "edit" && args.oldString === ""),
        command: input.tool === "bash" ? str(args.command) : "",
        subagent: input.tool === "task" ? str(args.subagent_type) : "",
        prompt: toWorker ? str(args.prompt) : "",
        replaceAll,
        old: replaceAll ? str(args.oldString) : "",
      }
      let deny = ""
      try {
        deny = (await ask(body)).deny ?? ""
      } catch {
        return
      }
      if (typeof deny === "string" && deny) throw new Error(deny)
    },
    // The worker has returned: the orchestrator reads what to check before it
    // accepts the work, in the task's own result.
    "tool.execute.after": async (input, output) => {
      if (input.tool !== "task" || input.args?.subagent_type !== "local-worker" || !output) return
      try {
        const agent = await orchestrator(input.sessionID)
        if (!agent) return
        const turn = turns.get(input.sessionID) ?? 0
        const text = (await ask({ session: input.sessionID, turn, agent, tool: "task", subagent: "local-worker", phase: "after" })).append
        if (typeof text === "string" && text) output.output = `${str(output.output)}\n\n${text}`
      } catch {}
    },
    // The orchestrator has written text. If the worker's work is still
    // unchecked, nav-pilot answers with a reminder once per turn, added as a
    // message the running session picks up before it ends. Synthetic, so it
    // does not start a turn. Same agent and model, or opencode would switch.
    "experimental.text.complete": async (input) => {
      try {
        const agent = await orchestrator(input.sessionID)
        const m = models.get(input.sessionID)
        if (!agent || !m) return
        const turn = turns.get(input.sessionID) ?? 0
        const text = (await ask({ session: input.sessionID, turn, agent, phase: "text" })).nudge
        if (typeof text !== "string" || !text) return
        await client.session.prompt({
          path: { id: input.sessionID },
          body: { agent, model: m.model, variant: m.variant, noReply: true, parts: [{ type: "text", text, synthetic: true }] },
        })
      } catch {}
    },
  }
}
