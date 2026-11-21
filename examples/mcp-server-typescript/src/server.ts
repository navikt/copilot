import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { createMcpHandler, McpServer, type AuthInfo } from "@modelcontextprotocol/server";
import { toNodeHandler } from "@modelcontextprotocol/node";
import * as z from "zod/v4";
import { exchangeToken, extractBearer, Forbidden, NotConfigured, Unauthorized, validateToken, type AuthConfig } from "./auth.js";

export const MAX_REQUEST_BYTES = 1 << 20;
export const MAX_RESPONSE_BYTES = 32 * 1024; // downstream body and tool output cap

export interface Config extends AuthConfig {
  downstreamUrl: string;
  downstreamScope: string;
}

/** Calls per tool and outcome. Bounded cardinality: tool names are fixed, outcome is ok|error. */
export class Metrics {
  private calls = new Map<string, number>();
  private seconds = new Map<string, number>();
  observe(tool: string, outcome: "ok" | "error", sec: number) {
    const k = `tool="${tool}",outcome="${outcome}"`;
    this.calls.set(k, (this.calls.get(k) ?? 0) + 1);
    this.seconds.set(k, (this.seconds.get(k) ?? 0) + sec);
  }
  render(): string {
    const lines = (name: string, m: Map<string, number>) => [...m].map(([k, v]) => `${name}{${k}} ${v}`);
    return [
      "# TYPE mcp_tool_calls_total counter", ...lines("mcp_tool_calls_total", this.calls),
      "# TYPE mcp_tool_duration_seconds_sum counter", ...lines("mcp_tool_duration_seconds_sum", this.seconds),
    ].join("\n") + "\n";
  }
}

function truncate(s: string): string {
  const b = Buffer.from(s);
  return b.length <= MAX_RESPONSE_BYTES ? s : b.subarray(0, MAX_RESPONSE_BYTES).toString() + "\n[truncated]";
}

function buildMcpServer(cfg: Config, metrics: Metrics, auth?: AuthInfo): McpServer {
  const mcp = new McpServer({ name: "mcp-example", version: "0.1.0" });
  mcp.registerTool(
    "list_codes",
    {
      description: "List the codes of one kodeverk by name. Read-only.",
      inputSchema: z.object({ kodeverksnavn: z.string().regex(/^[A-Za-z0-9_-]{1,64}$/) }),
      annotations: { readOnlyHint: true, destructiveHint: false, idempotentHint: true, openWorldHint: false },
    },
    async ({ kodeverksnavn }) => {
      const t0 = performance.now();
      try {
        if (!auth) throw new Error("missing auth context");
        const obo = await exchangeToken(cfg, auth.token, cfg.downstreamScope);
        const res = await cfg.fetch(`${cfg.downstreamUrl}/api/v1/kodeverk/${encodeURIComponent(kodeverksnavn)}/koder`, {
          headers: { Authorization: `Bearer ${obo}`, Accept: "application/json" },
        });
        if (!res.ok) throw new Error(`downstream returned status ${res.status}`);
        // Read at most cap+1 bytes: a huge body must not be buffered whole.
        const reader = res.body!.getReader();
        const chunks: Uint8Array[] = [];
        let n = 0;
        while (n <= MAX_RESPONSE_BYTES) {
          const { done, value } = await reader.read();
          if (done) break;
          chunks.push(value);
          n += value.length;
        }
        void reader.cancel();
        metrics.observe("list_codes", "ok", (performance.now() - t0) / 1000);
        return { content: [{ type: "text" as const, text: truncate(Buffer.concat(chunks).toString()) }] };
      } catch (e) {
        metrics.observe("list_codes", "error", (performance.now() - t0) / 1000);
        // Fixed message: upstream bodies and tokens must not reach the model or the logs.
        console.warn(JSON.stringify({ msg: "tool failed", tool: "list_codes", error: (e as Error).message }));
        return { isError: true, content: [{ type: "text" as const, text: "Downstream lookup failed." }] };
      }
    },
  );
  return mcp;
}

export function createApp(cfg: Config): { server: Server; metrics: Metrics } {
  const metrics = new Metrics();
  const handler = createMcpHandler((ctx) => buildMcpServer(cfg, metrics, ctx.authInfo), {
    maxRequestBodySize: MAX_REQUEST_BYTES,
    onerror: (e) => console.warn(JSON.stringify({ msg: "mcp error", error: e.message })),
  });
  const mcp = toNodeHandler(handler, { maxRequestBodySize: MAX_REQUEST_BYTES });

  const json = (res: ServerResponse, status: number, body: object) => {
    res.writeHead(status, { "Content-Type": "application/json" }).end(JSON.stringify(body));
  };

  const server = createServer(async (req: IncomingMessage, res: ServerResponse) => {
    if (req.url === "/health" || req.url === "/ready") return void res.writeHead(200).end("ok");
    if (req.url === "/metrics") return void res.writeHead(200, { "Content-Type": "text/plain" }).end(metrics.render());

    const token = extractBearer(req.headers.authorization);
    if (!token) return json(res, 401, { error: "invalid_token", message: "missing or malformed bearer token" });
    let navIdent: string;
    try {
      navIdent = await validateToken(cfg, token);
    } catch (e) {
      if (e instanceof Unauthorized) return json(res, 401, { error: "invalid_token", message: e.message });
      if (e instanceof Forbidden) return json(res, 403, { error: "forbidden", message: e.message });
      if (e instanceof NotConfigured) return json(res, 503, { error: "unavailable", message: "authentication is not configured" });
      console.warn(JSON.stringify({ msg: "texas introspection failed", error: (e as Error).message }));
      return json(res, 502, { error: "introspection_failed", message: "failed to validate token" });
    }
    if (req.method !== "POST") return json(res.setHeader("Allow", "POST"), 405, { error: "method_not_allowed" });
    const len = Number(req.headers["content-length"] ?? 0);
    if (len > MAX_REQUEST_BYTES) return json(res, 413, { error: "payload_too_large" });

    const info: AuthInfo = { token, clientId: cfg.gatewayClientId, scopes: [], extra: { navIdent } };
    await mcp(Object.assign(req, { auth: info }), res);
  });
  return { server, metrics };
}
