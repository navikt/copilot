import {
  BookIcon,
  WrenchIcon,
  LineGraphIcon,
  BankNoteIcon,
  PersonIcon,
  PieChartIcon,
  InformationSquareIcon,
} from "@navikt/aksel-icons";

export const NAV_ITEMS = [
  { href: "/praksis", icon: BookIcon, label: "God praksis" },
  { href: "/verktoy", icon: WrenchIcon, label: "Verktøy" },
  { href: "/statistikk", icon: LineGraphIcon, label: "Statistikk" },
  { href: "/adopsjon", icon: PieChartIcon, label: "Adopsjon" },
  { href: "/kostnad", icon: BankNoteIcon, label: "Kostnad" },
  { href: "/abonnement", icon: PersonIcon, label: "Abonnement" },
  { href: "/ordliste", icon: InformationSquareIcon, label: "Ordliste" },
];
