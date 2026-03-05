import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { InternalHeader, InternalHeaderTitle, InternalHeaderUser } from "@navikt/ds-react/InternalHeader";
import { Spacer, Box, HStack, BodyShort, Link } from "@navikt/ds-react";
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
          <Spacer />
          <div className="md:hidden flex items-center">
            <MobileNav />
          </div>
          <InternalHeaderUser name={`${user.firstName} ${user.lastName}`} className="hidden md:flex" />
        </InternalHeader>
        <div className="bg-gray-100">{children}</div>
        <footer className="text-white">
          <Box
            paddingBlock="space-12"
            paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
            className="max-w-7xl mx-auto"
          >
            <HStack justify="space-between" align="center" wrap gap="space-8">
              <BodyShort size="small" className="text-gray-400">
                Bygget med GitHub Copilot
              </BodyShort>
              <HStack gap="space-16">
                <Link href="https://docs.github.com/en/copilot" className="text-gray-400 hover:text-white text-sm">
                  Dokumentasjon
                </Link>
                <Link href="https://github.com/navikt/copilot" className="text-gray-400 hover:text-white text-sm">
                  GitHub
                </Link>
              </HStack>
            </HStack>
          </Box>
        </footer>
      </body>
      <Faro />
    </html>
  );
}
