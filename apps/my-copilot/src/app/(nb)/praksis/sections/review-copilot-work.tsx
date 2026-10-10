import NextLink from "next/link";
import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { ExclamationmarkTriangleIcon, LightBulbIcon } from "@navikt/aksel-icons";

export default function ReviewCopilotWork() {
  return (
    <div className="space-y-8">
      {/* Code Review Image */}
      <div className="mb-6 rounded-lg overflow-hidden border border-gray-200 shadow-sm relative aspect-video">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src="/images/github-copilot-code-review-updated.jpeg"
          alt="Copilot Code Review på GitHub"
          className="w-full h-full object-cover"
        />
      </div>

      <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
        <Box background="info-soft" padding="space-16" borderRadius="8" className="border-l-4 border-blue-600">
          <div className="flex items-center gap-2 mb-5">
            <span className="text-blue-600 font-bold text-lg">1</span>
            <Heading size="small" level="3">
              Session logs
            </Heading>
          </div>
          <ul className="space-y-2 text-sm">
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <span>Forstod agenten oppgaven?</span>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <span>Var det feil den ga opp på?</span>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <span>Gikk den i loop eller hallusinerte?</span>
            </li>
          </ul>
        </Box>

        <Box background="success-soft" padding="space-16" borderRadius="8" className="border-l-4 border-green-600">
          <div className="flex items-center gap-2 mb-5">
            <span className="text-green-600 font-bold text-lg">2</span>
            <Heading size="small" level="3">
              Files changed
            </Heading>
          </div>
          <ul className="space-y-2 text-sm">
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <span>Kun relevante filer endret?</span>
            </li>
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <span>Følger koden prosjektets stil?</span>
            </li>
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <span>Er det hardkodet/generert kode?</span>
            </li>
          </ul>
        </Box>

        <Box background="warning-soft" padding="space-16" borderRadius="8" className="border-l-4 border-orange-600">
          <div className="flex items-center gap-2 mb-5">
            <span className="text-orange-600 font-bold text-lg">3</span>
            <Heading size="small" level="3">
              Checks
            </Heading>
          </div>
          <ul className="space-y-2 text-sm">
            <li className="flex gap-2">
              <span className="text-orange-600">▪</span>
              <span>Godkjenn workflow-kjøringen når du har lest endringen</span>
            </li>
            <li className="flex gap-2">
              <span className="text-orange-600">▪</span>
              <span>Sjekk at alle tester passerer</span>
            </li>
            <li className="flex gap-2">
              <span className="text-orange-600">▪</span>
              <span>Verifiser i preview/staging</span>
            </li>
          </ul>
        </Box>
      </HGrid>

      <Box background="danger-soft" padding="space-16" borderRadius="8" className="mt-4">
        <div className="flex items-center gap-2 mb-2">
          <ExclamationmarkTriangleIcon className="text-red-700" aria-hidden />
          <Heading size="small" level="3" className="text-red-700">
            Viktig: CI venter på godkjenning
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm">
          Som standard kjører ikke workflows på PR-er fra Copilot cloud agent før noen med skrivetilgang godkjenner
          kjøringen. GitHub har en innstilling som lar workflows kjøre uten godkjenning. Nav anbefaler å beholde
          standarden, slik at et menneske ser på endringen før koden kjører med repoets tilganger.
        </BodyShort>
      </Box>

      {/* Pro tips */}
      <Box background="accent-soft" padding="space-16" borderRadius="8" className="mt-4">
        <div className="flex items-center gap-2 mb-2">
          <LightBulbIcon className="text-blue-600" aria-hidden />
          <Heading size="small" level="3">
            Pro-tips for effektiv gjennomgang
          </Heading>
        </div>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Be Copilot gjennomgå seg selv
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              &quot;Review this PR for bugs, security issues, and code style violations&quot;
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Bruk @code-review og Copilot code review
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Org-agenten{" "}
              <NextLink href="/verktoy?item=code-review" className="text-blue-600 hover:underline">
                @code-review
              </NextLink>{" "}
              går gjennom endringer lokalt. På GitHub kan du legge til Copilot som reviewer på PR-en. Ingen av dem
              erstatter en kollega.
            </BodyShort>
          </div>
          <div>
            <BodyShort weight="semibold" className="text-sm">
              Grupper lignende PR-er
            </BodyShort>
            <BodyShort className="text-gray-600 text-xs">
              Gjennomgå flere dokumentasjons-PR-er sammen for konsistens
            </BodyShort>
          </div>
        </HGrid>
      </Box>
    </div>
  );
}
