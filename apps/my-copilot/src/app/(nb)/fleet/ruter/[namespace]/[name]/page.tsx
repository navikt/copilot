import { permanentRedirect } from "next/navigation";

// An MCP server's page moved from «ruter»: a route is only its gateway address (#2748). 308, so old links keep working.
type Props = { params: Promise<{ namespace: string; name: string }> };

export default async function OldRoutePage({ params }: Props) {
  const { namespace, name } = await params;
  permanentRedirect(`/fleet/mcp-servere/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`);
}
