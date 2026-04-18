import { Heading, BodyShort, Box, HGrid, Label, VStack } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import {
  CheckmarkCircleIcon,
  XMarkOctagonIcon,
  FileTextIcon,
  CogIcon,
  PencilWritingIcon,
  LightBulbIcon,
} from "@navikt/aksel-icons";

export default function EffectiveCustomizations() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Skriv Effektive Tilpasninger
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        Nå som du vet hvilke tilpasningstyper som finnes, her er konkrete råd for å skrive dem godt. Kilde:{" "}
        <a
          href="https://code.visualstudio.com/docs/copilot/customization/overview"
          className="text-blue-600 hover:underline"
          target="_blank"
          rel="noopener noreferrer"
        >
          VS Code Docs
        </a>
        ,{" "}
        <a
          href="https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/"
          className="text-blue-600 hover:underline"
          target="_blank"
          rel="noopener noreferrer"
        >
          GitHub Blog (2500+ repos)
        </a>
        ,{" "}
        <a
          href="https://agentskills.io/specification"
          className="text-blue-600 hover:underline"
          target="_blank"
          rel="noopener noreferrer"
        >
          agentskills.io
        </a>
      </BodyShort>

      {/* Instructions */}
      <Box background="success-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-5">
          <FileTextIcon className="text-green-700" aria-hidden />
          <Heading size="small" level="3" className="text-green-700">
            Instructions
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-5">
          Instructions definerer kodestil og regler som alltid gjelder. Tenk på dem som teamets stilguide – korte,
          konkrete og sjelden i endring. Se{" "}
          <a
            href="https://code.visualstudio.com/docs/copilot/customization/custom-instructions"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            VS Code: Custom Instructions
          </a>
        </BodyShort>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
          <Box background="default" padding="space-12" borderRadius="4">
            <BodyShort weight="semibold" className="text-sm text-green-700 mb-2">
              Gode mønstre
            </BodyShort>
            <ul className="space-y-2 text-xs text-gray-600">
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Hold instruksjoner korte og selvstendige – én regel per punkt</span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>
                  Forklar <em>hvorfor</em> – &quot;Bruk date-fns i stedet for moment.js – moment er deprecated og øker
                  bundle size&quot;
                </span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Vis konkrete kodeeksempler (✅ Good / ❌ Bad)</span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Bruk applyTo-glob for filtype-spesifikke regler</span>
              </li>
            </ul>
          </Box>
          <Box background="default" padding="space-12" borderRadius="4">
            <BodyShort weight="semibold" className="text-sm text-red-700 mb-2">
              Vanlige feil
            </BodyShort>
            <ul className="space-y-2 text-xs text-gray-600">
              <li className="flex gap-2">
                <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>For lange filer – hold det fokusert, hopp over ting linteren allerede sjekker</span>
              </li>
              <li className="flex gap-2">
                <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Vage direktiver som &quot;skriv ren kode&quot; – vær konkret</span>
              </li>
              <li className="flex gap-2">
                <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Duplikate regler på tvers av filer – bruk Markdown-lenker for gjenbruk</span>
              </li>
              <li className="flex gap-2">
                <XMarkOctagonIcon className="text-red-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Alt i én fil – splitt i *.instructions.md per språk/rammeverk</span>
              </li>
            </ul>
          </Box>
        </HGrid>
        <Box background="default" padding="space-12" borderRadius="4" className="mt-4">
          <BodyShort weight="semibold" className="text-sm mb-2">
            Prioritet (ved konflikt)
          </BodyShort>
          <BodyShort className="text-gray-600 text-xs">
            1. Personlige instruksjoner (bruker-nivå) → 2. Repository-instruksjoner (copilot-instructions.md /
            AGENTS.md) → 3. Organisasjons-instruksjoner. Høyere prioritet vinner.
          </BodyShort>
        </Box>
      </Box>

      {/* Custom Agents */}
      <Box background="info-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-5">
          <CogIcon className="text-blue-700" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            Custom Agents
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-5">
          Agenter er spesialiserte roller med eget verktøysett og instruksjoner. Nøkkelen er spesifisitet – en god agent
          har én jobb, ikke ti. Se{" "}
          <a
            href="https://code.visualstudio.com/docs/copilot/customization/custom-agents"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            VS Code: Custom Agents
          </a>
        </BodyShort>

        <BodyShort weight="semibold" className="text-sm mb-2">
          Anbefalt struktur i agent-filen (fra{" "}
          <a
            href="https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            analyse av 2500+ repos
          </a>
          ):
        </BodyShort>
        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="mb-4">
          <Box background="default" padding="space-12" borderRadius="4">
            <ol className="space-y-2 text-xs text-gray-600 list-decimal list-inside">
              <li>
                <strong>YAML-frontmatter</strong> – name, description, tools
              </li>
              <li>
                <strong>Persona</strong> – én setning: hvem du er og hva du gjør
              </li>
              <li>
                <strong>Kommandoer</strong> – kjørbare kommandoer tidlig, med flagg
              </li>
              <li>
                <strong>Relaterte agenter</strong> – eventuelle handoffs
              </li>
              <li>
                <strong>Kodeeksempler</strong> – vis, ikke forklar
              </li>
              <li>
                <strong>Tre-trinns grenser</strong> – ✅ Alltid / ⚠️ Spør først / 🚫 Aldri
              </li>
            </ol>
          </Box>
          <Box background="default" padding="space-12" borderRadius="4">
            <BodyShort weight="semibold" className="text-sm mb-2">
              YAML-frontmatter felter
            </BodyShort>
            <ul className="space-y-1 text-xs text-gray-600">
              <li>
                <strong>description</strong> – kort beskrivelse (vises som placeholder i chat)
              </li>
              <li>
                <strong>tools</strong> – liste over tilgjengelige verktøy (f.eks. search, fetch, editFiles)
              </li>
              <li>
                <strong>model</strong> – valgfri AI-modell (én eller prioritert liste)
              </li>
              <li>
                <strong>handoffs</strong> – sekvensielle workflows mellom agenter
              </li>
              <li>
                <strong>agents</strong> – tillatte sub-agenter (bruk * for alle)
              </li>
            </ul>
          </Box>
        </HGrid>

        <Box background="danger-soft" padding="space-12" borderRadius="4">
          <BodyShort weight="semibold" className="text-sm text-red-700 mb-1">
            Vanligste feilen
          </BodyShort>
          <BodyShort className="text-gray-600 text-xs">
            &quot;You are a helpful coding assistant&quot; fungerer ikke. &quot;You are a test engineer who writes tests
            for React components, follows these examples, and never modifies source code&quot; fungerer. Spesifisitet
            slår generalitet.
          </BodyShort>
        </Box>
      </Box>

      {/* Agent Skills */}
      <Box background="accent-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8" className="mb-6">
        <div className="flex items-center gap-2 mb-5">
          <PencilWritingIcon className="text-blue-600" aria-hidden />
          <Heading size="small" level="3">
            Agent Skills
          </Heading>
        </div>
        <BodyShort className="text-gray-600 text-sm mb-5">
          Skills er gjenbrukbare kapabiliteter med skript og ressurser som Copilot laster automatisk når de er
          relevante. Åpen standard via{" "}
          <a
            href="https://agentskills.io/specification"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            agentskills.io
          </a>
          . Fungerer i VS Code, Copilot CLI og Coding Agent. Se{" "}
          <a
            href="https://code.visualstudio.com/docs/copilot/customization/agent-skills"
            className="text-blue-600 hover:underline"
            target="_blank"
            rel="noopener noreferrer"
          >
            VS Code: Agent Skills
          </a>
        </BodyShort>

        <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="mb-4">
          <VStack gap="space-16">
            <Box background="default" padding="space-12" borderRadius="4">
              <BodyShort weight="semibold" className="text-sm text-blue-700 mb-2">
                Progressive disclosure
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Copilot laster kun det som trengs i tre nivåer: 1) name + description (alltid synlig), 2) SKILL.md body
                (ved match), 3) scripts/resources (ved referanse). Installer mange skills uten å bruke kontekst.
              </BodyShort>
            </Box>
            <Box background="default" padding="space-12" borderRadius="4">
              <BodyShort weight="semibold" className="text-sm text-blue-700 mb-2">
                Invokering
              </BodyShort>
              <BodyShort className="text-gray-600 text-xs">
                Skills kan både brukes som /slash-commands og lastes automatisk basert på description-match. Kontroller
                med user-invokable og disable-model-invocation i frontmatter.
              </BodyShort>
            </Box>
          </VStack>
          <Box background="default" padding="space-12" borderRadius="4">
            <BodyShort weight="semibold" className="text-sm text-blue-700 mb-2">
              Mappestruktur
            </BodyShort>
            <pre className="text-xs font-mono text-gray-600 leading-relaxed">{`.github/skills/my-skill/
├── SKILL.md          # Påkrevd
├── scripts/          # Valgfri
│   └── run-tests.sh
├── references/       # Valgfri
│   └── FORMAT.md
└── examples/         # Valgfri`}</pre>
          </Box>
        </HGrid>

        <Box background="default" padding="space-12" borderRadius="4">
          <BodyShort weight="semibold" className="text-sm mb-2">
            Tips for gode skills
          </BodyShort>
          <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
            <ul className="space-y-2 text-xs text-gray-600">
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Skriv en presis description med trigger-ord som brukere faktisk sier</span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Hold SKILL.md body under 500 linjer – flytt detaljer til references/</span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>name i frontmatter må matche mappenavnet</span>
              </li>
            </ul>
            <ul className="space-y-2 text-xs text-gray-600">
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Inkluder skript som agenten kan kjøre for å verifisere arbeidet</span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>Skills er portable – fungerer i VS Code, CLI og Coding Agent</span>
              </li>
              <li className="flex gap-2">
                <CheckmarkCircleIcon className="text-green-600 shrink-0 mt-0.5" fontSize="1rem" aria-hidden />
                <span>
                  Se{" "}
                  <a
                    href="https://github.com/anthropics/skills"
                    className="text-blue-600 hover:underline"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    anthropics/skills
                  </a>{" "}
                  og{" "}
                  <a
                    href="https://github.com/github/awesome-copilot"
                    className="text-blue-600 hover:underline"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    awesome-copilot
                  </a>{" "}
                  for eksempler
                </span>
              </li>
            </ul>
          </HGrid>
        </Box>
      </Box>

      {/* Quick reference: when to use what */}
      <Box background="warning-soft" padding={{ xs: "space-12", sm: "space-16" }} borderRadius="8">
        <div className="flex items-center gap-2 mb-5">
          <LightBulbIcon className="text-orange-700" aria-hidden />
          <Heading size="small" level="3" className="text-orange-700">
            Når bruker du hva?
          </Heading>
        </div>
        <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
          <Box background="default" padding="space-12" borderRadius="4">
            <Label size="small" className="text-green-700">
              <a href="/verktoy?type=instruction" className="hover:underline">
                Instructions
              </a>
            </Label>
            <BodyShort className="text-gray-600 text-xs mt-1">
              Kodestil, navnekonvensjoner, sikkerhetsregler. Start med én copilot-instructions.md, utvid med
              *.instructions.md per språk.
            </BodyShort>
          </Box>
          <Box background="default" padding="space-12" borderRadius="4">
            <Label size="small" className="text-blue-700">
              <a href="/verktoy?type=agent" className="hover:underline">
                Agents
              </a>
            </Label>
            <BodyShort className="text-gray-600 text-xs mt-1">
              Spesialiserte roller som @test-agent, @docs-agent. Når du trenger eget verktøysett og persona. Støtter
              handoffs mellom agenter.
            </BodyShort>
          </Box>
          <Box background="default" padding="space-12" borderRadius="4">
            <Label size="small" className="text-purple-700">
              <a href="/verktoy?type=skill" className="hover:underline">
                Skills
              </a>
            </Label>
            <BodyShort className="text-gray-600 text-xs mt-1">
              Gjenbrukbare kapabiliteter med skript. Når du trenger portabilitet på tvers av VS Code, CLI og Coding
              Agent.
            </BodyShort>
          </Box>
        </HGrid>
      </Box>
    </Box>
  );
}
