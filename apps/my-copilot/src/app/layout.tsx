import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import {
  InternalHeader,
  InternalHeaderButton,
  InternalHeaderTitle,
  InternalHeaderUser,
} from "@navikt/ds-react/InternalHeader";
import { Spacer, Box, HGrid } from "@navikt/ds-react";
import { getUser } from "@/lib/auth";
import Faro from "@/components/faro";
import { MobileNav } from "@/components/mobile-nav";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Min Copilot",
  description: "Min Copilot er et selvbetjeningsverktøy for administrasjon av ditt GitHub Copilot abonnement.",
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const user = await getUser(false);

  if (!user) {
    return null;
  }

  return (
    <html lang="en">
      <body className={`${inter.className} bg-gray-800`}>
        <InternalHeader>
          <InternalHeaderTitle as="a" href="/">
            Min Copilot
          </InternalHeaderTitle>
          <InternalHeaderButton as="a" href="/usage" className="hidden md:flex">
            Bruksstatistikk
          </InternalHeaderButton>
          <InternalHeaderButton as="a" href="/overview" className="hidden md:flex">
            Lisensoversikt
          </InternalHeaderButton>
          <InternalHeaderButton as="a" href="/best-practices" className="hidden md:flex">
            Beste Praksis
          </InternalHeaderButton>
          <Spacer />
          <div className="md:hidden flex items-center">
            <MobileNav />
          </div>
          <InternalHeaderUser name={`${user.firstName} ${user.lastName}`} className="hidden md:flex" />
        </InternalHeader>
        <div className="bg-gray-100">{children}</div>
        <Box as="footer" paddingBlock="space-20" paddingInline="space-8" className="text-white text-left text-md">
          <HGrid columns={{ xs: 1, md: 3 }} gap="space-8">
            <div>
              <p>Bygget med GitHub Copilot</p>
            </div>
            <div>
              <h2 className="text-lg font-bold mb-2">Relevante Lenker</h2>
              <ul className="list-disc list-inside">
                <li>
                  <a href="/best-practices" className="text-blue-400 hover:underline">
                    Beste Praksis og Læring
                  </a>
                </li>
                <li>
                  <a href="https://docs.github.com/en/copilot" className="text-blue-400 hover:underline">
                    GitHub Copilot Dokumentasjon
                  </a>
                </li>
                <li>
                  <a href="https://github.com/features/copilot" className="text-blue-400 hover:underline">
                    GitHub Copilot Funksjoner
                  </a>
                </li>
                <li>
                  <a
                    href="https://utvikling.intern.nav.no/teknisk/github-copilot.html"
                    className="text-blue-400 hover:underline"
                  >
                    Om GitHub Copilot i Nav
                  </a>
                </li>
              </ul>
            </div>
            <div>
              <h2 className="text-lg font-bold mb-2">Tekniske Lenker</h2>
              <ul className="list-disc list-inside">
                <li>
                  <a href="https://github.com/nais/my-copilot" className="text-blue-400 hover:underline">
                    github.com/nais/my-copilot
                  </a>
                </li>
                <li>
                  <a href="https://grafana.nav.cloud.nais.io/d/min-copilot" className="text-blue-400 hover:underline">
                    Grafana Dashboard
                  </a>
                </li>
              </ul>
            </div>
          </HGrid>
        </Box>
      </body>
      <Faro />
    </html>
  );
}
