import { SectionLayout } from "@/components/navigation/site-nav";

// Route group: /kom-i-gang, /verktoy and the nav-pilot subpages share the
// section menu. The group changes no URLs. The landing page /nav-pilot sits
// outside it, so its hero runs the full width. /cplt is under (en), with no
// section menu: it is the cplt project's own landing page.
export default function NavPilotSectionLayout({ children }: { children: React.ReactNode }) {
  return <SectionLayout>{children}</SectionLayout>;
}
