import { SectionLayout } from "@/components/navigation/site-nav";

// Route group: the /mcp subpages share the MCP section menu. The landing page
// /mcp sits outside it, so its hero runs the full width.
export default function McpSectionLayout({ children }: { children: React.ReactNode }) {
  return <SectionLayout>{children}</SectionLayout>;
}
