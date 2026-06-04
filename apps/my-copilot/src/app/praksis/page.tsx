import { Heading, BodyShort, Box, VStack } from "@navikt/ds-react";
import { PageHero } from "@/components/page-hero";
import { TableOfContents } from "@/components/table-of-contents";
import { BackToTop } from "@/components/back-to-top";
import { LightBulbIcon } from "@navikt/aksel-icons";
import { LevelSection, LevelTransition } from "@/components/level-section";
import StrengthsLimitations from "./sections/strengths-limitations";
import ToolsAndModes from "./sections/tools-and-modes";
import PrepareForSuccess from "./sections/prepare-for-success";
import EffectiveCustomizations from "./sections/effective-customizations";
import PromptEngineering from "./sections/prompt-engineering";
import CostOptimization from "./sections/cost-optimization";
import WrapMethod from "./sections/wrap-method";
import OrchestrateAgents from "./sections/orchestrate-agents";
import ReviewCopilotWork from "./sections/review-copilot-work";
import Verification from "./sections/verification";
import AgentModePatterns from "./sections/agent-mode-patterns";
import Resources from "./sections/resources";

export default async function BestPractices() {
  return (
    <main>
      <PageHero
        title="God praksis"
        description="Lær å bruke GitHub Copilot effektivt og trygt — fra kodeforslag til autonome agenter."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="flex gap-12">
            <aside className="hidden lg:block w-56 shrink-0">
              <div className="sticky top-6">
                <TableOfContents
                  items={[
                    {
                      id: "grunnleggende",
                      label: "Nivå 1 · Grunnleggende",
                      children: [
                        { id: "styrker-begrensninger-og-farer", label: "Styrker og farer" },
                        { id: "verktøy-og-moduser", label: "Verktøy og moduser" },
                        { id: "gjennomgå-copilots-arbeid", label: "Gjennomgå arbeid" },
                        { id: "verifisering-nøkkelen-til-kvalitet", label: "Verifisering" },
                      ],
                    },
                    {
                      id: "mellomnivå",
                      label: "Nivå 2 · Mellomnivå",
                      children: [
                        { id: "prompt-engineering", label: "Prompt engineering" },
                        { id: "kostnadsoptimalisering-i-praksis", label: "Kostnadsoptimalisering" },
                        { id: "forbered-for-suksess", label: "Forbered for suksess" },
                      ],
                    },
                    {
                      id: "avansert",
                      label: "Nivå 3 · Avansert",
                      children: [
                        { id: "skriv-effektive-tilpasninger", label: "Effektive tilpasninger" },
                        { id: "wrap-metoden-for-coding-agent", label: "WRAP-metoden" },
                        { id: "orkestrer-og-styr-agenter", label: "Orkestrer agenter" },
                        { id: "vanlige-mønstre-for-agent-mode", label: "Vanlige mønstre" },
                      ],
                    },
                    { id: "ressurser", label: "Ressurser" },
                  ]}
                />
              </div>
            </aside>
            <div className="min-w-0 flex-1">
              <VStack gap={{ xs: "space-24", md: "space-32" }}>
                {/* Grunnleggende */}
                <LevelSection
                  level="grunnleggende"
                  title="Grunnleggende"
                  description="Start her. Dette trenger alle som bruker Copilot."
                >
                  <StrengthsLimitations />
                  <ToolsAndModes />
                  <ReviewCopilotWork />
                  <Verification />
                </LevelSection>

                <LevelTransition text="Bra! Du har grunnlaget. Klar for neste nivå?" />

                {/* Mellomnivå */}
                <LevelSection
                  level="mellom"
                  title="Mellomnivå"
                  description="Etter noen ukers bruk. Gjør Copilot til en bedre partner."
                >
                  <PromptEngineering />
                  <CostOptimization />
                  <PrepareForSuccess />
                </LevelSection>

                <LevelTransition text="Du mestrer verktøyet. Klar for å bli en power user?" />

                {/* Avansert */}
                <LevelSection level="avansert" title="Avansert" description="For de som vil utnytte hele økosystemet.">
                  <EffectiveCustomizations />
                  <WrapMethod />
                  <OrchestrateAgents />
                  <AgentModePatterns />
                </LevelSection>

                {/* Ressurser */}
                <Resources />

                {/* Footer tip */}
                <Box background="info-soft" padding="space-16" borderRadius="8">
                  <div className="flex items-center gap-2 mb-2">
                    <LightBulbIcon className="text-blue-700" aria-hidden />
                    <Heading size="small" level="3" className="text-blue-700">
                      Tips
                    </Heading>
                  </div>
                  <BodyShort className="text-gray-700 text-sm">
                    Copilot utvikles raskt – hold deg oppdatert via GitHub Blog og awesome-copilot. Husk at agenten er
                    et verktøy: du eier arkitekturen, den implementerer.
                  </BodyShort>
                </Box>
              </VStack>
            </div>
          </div>
        </Box>
      </div>
      <BackToTop />
    </main>
  );
}
