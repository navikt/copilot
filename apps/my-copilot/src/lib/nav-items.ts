import {
  BookIcon,
  WrenchIcon,
  LineGraphIcon,
  BankNoteIcon,
  PersonIcon,
  PieChartIcon,
  RocketIcon,
} from "@navikt/aksel-icons";

export type NavItem = {
  href: string;
  icon: typeof BookIcon;
  label: string;
  requiresAuth?: boolean;
};

export const NAV_ITEMS: NavItem[] = [
  { href: "/nav-pilot", icon: RocketIcon, label: "nav-pilot" },
  { href: "/praksis", icon: BookIcon, label: "God praksis" },
  { href: "/verktoy", icon: WrenchIcon, label: "Verktøy" },
  { href: "/statistikk", icon: LineGraphIcon, label: "Statistikk", requiresAuth: true },
  { href: "/adopsjon", icon: PieChartIcon, label: "Adopsjon", requiresAuth: true },
  { href: "/kostnad", icon: BankNoteIcon, label: "Kostnad", requiresAuth: true },
  { href: "/abonnement", icon: PersonIcon, label: "Abonnement", requiresAuth: true },
];
