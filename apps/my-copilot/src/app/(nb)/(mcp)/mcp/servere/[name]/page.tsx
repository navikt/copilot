import type { Metadata } from "next";
import NextLink from "next/link";
import { notFound } from "next/navigation";
import { BodyShort, HStack, Tag, VStack } from "@navikt/ds-react";
import { runsWhere } from "@/lib/mcp-labels";
import { McpDetails } from "@/components/detail-drawer/mcp-details";
import { isFleetGatewayMcp, isLocalMcp, riskCounts, serverPagePath } from "@/components/detail-drawer/mcp-meta";
import { DocPage, linkClass } from "@/components/nav-pilot/doc-page";
import { enrichWithUsage } from "@/lib/enrich-customizations";
import { getMcpServers } from "@/lib/mcp-registry";
import { itemKey } from "@/lib/item-key";

interface Props {
  params: Promise<{ name: string }>;
}

// Only gateway servers get a page of their own (#2822); the rest stay in the /verktoy drawer.
async function findServer(name: string) {
  const servers = enrichWithUsage(await getMcpServers(), []);
  return servers.find(
    (s) => s.type === "mcp" && isFleetGatewayMcp(s) && serverPagePath(s.serverId) === `/mcp/servere/${name}`
  );
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const server = await findServer((await params).name);
  return server ? { title: `${server.name} – MCP-server`, description: server.description } : {};
}

export default async function McpServerPage({ params }: Props) {
  const server = await findServer((await params).name);
  if (!server || server.type !== "mcp") notFound();
  const tools = server.tools ?? [];

  return (
    <DocPage
      label="MCP-server"
      title={server.name}
      description={server.description}
      badge={
        <HStack gap="space-6" wrap>
          {tools.length > 0 && (
            <Tag size="small" variant="neutral">
              {riskCounts(tools, server.toolRisk)}
            </Tag>
          )}
          <Tag size="small" variant="info">
            Nav-innlogging
          </Tag>
          <BodyShort size="small">{runsWhere(isLocalMcp(server))}</BodyShort>
        </HStack>
      }
    >
      <VStack gap="space-16" className="mcp-server-page">
        <McpDetails item={server} />
        <BodyShort size="small">
          Versjon {server.version}.{" "}
          <NextLink href={`/verktoy?item=${encodeURIComponent(itemKey(server))}`} className={linkClass}>
            Se serveren i verktøykatalogen
          </NextLink>
        </BodyShort>
      </VStack>
    </DocPage>
  );
}
