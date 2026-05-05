/**
 * Mock Texas token introspection server for local development.
 *
 * Simulates the Nais Texas sidecar to test authentication flows locally
 * without needing Azure AD or a real Wonderwall setup.
 *
 * Usage:
 *   npx tsx scripts/mock-texas.ts
 *
 * Then start the app with:
 *   NAIS_TOKEN_INTROSPECTION_ENDPOINT=http://localhost:6969/introspect pnpm dev
 *
 * Any token sent to the introspection endpoint will be treated as valid
 * and return a mock Nav user. Send "expired" or "invalid" as the token
 * to test error cases.
 */

const MOCK_USER = {
  name: "Flaatten, Hans Kristian",
  preferred_username: "hans.kristian.flaatten@nav.no",
  groups: [
    "48120347-8582-4329-8673-7beb3ed6ca06",
    "76e9ee7e-2cd1-4814-b199-6c0be007d7b4",
  ],
  sub: "mock-subject-id",
  aud: "mock-client-id",
  iss: "https://login.microsoftonline.com/mock-tenant/v2.0",
  iat: Math.floor(Date.now() / 1000),
  exp: Math.floor(Date.now() / 1000) + 3600,
};

const port = Number(process.env.PORT ?? 6969);

// Node.js http server
import { createServer } from "node:http";

const httpServer = createServer(async (req, res) => {
  if (req.method === "POST" && req.url === "/introspect") {
    const chunks: Buffer[] = [];
    for await (const chunk of req) {
      chunks.push(chunk as Buffer);
    }
    const body = JSON.parse(Buffer.concat(chunks).toString());

    const token = body.token ?? "";

    // Simulate error cases
    if (token === "expired") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ active: false, error: "token is expired" }));
      console.log("← 200 inactive (expired token)");
      return;
    }

    if (token === "invalid") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ active: false, error: "invalid signature" }));
      console.log("← 200 inactive (invalid token)");
      return;
    }

    // Valid token response
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ active: true, ...MOCK_USER }));
    console.log(`← 200 active (user: ${MOCK_USER.preferred_username})`);
    return;
  }

  // Health check
  if (req.url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok" }));
    return;
  }

  res.writeHead(404);
  res.end("Not found");
});

httpServer.listen(port, () => {
  console.log(`\n🤠 Mock Texas introspection server running on http://localhost:${port}/introspect`);
  console.log(`\nTo use with the app:`);
  console.log(`  NAIS_TOKEN_INTROSPECTION_ENDPOINT=http://localhost:${port}/introspect pnpm dev\n`);
  console.log(`Test tokens:`);
  console.log(`  Any value     → valid Nav user`);
  console.log(`  "expired"     → inactive (expired)`);
  console.log(`  "invalid"     → inactive (bad signature)\n`);
});
