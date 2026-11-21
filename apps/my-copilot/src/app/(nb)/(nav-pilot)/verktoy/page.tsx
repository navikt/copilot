import { Suspense } from "react";
import type { Metadata } from "next";
import { Box } from "@navikt/ds-react";
import { getAllCustomizations, getCustomizationById } from "@/lib/customizations";
import { TYPE_LABELS } from "@/lib/customization-types";
import { CustomizationCatalog } from "@/components/customization-catalog";
import { getMcpServers } from "@/lib/mcp-registry";
import { getCustomizationUsage } from "@/lib/cached-bigquery";
import { getUserToken, getUser } from "@/lib/auth";
import { enrichWithUsage } from "@/lib/enrich-customizations";
import type { CustomizationUsage } from "@/lib/types";

interface Props {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export async function generateMetadata({ searchParams }: Props): Promise<Metadata> {
  const params = await searchParams;
  const itemId = typeof params.item === "string" ? params.item : undefined;

  if (itemId) {
    const item = getCustomizationById(itemId);
    if (item) {
      const typeLabel = TYPE_LABELS[item.type];
      const title = `${item.name} — ${typeLabel}`;
      const description = item.description;

      return {
        title,
        description,
        openGraph: {
          title: `${item.name} — ${typeLabel} for GitHub Copilot`,
          description,
          type: "website",
        },
        twitter: {
          card: "summary_large_image",
          title: `${item.name} — ${typeLabel} for GitHub Copilot`,
          description,
        },
      };
    }
  }

  return {
    title: "Verktøykatalog",
    description: "Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack.",
    openGraph: {
      title: "Verktøykatalog",
      description: "Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack.",
      type: "website",
    },
    twitter: {
      card: "summary_large_image",
      title: "Verktøykatalog",
      description: "Agenter, instruksjoner, skills og MCP-servere som gjør GitHub Copilot smartere for Navs stack.",
    },
  };
}

export default async function CustomizationsPage() {
  const user = await getUser(false);
  const customizations = getAllCustomizations();
  const token = user ? await getUserToken() : null;
  const [mcpServers, usageResult] = await Promise.all([
    getMcpServers(),
    token ? getCustomizationUsage(token) : Promise.resolve({ usage: [] as CustomizationUsage[], error: null }),
  ]);
  const items = [...customizations, ...mcpServers].sort((a, b) => a.name.localeCompare(b.name, "nb"));
  const enrichedItems = enrichWithUsage(items, usageResult.usage);

  return (
    <main id="hovedinnhold" tabIndex={-1} className="verktoy-page">
      <div className="max-w-6xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-20", md: "space-32" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <Suspense>
            <CustomizationCatalog items={enrichedItems} />
          </Suspense>
        </Box>
      </div>
    </main>
  );
}
