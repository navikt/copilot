import { connection } from "next/server";
import { register } from "@/lib/metrics";

// Prometheus scrapes the pod port directly, without a token. Through the
// ingress, /metrics is not in autoLoginIgnorePaths, so Wonderwall only lets a
// logged-in reader through and adds their Authorization header; they get a 404.
// (X-Forwarded-For does not work here: Next.js's own server adds it.)
export async function GET(request: Request) {
  await connection(); // render per request, never at build time
  if (request.headers.has("authorization")) return new Response("Not Found", { status: 404 });
  return new Response(await register.metrics(), { headers: { "Content-Type": register.contentType } });
}
