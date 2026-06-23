import Faro from "@/components/faro";
import { FooterMessage } from "@/components/footer-message";
import { HashAnchorScroll } from "@/components/hash-anchor-scroll";
import NavBudgetBar from "@/components/nav-budget-bar";
import { getUser } from "@/lib/auth";
import { BodyShort, Box, HStack, Link, Theme } from "@navikt/ds-react";
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import NextLink from "next/link";
import { Suspense } from "react";
import "./globals.css";

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
      <body className={`${inter.className} bg-gray-800 min-h-dvh flex flex-col`}>
        <Suspense fallback={null}>
          <HashAnchorScroll />
        </Suspense>
        <Theme theme="dark" hasBackground={false} asChild>
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
                    <NavBudgetBar />
                    <BodyShort size="small" style={{ color: "rgba(255, 255, 255, 0.7)" }}>
                      {user.firstName} {user.lastName}
                    </BodyShort>
                  </HStack>
                ) : (
                  <Link href="/oauth2/login" data-color="neutral" className="text-sm" underline={false}>
                    Logg inn
                  </Link>
                )}
              </HStack>
            </Box>
          </header>
        </Theme>
        <div className="bg-gray-100 flex-1 min-h-0">{children}</div>
        <Theme theme="dark" hasBackground>
          <HStack
            asChild
            justify="space-between"
            align="center"
            wrap
            gap="space-8"
            paddingBlock="space-12"
            paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
            className="max-w-7xl mx-auto"
          >
            <footer>
              <FooterMessage />
              <HStack gap="space-16" asChild>
                <BodyShort size="small" as="div">
                  <Link href="/personvern" data-color="neutral">
                    Personvern
                  </Link>
                  <Link href="/tilgjengelighet" data-color="neutral">
                    Tilgjengelighet
                  </Link>
                  <Link href="https://github.com/navikt/copilot" data-color="neutral">
                    GitHub
                  </Link>
                </BodyShort>
              </HStack>
            </footer>
          </HStack>
        </Theme>
        <Faro collectorUrl={process.env.NAIS_FRONTEND_TELEMETRY_COLLECTOR_URL} />
      </body>
    </html>
  );
}
