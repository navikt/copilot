import { BodyLong, BodyShort, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Tilpasse",
  description:
    "Endre innstillingene, legg til teamets egne instruksjoner, overstyr og ignorer komponenter, og slå nav-pilots hooks av og på.",
};

const TOC: TocItem[] = [
  { id: "endre-innstillinger", label: "Endre innstillinger" },
  { id: "team-egne-instruksjoner", label: "Teamets egne instruksjoner" },
  { id: "prosjektkontekst-med-nav-pilot-init", label: "Prosjektkontekst med nav-pilot init" },
  { id: "overstyre-installerte-filer", label: "Overstyre installerte filer" },
  { id: "ignorere-enkeltkomponenter", label: "Ignorere enkeltkomponenter" },
  { id: "hooks", label: "Slå hooks av og på" },
];

const CONFIG_EXAMPLE = `# Skjemaversjon
version = 1

# Klient (copilot er standard)
client = "opencode"

# Modell. En Copilot-id som claude-opus-4.8 virker for copilot og opencode;
# opencode kjører den som github-copilot/claude-opus-4.8.
# Uten modell velger klienten selv.
# model = "claude-opus-4.8"

# Modus (default | plan | autopilot), bare Copilot
# mode = "default"

# Resonneringsinnsats (none|low|medium|high|xhigh|max)
reasoning_effort = "high"

# Loggnivå
# log_level = "info"`;

export default function Tilpasse() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Tilpasse"
      description="Repoet ditt trenger ofte egne regler og egen kontekst. Slik legger du dem til uten å miste oppdateringene fra nav-pilot."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="endre-innstillinger" size="medium" level="2">
            Endre innstillinger
          </LinkableHeading>
          <BodyLong>
            Innstillingene dine ligger i <code className={code}>~/.nav-pilot/config.toml</code>. Det finnes ingen konfig
            per repo. Et flagg vinner over fila, og fila vinner over standardverdien.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot config                 # innstillingssiden i terminalen
