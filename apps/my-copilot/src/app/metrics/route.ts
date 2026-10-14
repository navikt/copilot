import { connection } from "next/server";
import { register } from "@/lib/metrics";

// Scraped by Prometheus straight from the pod; not in autoLoginIgnorePaths, so
// Wonderwall keeps it off the public ingress.
export async function GET() {
  await connection(); // render per request, never at build time
  return new Response(await register.metrics(), { headers: { "Content-Type": register.contentType } });
}
