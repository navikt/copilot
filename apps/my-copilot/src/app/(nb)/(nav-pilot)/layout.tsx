import { SectionLayout } from "@/components/navigation/site-nav";

// Route group: /kom-i-gang, /verktoy, /cplt and the nav-pilot subpages share
// the section menu. The group changes no URLs. The landing page /nav-pilot sits
// outside it, so its hero runs the full width.
export default function NavPilotSectionLayout({ children }: { children: React.ReactNode }) {
  return <SectionLayout>{children}</SectionLayout>;
}
