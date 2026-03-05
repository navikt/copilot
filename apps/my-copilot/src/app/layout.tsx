import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { Box, HStack, BodyShort, Link } from "@navikt/ds-react";
import { getUser } from "@/lib/auth";
import Faro from "@/components/faro";
import NextLink from "next/link";
import { FooterMessage } from "@/components/footer-message";

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
        <header style={{ background: "#0f1825" }}>
          <Box
            paddingBlock="space-8"
            paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
            className="max-w-7xl mx-auto"
          >
            <HStack justify="space-between" align="center">
              <NextLink
                href="/"
                className="text-white/90 text-sm font-medium no-underline hover:text-white transition-colors"
              >
                Min Copilot
              </NextLink>
              <BodyShort size="small" className="text-white/50">
                {user.firstName} {user.lastName}
              </BodyShort>
            </HStack>
          </Box>
        </header>
        <div className="bg-gray-100">{children}</div>
        <footer className="text-white">
          <Box
            paddingBlock="space-12"
            paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
            className="max-w-7xl mx-auto"
          >
            <HStack justify="space-between" align="center" wrap gap="space-8">
              <FooterMessage />
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
