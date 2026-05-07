import {
  BookIcon,
  InformationSquareIcon,
  LineGraphIcon,
  PieChartIcon,
  PlayIcon,
  RocketIcon,
  ShieldLockIcon,
  TerminalIcon,
  WrenchIcon,
} from "@navikt/aksel-icons";

export type NavItem = {
  href: string;
  icon: typeof BookIcon;
  label: string;
  requiresAuth?: boolean;
};

export const NAV_ITEMS: NavItem[] = [
  { href: "/kom-i-gang", icon: PlayIcon, label: "Kom i gang" },
  { href: "/praksis", icon: BookIcon, label: "God praksis" },
  { href: "/verktoy", icon: WrenchIcon, label: "Verktøy" },
  { href: "/retningslinjer", icon: ShieldLockIcon, label: "Retningslinjer" },
  { href: "/nav-pilot", icon: RocketIcon, label: "nav-pilot" },
  { href: "/ordliste", icon: BookIcon, label: "Ordlista" },
  { href: "/cplt", icon: TerminalIcon, label: "cplt" },
  { href: "/statistikk", icon: LineGraphIcon, label: "Statistikk", requiresAuth: true },
  { href: "/adopsjon", icon: PieChartIcon, label: "Adopsjon", requiresAuth: true },
  { href: "/ordliste", icon: InformationSquareIcon, label: "Ordliste" },
];
