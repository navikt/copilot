import Faro from "@/components/faro";
import { FooterMessage } from "@/components/footer-message";
import { HashAnchorScroll } from "@/components/hash-anchor-scroll";
import { SiteHeader, type HeaderLabels } from "@/components/navigation/site-nav";
import { getUser } from "@/lib/auth";
import { BodyShort, Box, HStack, Link, Theme } from "@navikt/ds-react";
import { Inter } from "next/font/google";
import { Suspense } from "react";

export interface ShellLabels extends Omit<HeaderLabels, "lang"> {
  glossary: string;
  otherLang: string;
  otherLangHref: string;
  privacy: string;
  privacyHref: string;
  // Set when the target page is in another language than the shell, so the
  // link carries hreflang and WCAG 3.1.2 is satisfied for the text inside it.
  privacyHrefLang?: string;
  accessibility: string;
  accessibilityHref: string;
  accessibilityHrefLang?: string;
}

const inter = Inter({ subsets: ["latin"] });

export async function SiteShell({
  lang,
  labels,
  children,
}: Readonly<{
  lang: "nb" | "en";
  labels: ShellLabels;
  children: React.ReactNode;
}>) {
  const user = await getUser(false);

  return (
    <html lang={lang}>
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
              <SiteHeader
                labels={{ ...labels, lang }}
                userName={user ? `${user.firstName} ${user.lastName}` : undefined}
              />
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
              <FooterMessage lang={lang} />
              <HStack gap="space-16" wrap asChild>
                <BodyShort size="small" as="div">
                  <Link href="/ordbok" hrefLang={lang === "en" ? "nb" : undefined} data-color="neutral">
                    {labels.glossary}
                  </Link>
                  <Link
                    href={labels.otherLangHref}
                    hrefLang={lang === "nb" ? "en" : "nb"}
                    lang={lang === "nb" ? "en" : "nb"}
                    data-color="neutral"
                  >
                    {labels.otherLang}
                  </Link>
                  <Link href={labels.privacyHref} hrefLang={labels.privacyHrefLang} data-color="neutral">
                    {labels.privacy}
                  </Link>
                  <Link href={labels.accessibilityHref} hrefLang={labels.accessibilityHrefLang} data-color="neutral">
                    {labels.accessibility}
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
