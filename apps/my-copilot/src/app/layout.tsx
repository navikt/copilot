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
  title: {
    template: "%s — Oh-My-Nav",
    default: "Oh-My-Nav",
  },
  description: "Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.",
  metadataBase: new URL("https://ki-utvikling.nav.no"),
  openGraph: {
    type: "website",
    locale: "nb_NO",
    siteName: "Oh-My-Nav",
    title: "Oh-My-Nav",
    description: "Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.",
  },
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const user = await getUser(false);

  return (
    <html lang="nb">
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
                Oh-My-Nav
              </NextLink>
              {user ? (
                <HStack gap="space-16" align="center">
                  <NextLink
                    href="/abonnement"
                    className="text-white/70 text-sm no-underline hover:text-white transition-colors"
                  >
                    Abonnement
                  </NextLink>
                  <BodyShort size="small" style={{ color: "rgba(255, 255, 255, 0.7)" }}>
                    {user.firstName} {user.lastName}
                  </BodyShort>
                </HStack>
              ) : (
                <Link
                  href="/oauth2/login"
                  className="text-white/90 text-sm no-underline hover:text-white transition-colors"
                >
                  Logg inn
                </Link>
              )}
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
                <Link href="/personvern" className="text-white/90 hover:text-white text-sm">
                  Personvern
                </Link>
                <Link href="/tilgjengelighet" className="text-white/90 hover:text-white text-sm">
                  Tilgjengelighet
                </Link>
                <Link href="https://github.com/navikt/copilot" className="text-white/90 hover:text-white text-sm">
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
