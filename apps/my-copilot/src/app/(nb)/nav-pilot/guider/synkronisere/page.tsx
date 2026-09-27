import { BodyLong, BodyShort, Heading, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { GUIDE_PAGES } from "@/components/nav-pilot/doc-pages";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Synkronisere — nav-pilot",
  description:
    "Hold agentpakka i repoet oppdatert med en ukentlig pull request eller med nav-pilot sync, og bestem selv hvilke filer som oppdateres.",
};

const TOC: TocItem[] = [
  { id: "automatisk-sync", label: "Automatisk sync" },
  { id: "lokal-sync", label: "Lokal sync" },
  { id: "hvordan-nav-pilot-finner-filer", label: "Hvordan nav-pilot finner filer" },
  { id: "tilpasse-sync", label: "Tilpasse sync" },
  { id: "faq", label: "Spørsmål og svar" },
];

const WORKFLOW = `name: Copilot Customization Sync
on:
  schedule:
    - cron: '0 7 * * 1'  # mandager kl. 07.00
  workflow_dispatch:
jobs:
  sync:
    uses: navikt/copilot/.github/workflows/copilot-customization-sync.yml@main
    permissions:
      contents: write
      pull-requests: write`;

const FAQ = [
  {
    q: "Trenger jeg et GitHub-token eller en secret?",
    a: "Nei. Workflowen bruker GITHUB_TOKEN og leser offentlige kildefiler.",
  },
  {
    q: "Hva skjer med en fil jeg har endret?",
    a: "Oppdateringen tar kildens versjon, lagrer kopien din som <fil>.orig ved siden av og sier fra. I CI står fila under «Changed» i pull requesten, merket med at den har lokale endringer. Du kan gå gjennom, flette det du vil ha eller lukke pull requesten. Workflowen tvinger aldri gjennom noe.",
  },
  {
    q: "Hva skjer når kilden slutter å levere en fil?",
    a: "nav-pilot sync --apply fjerner kopien din i repoet, i ~/.copilot og i mappa til opencode. Uten --apply lister sync bare filene. Har du endret fila, lagres kopien din som <fil>.orig før den fjernes, og i CI er den merket i pull requesten. En hook du har endret, blir stående og kjører videre, og sync sier fra. Vil du beholde fila, legg den i overrides før du kjører --apply.",
  },
  {
    q: "Hva skjer hvis jeg sletter en fil selv?",
    a: "Fila blir merket som ignorert og kommer ikke tilbake ved neste sync. Vil du ha den tilbake, kjør nav-pilot install <navn>.",
  },
  {
    q: "Hva er forskjellen fra Dependabot?",
    a: "Samme idé, en pull request med oppdateringer, men for Copilot-filer. nav-pilot sammenligner SHA-256 i stedet for versjonsnumre.",
  },
];

export default function Synkronisere() {
  return (
    <DocPage
      label="Guider"
      title="Synkronisere"
      description="Agentpakka i navikt/copilot endres jevnlig. Hold repoet ditt oppdatert med en ukentlig pull request eller med nav-pilot sync."
      toc={TOC}
      siblings={{ pages: GUIDE_PAGES, current: "/nav-pilot/guider/synkronisere" }}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="automatisk-sync" size="medium" level="2">
            Automatisk sync
          </LinkableHeading>
          <BodyLong>
            Legg denne workflowen i repoet. Den åpner en pull request når noe er endret, som Dependabot. Pull requesten
            viser hvilke filer som er oppdatert, med lenker til kilderepoet.
          </BodyLong>
          <CodeBlock compact filename=".github/workflows/copilot-sync.yml">
            {WORKFLOW}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lokal-sync" size="medium" level="2">
            Lokal sync
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot sync</code> sammenligner SHA-256 for filene dine med kilderepoet. Uten
            flagg sjekker den alle stedene der du har installert.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot sync            # sjekk om det finnes oppdateringer (kode 1 hvis ja)
nav-pilot sync --apply    # oppdater filene; spør før den sletter noe
nav-pilot sync --json     # resultatet som JSON`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hvordan-nav-pilot-finner-filer" size="medium" level="2">
            Hvordan nav-pilot finner filer
          </LinkableHeading>
          <BodyLong>
            Har du installert med <code className={code}>nav-pilot install</code>, står hver fil i tilstandsfila. Har
            teamet kopiert filene for hånd, ser nav-pilot etter filer som også finnes i kilderepoet:
          </BodyLong>
          <Bullets>
            <li>
              <code className={code}>.github/agents/*.agent.md</code>
            </li>
            <li>
              <code className={code}>.github/instructions/*.instructions.md</code>
            </li>
            <li>
              <code className={code}>.github/prompts/*.prompt.md</code>
            </li>
            <li>
              <code className={code}>.github/skills/*/</code> (hele mapper)
            </li>
          </Bullets>
          <BodyShort size="small" textColor="subtle">
            AGENTS.md og <code className={code}>.github/copilot-instructions.md</code> oppdateres aldri. De hører til
            repoet.
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="tilpasse-sync" size="medium" level="2">
            Tilpasse sync
          </LinkableHeading>
          <BodyLong>
            Vil du fjerne filer for et rammeverk du ikke bruker, for eksempel instruksjonene for Next.js i et
            Astro-prosjekt? Legg dem i <code className={code}>overrides</code> i{" "}
            <code className={code}>.github/copilot-sync.json</code>:
          </BodyLong>
          <CodeBlock compact filename=".github/copilot-sync.json">
            {`{
  "overrides": [
    ".github/instructions/nextjs-aksel.instructions.md",
    ".github/instructions/performance.instructions.md",
    ".github/prompts/nextjs-api-route.prompt.md"
  ]
}`}
          </CodeBlock>
          <BodyLong>
            Sync hopper helt over filene i <code className={code}>overrides</code>. Du kan slette dem, og de kommer ikke
            tilbake. Du kan også velge dem bort i velgeren når du installerer.
          </BodyLong>
          <BodyLong>
            Har teamet en egen fil med samme navn som en i kilden, for eksempel en egen skill{" "}
            <code className={code}>kotlin-app-config</code>, avhenger det av når den kom:
          </BodyLong>
          <Bullets>
            <li>
              Fantes fila før <code className={code}>nav-pilot install</code>, hopper install over den og sier fra, og
              sync rører den ikke.
            </li>
            <li>
              I et repo med kopierte filer og uten install vil sync foreslå å overskrive den. Legg den i{" "}
              <code className={code}>overrides</code>.
            </li>
          </Bullets>
          <BodyShort size="small" textColor="subtle">
            Filer med navn som ikke finnes i kilden, rører sync aldri. Varsler om nye komponenter stopper du med{" "}
            <NextLink href="/nav-pilot/guider/tilpasse#ignorere-enkeltkomponenter" className={linkClass}>
              nav-pilot ignore
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="faq" size="medium" level="2">
            Spørsmål og svar
          </LinkableHeading>
          {FAQ.map((f) => (
            <VStack key={f.q} gap="space-4">
              <Heading size="xsmall" level="3">
                {f.q}
              </Heading>
              <BodyLong>{f.a}</BodyLong>
            </VStack>
          ))}
        </VStack>
      </section>
    </DocPage>
  );
}
