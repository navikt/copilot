// my-copilot's side of the fleet-control contract (#3087). The golden files come
// from fleet-control's handlers; a renamed or retyped field fails here.
import agentList from "../../../fleet-control/testdata/contract/agentprofile-list.json";
import agentPending from "../../../fleet-control/testdata/contract/agentprofile-pending.json";
import agentReady from "../../../fleet-control/testdata/contract/agentprofile-ready.json";
import serverList from "../../../fleet-control/testdata/contract/mcpserver-list.json";
import serverPending from "../../../fleet-control/testdata/contract/mcpserver-pending.json";
import serverPublished from "../../../fleet-control/testdata/contract/mcpserver-published.json";
import reasons from "../../../fleet-control/internal/api/reasons.json";
import type { AgentProfile, List, MCPServer } from "./fleet-control";
import { golden } from "@/test/fake-fleet-control";

// tsc checks required fields and types; the expects below check the optional ones the pages read.
const pending: MCPServer = serverPending;
const published: MCPServer = serverPublished;
const servers: List<MCPServer> = serverList;
const agentWaiting: AgentProfile = agentPending;
const agent: AgentProfile = agentReady;
const agents: List<AgentProfile> = agentList;

const cond = (c: { type: string; reason: string }[] | undefined, type: string) => c?.find((x) => x.type === type);

describe("fleet-control contract", () => {
  it("reads a pending and a published MCPServer", () => {
    expect(pending.metadata).toMatchObject({ name: "svc", namespace: "team-a", generation: 1 });
    expect(pending.metadata.resourceVersion).toEqual(expect.any(String));
    expect(pending.spec?.upstream?.app).toBe("svc");
    expect(pending.spec?.allowedTools).toEqual(["fetch", "search"]);
    expect(pending.status.pending).toMatchObject({ generation: 1, submittedBy: { name: "A111111" } });
    expect(pending.status.pending?.contentHash).toMatch(/^v1:sha256:/);
    expect(pending.status.pending?.expiresAt).toEqual(expect.any(String));
    expect(cond(pending.status.conditions, "PendingApproval")?.reason).toEqual(expect.any(String));
    expect(published.status.publishedGeneration).toBe(1);
    expect(published.status.route?.path).toBe("/mcp/team-a/svc");
    expect(cond(published.status.conditions, "Ready")).toMatchObject({ status: "True" });
    expect(servers.items.map((s) => s.metadata.name)).toEqual(["svc"]);
  });

  it("reads a pending and a ready AgentProfile", () => {
    expect(agentWaiting.status.pending?.generation).toBe(1);
    expect(agentWaiting.status.grants?.[0]).toMatchObject({ granted: false, serverRef: { name: "kodeverk" } });
    expect(agentWaiting.status.grants?.[0].contentHash).toMatch(/^v1:sha256:/);
    expect(agent.spec.grants?.[0]).toMatchObject({ allowedTools: ["lookup"], maxRisk: "Read" });
    expect(agent.spec.dpia).toMatchObject({ owner: "team-c" });
    expect(agent.status.grants?.[0]).toMatchObject({ granted: true, grantedBy: { name: "B444444" } });
    expect(agent.status.grants?.[0].grantedAt).toEqual(expect.any(String));
    expect(agent.status.expiresAt).toEqual(expect.any(String));
    expect(cond(agent.status.conditions, "Ready")).toMatchObject({ status: "True" });
    expect(agents.items.map((a) => a.metadata.name)).toEqual(["assistant"]);
  });

  it("reads reason from the error body, and the reason is pinned", () => {
    const st = golden("status-already-exists.json") as { kind: string; reason: string; message: string };
    expect(st).toMatchObject({ kind: "Status", code: 409 });
    expect(reasons).toContain(st.reason);
  });
});
