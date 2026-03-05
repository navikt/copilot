import SubscriptionDetails from "@/components/subscription";
import { getUser } from "@/lib/auth";
import React from "react";
import { Box, VStack, Heading, HGrid, BodyShort } from "@navikt/ds-react";
import { BookIcon, WrenchIcon, LineGraphIcon, BankNoteIcon, ExternalLinkIcon } from "@navikt/aksel-icons";
import NextLink from "next/link";

export default async function Home() {
  const user = await getUser(false);

  return (
    <main className="max-w-7xl mx-auto">
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
      >
        <VStack gap={{ xs: "space-24", md: "space-32" }}>
          <VStack gap="space-8">
            <Heading size="xlarge" level="1">
              GitHub Copilot
            </Heading>
            <BodyShort className="max-w-2xl">
              AI-drevet utviklingsverktøy for Nav. Administrer abonnementet ditt, utforsk beste praksis, og installer
              skreddersydde verktøy for Navs tekniske stack.
            </BodyShort>
          </VStack>

          <Box>
            <Heading size="medium" level="2" className="mb-4">
              Kom i gang
            </Heading>
            <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-12">
              <NavCard
                href="/practice"
                icon={<BookIcon aria-hidden />}
                title="God praksis"
                description="Lær å bruke Copilot effektivt og trygt"
              />
              <NavCard
                href="/customizations"
                icon={<WrenchIcon aria-hidden />}
                title="Verktøy"
                description="Agenter, instruksjoner og prompts for Nav"
              />
              <NavCard
                href="/stats"
                icon={<LineGraphIcon aria-hidden />}
                title="Statistikk"
                description="Se bruksdata og trender for organisasjonen"
              />
              <NavCard
                href="/cost"
                icon={<BankNoteIcon aria-hidden />}
                title="Kostnad"
                description="Lisenser, kostnader og innstillinger"
              />
            </HGrid>
          </Box>

          <Box>
            <Heading size="medium" level="2" className="mb-4">
              Ressurser
            </Heading>
            <HGrid columns={{ xs: 1, sm: 2 }} gap="space-12">
              <NavCard
                href="https://docs.github.com/en/copilot"
                icon={<ExternalLinkIcon aria-hidden />}
                title="GitHub Copilot Dokumentasjon"
                description="Offisiell dokumentasjon fra GitHub"
                external
              />
              <NavCard
                href="https://utvikling.intern.nav.no/teknisk/github-copilot.html"
                icon={<ExternalLinkIcon aria-hidden />}
                title="Om GitHub Copilot i Nav"
                description="Navs retningslinjer og oppsett"
                external
              />
            </HGrid>
          </Box>

          <Box>
            <Heading size="medium" level="2" className="mb-4">
              Mitt Abonnement
            </Heading>
            <SubscriptionDetails user={user!} />
          </Box>
        </VStack>
      </Box>
    </main>
  );
}

function NavCard({
  href,
  icon,
  title,
  description,
  external,
}: {
  href: string;
  icon: React.ReactNode;
  title: string;
  description: string;
  external?: boolean;
}) {
  const linkProps = external ? { target: "_blank", rel: "noopener noreferrer" } : {};
  return (
    <Box borderColor="neutral" borderWidth="1" borderRadius="8" padding="space-16" asChild>
      <NextLink href={href} {...linkProps} className="no-underline hover:shadow-md transition-shadow">
        <VStack gap="space-8">
          <Heading size="xsmall" level="3">
            <span className="flex items-center gap-2">
              {icon}
              {title}
            </span>
          </Heading>
          <span className="text-text-subtle text-sm">{description}</span>
        </VStack>
      </NextLink>
    </Box>
  );
}
