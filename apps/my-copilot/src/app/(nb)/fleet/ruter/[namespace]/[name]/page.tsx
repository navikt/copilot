import { notFound, permanentRedirect } from "next/navigation";
import { isName } from "@/lib/fleet-names";
import { requireFleet } from "@/lib/fleet-control";

// An MCP server's page moved from «ruter»: a route is only its gateway address (#2748). 308, so old links keep working.
type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function OldRoutePage({ params }: Props) {
  requireFleet();
  const { namespace, name } = await params;
  if (!isName(namespace) || !isName(name)) notFound();
  permanentRedirect(`/fleet/mcp-servere/${namespace}/${name}`);
}