nav-pilot config setup           # veiviser: klient, modus, modell og hva agenten får gjøre selv
nav-pilot config set <nøkkel> <verdi>
nav-pilot config unset <nøkkel>  # tilbake til standardverdien
nav-pilot config show            # hver nøkkel, verdien og hvor den kommer fra
nav-pilot config init            # lag fila med alle valg kommentert ut`}
          </CodeBlock>
          <BodyLong>Slik kan fila se ut:</BodyLong>
          <CodeBlock compact filename="~/.nav-pilot/config.toml">
            {CONFIG_EXAMPLE}
          </CodeBlock>
          <BodyShort size="small" textColor="subtle">
            Alle nøklene står i{" "}
            <NextLink href="/nav-pilot/referanse#konfignokler" className={linkClass}>
              referansen
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="team-egne-instruksjoner" size="medium" level="2">
            Teamets egne instruksjoner
          </LinkableHeading>
          <BodyLong>
            Legg teamets filer i <code className={code}>.github/instructions/</code>, ved siden av det nav-pilot
            installerer. nav-pilot rører aldri filer den ikke har installert selv.
          </BodyLong>
          <CodeBlock compact>
            {`.github/instructions/
  golang.instructions.md           ← fra nav-pilot
  security-owasp.instructions.md   ← fra nav-pilot
  team-conventions.instructions.md ← teamets egen fil`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="prosjektkontekst-med-nav-pilot-init" size="medium" level="2">
            Prosjektkontekst med nav-pilot init
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot init</code> lager tre filer med TODO-er, forhåndsutfylt etter stacken den
            finner. Teamet fyller inn resten. nav-pilot lager filene én gang og rører dem ikke etterpå. Filer som finnes
            fra før, blir stående.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot init
# AGENTS.md
# .github/copilot-instructions.md
# .github/copilot-review-instructions.md`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="overstyre-installerte-filer" size="medium" level="2">
            Overstyre installerte filer
          </LinkableHeading>
          <BodyLong>
            Vil teamet eie en fil nav-pilot vanligvis oppdaterer, legg den i{" "}
            <code className={code}>.github/copilot-sync.json</code>. <code className={code}>nav-pilot sync</code> hopper
            over filene i <code className={code}>overrides</code>, så teamets versjon blir stående.
          </BodyLong>
          <CodeBlock compact filename=".github/copilot-sync.json">
            {`{
  "overrides": [
    ".github/instructions/golang.instructions.md"
  ]
}`}
          </CodeBlock>
          <BodyShort size="small" textColor="subtle">
            Mer om hva sync gjør med filene, står i{" "}
            <NextLink href="/nav-pilot/guider/synkronisere#tilpasse-sync" className={linkClass}>
              Tilpasse sync
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="ignorere-enkeltkomponenter" size="medium" level="2">
            Ignorere enkeltkomponenter
          </LinkableHeading>
          <BodyLong>
            Når agentpakka får en ny komponent, sier nav-pilot fra ved oppstart. Vil du ikke ha den, stopper du
            varselet. <code className={code}>ignore</code> virker bare for installasjonen i{" "}
            <code className={code}>~/.copilot</code> (<code className={code}>--user</code>).
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot ignore agent rust-agent --user
nav-pilot ignore skill rust-development --user
nav-pilot ignore instruction nextjs-aksel --user`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hooks" size="medium" level="2">
            Slå hooks av og på
          </LinkableHeading>
          <BodyLong>
            nav-pilot legger egne hooks i <code className={code}>~/.copilot/hooks/</code>, og de kjører i alle økter med
            Copilot CLI. Fem nøkler slår dem av og på, og alle er på som standard:
          </BodyLong>
          <Bullets>
            <li>
              <code className={code}>hook_loop_guard</code>: sier fra til modellen når den gjør det samme kallet om og
              om igjen.
            </li>
            <li>
              <code className={code}>hook_redact_secrets</code>: maskerer tokener, nøkler og passord i verktøyresultater
              før modellen leser dem.
            </li>
            <li>
              <code className={code}>hook_redact_fnr</code>: maskerer fødselsnummer, D-nummer og H-nummer.
            </li>
            <li>
              <code className={code}>hook_injection_note</code>: merker verktøyresultater som ser ut som instrukser til
              modellen, så den leser dem som data.
            </li>
            <li>
              <code className={code}>hook_action_check</code>: spør den lokale modellen om en risikabel skallkommando,
              for eksempel <code className={code}>kubectl delete</code> eller <code className={code}>rm -rf</code>, gir
              mening før den kjører, og lagrer svaret. Den stopper ingenting, og KI-agenten ser ikke svaret. Virker bare
              med lokal modell. Slå den av med <code className={code}>off</code>.
              <span id="hook-action-check" className="block" style={{ marginTop: "var(--ax-space-8)" }}>
                Kommandoene den sjekker: endringer med <code className={code}>kubectl</code>,{" "}
                <code className={code}>nais</code>, <code className={code}>gcloud</code> og{" "}
                <code className={code}>helm</code>, <code className={code}>terraform apply</code>,{" "}
                <code className={code}>rm -r</code>, <code className={code}>git push --force</code> og lignende.
                Modellen vurderer om kommandoen står i forhold til formålet, om den er destruktiv, og om formålet
                agenten oppga støtter den. Med <code className={code}>log</code> lagres svaret i telemetrien og en lokal
                logg, og kommandoen kjører alltid. Den krever <code className={code}>local_enabled</code> og en server
                som kjører, og starter aldri en server selv.
              </span>
            </li>
          </Bullets>
          <CodeBlock compact>
            {`nav-pilot config set hook_redact_fnr false   # fjernes ved neste oppstart
nav-pilot config set hook_redact_fnr true    # på igjen`}
          </CodeBlock>
          <BodyLong>
            Kan ikke nav-pilot lese <code className={code}>config.toml</code>, kjører maskeringen og løkkevakta likevel,
            og du får beskjed på stderr. Løkkevakta nevner ikke terskelen overfor modellen, så modellen ikke kan heve
            den selv.
          </BodyLong>
          <LinkableHeading id="hooks-fra-agentpakka" size="small" level="3">
            Hooks fra agentpakka er kode
          </LinkableHeading>
          <BodyLong>
            Agentpakka har også hooks. En hook er et Python-skript Copilot CLI kjører ved verktøykall, ikke tekst
            modellen leser. <code className={code}>nav-pilot install</code> legger dem inn sammen med resten, så les dem
            før du stoler på dem. De ligger i <code className={code}>.github/hooks/</code> eller{" "}
            <code className={code}>~/.copilot/hooks/</code>. <code className={code}>nav-pilot uninstall</code> fjerner
            bare det nav-pilot har skrevet, så dine egne hooks blir stående.
          </BodyLong>
          <BodyLong>
            To av hookene i agentpakka er porter som kan stoppe et verktøykall: polling-porten og ARIA-porten. Portene
            slipper kallet gjennom når Python svikter: <code className={code}>python3</code> mangler, skriptet feiler,
            eller det svarer ikke innen fristen på ett sekund. Hver port har et unntak:{" "}
            <code className={code}>POLL_OK=1</code> foran kommandoen for polling-porten, og en kommentar med{" "}
            <code className={code}>ARIA_OK</code> og begrunnelsen ved siden av <code className={code}>role</code>
            -attributtet for ARIA-porten. ARIA-porten ber bare modellen spørre deg og nevner ikke merket. Merket er et
            spor du legger igjen etter at du har sagt ja. Det er ingen lås, for en modell kan skrive det selv.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
