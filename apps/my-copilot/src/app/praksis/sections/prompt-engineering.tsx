import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { Carousel } from "@/components/carousel";
import { LinkableHeading } from "@/components/linkable-heading";
import { XMarkOctagonIcon, TasklistIcon, FileTextIcon } from "@navikt/aksel-icons";

export default function PromptEngineering() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Prompt Engineering
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        Hvordan du formulerer forespørselen påvirker kvaliteten på Copilots svar. Spesifisitet er nøkkelen.
      </BodyShort>

      <div className="space-y-6">
        {/* Strategy 1: Specific prompts */}
        <div>
          <Heading size="small" level="3" className="mb-4 flex items-center gap-2">
            <span className="text-blue-600">1.</span>
            Vær spesifikk, ikke vag
          </Heading>

          <Carousel showIndicators={true} showSwipeHint={true}>
            <Box background="danger-soft" padding="space-16" borderRadius="8" className="border-l-4 border-red-600">
              <BodyShort weight="semibold" className="text-red-700 mb-2">
                ❌ Vag
              </BodyShort>
              <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                {`Fix the authentication bug.`}
              </code>
            </Box>

            <Box background="success-soft" padding="space-16" borderRadius="8" className="border-l-4 border-green-600">
              <BodyShort weight="semibold" className="text-green-700 mb-2">
                ✓ Spesifikk
              </BodyShort>
              <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                {`Users report 'Invalid token' errors
after 30 minutes. JWT tokens are
configured with 1-hour expiration
in auth.config.ts. Investigate why
tokens expire early and fix the
validation logic in middleware/auth.ts`}
              </code>
            </Box>
          </Carousel>
        </div>

        {/* Strategy 2: Examples */}
        <div>
          <Heading size="small" level="3" className="mb-4 flex items-center gap-2">
            <span className="text-blue-600">2.</span>
            Gi eksempler på forventet output
          </Heading>

          <Carousel showIndicators={true} showSwipeHint={true}>
            <Box background="danger-soft" padding="space-16" borderRadius="8" className="border-l-4 border-red-600">
              <BodyShort weight="semibold" className="text-red-700 mb-2">
                ❌ Uten eksempel
              </BodyShort>
              <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                {`Write a function that formats
currency in Norwegian style`}
              </code>
            </Box>

            <Box background="success-soft" padding="space-16" borderRadius="8" className="border-l-4 border-green-600">
              <BodyShort weight="semibold" className="text-green-700 mb-2">
                ✓ Med eksempel
              </BodyShort>
              <code className="text-sm bg-white p-2 block rounded whitespace-pre-wrap">
                {`Write a TypeScript function that
formats numbers as Norwegian currency.

Example:
formatNOK(1234.5) → "1 234,50 kr"
formatNOK(1000000) → "1 000 000,00 kr"`}
              </code>
            </Box>
          </Carousel>
        </div>

        {/* Strategy 3: Break down */}
        <div>
          <Heading size="small" level="3" className="mb-4 flex items-center gap-2">
            <span className="text-blue-600">3.</span>
            Bryt ned komplekse oppgaver
          </Heading>
          <BodyShort className="text-gray-600 mb-4">
            Store oppgaver bør deles i mindre steg. Bruk <strong>Plan Mode</strong> for å la Copilot analysere oppgaven
            og foreslå en plan før implementering.
          </BodyShort>

          {/* Plan Mode Image */}
          <div className="mb-4 rounded-lg overflow-hidden border border-gray-200 shadow-sm relative aspect-video">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src="/images/copilot-in-vs-code-hero-plan-mode.jpeg"
              alt="Plan Mode i VS Code - Copilot analyserer og planlegger oppgaven"
              className="w-full h-full object-cover"
            />
          </div>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
              <div className="flex items-center gap-2 mb-2">
                <TasklistIcon className="text-blue-600" aria-hidden />
                <BodyShort weight="semibold">Plan Mode</BodyShort>
              </div>
              <BodyShort className="text-gray-600 text-sm mb-2">
                Aktiver med &quot;/plan&quot; eller velg Plan i modusvelgeren. Copilot vil:
              </BodyShort>
              <ol className="space-y-1 list-decimal list-inside text-xs text-gray-600">
                <li>Analysere oppgaven og konteksten</li>
                <li>Foreslå en detaljert plan med steg</li>
                <li>La deg godkjenne eller justere planen</li>
                <li>Implementere steg for steg</li>
              </ol>
            </Box>

            <Box background="success-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
              <BodyShort weight="semibold" className="mb-2">
                Eksempel: Legg til autentisering
              </BodyShort>
              <ol className="space-y-1 list-decimal list-inside text-sm">
                <li>Lag en AuthContext med login/logout</li>
                <li>Lag en useAuth-hook</li>
                <li>Lag ProtectedRoute-komponent</li>
                <li>Integrer i app layout</li>
              </ol>
            </Box>
          </HGrid>

          <Box background="warning-soft" padding="space-12" borderRadius="8" className="mt-3">
            <BodyShort className="text-gray-600 text-xs">
              <strong>Tips:</strong> For coding agent på GitHub.com, skriv issues med klare akseptkriterier og bruk
              sub-issues for store oppgaver. Se{" "}
              <a
                href="https://docs.github.com/en/copilot/tutorials/coding-agent/get-the-best-results"
                className="text-blue-600 hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                Get the best results from the coding agent
              </a>
              .
            </BodyShort>
          </Box>

          {/* Spec Kit */}
          <Box background="success-soft" padding="space-12" borderRadius="8" className="mt-3">
            <div className="flex items-center gap-2 mb-2">
              <FileTextIcon className="text-green-700" aria-hidden />
              <BodyShort weight="semibold" className="text-green-700 text-sm">
                Spec Kit – Strukturert planlegging
              </BodyShort>
            </div>
            <BodyShort className="text-gray-600 text-xs mb-2">
              GitHubs offisielle verktøy for &quot;Spec-Driven Development&quot;. Skriv spesifikasjoner først, la
              Copilot implementere. Støtter slash-commands:
            </BodyShort>
            <div className="flex flex-wrap gap-1.5 mb-2">
              <code className="text-xs bg-white/70 px-1.5 py-0.5 rounded font-mono">/speckit.specify</code>
              <code className="text-xs bg-white/70 px-1.5 py-0.5 rounded font-mono">/speckit.plan</code>
              <code className="text-xs bg-white/70 px-1.5 py-0.5 rounded font-mono">/speckit.tasks</code>
              <code className="text-xs bg-white/70 px-1.5 py-0.5 rounded font-mono">/speckit.implement</code>
            </div>
            <BodyShort className="text-gray-500 text-xs">
              Installer: <code className="bg-white px-1 rounded">specify init my-project --ai copilot</code> –{" "}
              <a
                href="https://github.com/github/spec-kit"
                className="text-blue-600 hover:underline"
                target="_blank"
                rel="noopener noreferrer"
              >
                github/spec-kit
              </a>
            </BodyShort>
          </Box>
        </div>

        {/* Strategy 4: Context */}
        <div>
          <Heading size="small" level="3" className="mb-4 flex items-center gap-2">
            <span className="text-blue-600">4.</span>
            Gi relevant kontekst
          </Heading>
          <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
            <ul className="space-y-2">
              <li className="flex gap-2">
                <span className="text-blue-600">▪</span>
                <BodyShort className="text-sm">Åpne relevante filer, lukk irrelevante</BodyShort>
              </li>
              <li className="flex gap-2">
                <span className="text-blue-600">▪</span>
                <BodyShort className="text-sm">Bruk @workspace for prosjektkontekst i chat</BodyShort>
              </li>
              <li className="flex gap-2">
                <span className="text-blue-600">▪</span>
                <BodyShort className="text-sm">Merk opp koden du vil referere til</BodyShort>
              </li>
              <li className="flex gap-2">
                <span className="text-blue-600">▪</span>
                <BodyShort className="text-sm">Start ny chat når du bytter tema</BodyShort>
              </li>
            </ul>
          </Box>
        </div>

        {/* Anti-patterns */}
        <Box background="danger-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
          <div className="flex items-center gap-2 mb-5">
            <XMarkOctagonIcon className="text-red-700" aria-hidden />
            <Heading size="small" level="3" className="text-red-700">
              Anti-mønstre å unngå
            </Heading>
          </div>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <div>
              <BodyShort weight="semibold" className="text-sm">
                Vage direktiver
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                &quot;Be more accurate&quot; eller &quot;Identify all issues&quot; – Copilot gjør allerede sitt beste
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                Eksterne lenker
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Copilot følger ikke lenker – kopier relevant innhold inn i prompten
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                Tvetydige referanser
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                &quot;Fix this&quot; eller &quot;What does it do?&quot; – vær eksplisitt om hva du refererer til
              </BodyShort>
            </div>
            <div>
              <BodyShort weight="semibold" className="text-sm">
                UX-endringer
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Du kan ikke endre fonter eller formatering på Copilot-kommentarer
              </BodyShort>
            </div>
          </HGrid>
        </Box>
      </div>
    </Box>
  );
}
