import { Heading, BodyShort, Box, HGrid } from "@navikt/ds-react";
import { LinkableHeading } from "@/components/linkable-heading";
import { BookIcon, StarIcon, CogIcon, ShieldLockIcon, BranchingIcon } from "@navikt/aksel-icons";

export default function Resources() {
  return (
    <Box background="neutral-soft" padding={{ xs: "space-12", sm: "space-16", md: "space-24" }} borderRadius="12">
      <LinkableHeading size="medium" level="2" className="mb-3">
        Ressurser
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-600 mb-6">
        Offisielle kilder, fellesskapsressurser og Nav-spesifikk dokumentasjon.
      </BodyShort>

      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        <Box background="info-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <BookIcon className="text-blue-600" aria-hidden />
            <Heading size="small" level="3">
              Offisiell dokumentasjon
            </Heading>
          </div>
          <ul className="space-y-2">
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://docs.github.com/en/copilot"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                GitHub Copilot Docs
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://docs.github.com/en/copilot/get-started/best-practices"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Best Practices (Official)
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://docs.github.com/en/copilot/concepts/prompting/prompt-engineering"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Prompt Engineering Guide
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://docs.github.com/en/copilot/managing-copilot/monitoring-usage-and-entitlements/about-premium-requests"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Premium Requests & Limits
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://github.blog/changelog/2025-10-28-a-mission-control-to-assign-steer-and-track-copilot-coding-agent-tasks/"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Mission Control Changelog
              </a>
            </li>
          </ul>
        </Box>

        <Box background="success-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <StarIcon className="text-green-600" aria-hidden />
            <Heading size="small" level="3">
              Fellesskapsressurser (offisielle)
            </Heading>
          </div>
          <ul className="space-y-2">
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <a
                href="https://github.com/github/awesome-copilot"
                className="text-green-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Awesome Copilot – offisiell kuratert samling av prompts, instructions, agents og skills
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <a
                href="https://github.com/github/spec-kit"
                className="text-green-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Spec Kit – GitHubs offisielle verktøy for Spec-Driven Development (60k+ ⭐)
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <a
                href="https://github.com/anthropics/skills"
                className="text-green-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Anthropic Skills – offisielle eksempler på Agent Skills
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-green-600">▪</span>
              <a
                href="https://github.blog/tag/github-copilot/"
                className="text-green-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                GitHub Blog – Copilot-artikler
              </a>
            </li>
          </ul>
        </Box>

        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <CogIcon className="text-gray-600" aria-hidden />
            <Heading size="small" level="3">
              Verifiseringsverktøy
            </Heading>
          </div>
          <ul className="space-y-2">
            <li className="flex gap-2">
              <span className="text-gray-600">▪</span>
              <a
                href="https://knip.dev/"
                className="text-gray-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Knip – Finn ubrukt kode, deps og exports i JS/TS-prosjekter
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-gray-600">▪</span>
              <a
                href="https://knip.dev/blog/for-editors-and-agents"
                className="text-gray-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Knip for Editors & Agents – Integrasjon med AI-verktøy
              </a>
            </li>
          </ul>
        </Box>

        <Box background="warning-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <ShieldLockIcon className="text-orange-600" aria-hidden />
            <Heading size="small" level="3">
              Sikkerhet og tillit
            </Heading>
          </div>
          <ul className="space-y-2">
            <li className="flex gap-2">
              <span className="text-orange-600">▪</span>
              <a
                href="https://copilot.github.trust.page/"
                className="text-orange-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                GitHub Copilot Trust Center
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-orange-600">▪</span>
              <a
                href="https://docs.github.com/en/copilot/managing-copilot"
                className="text-orange-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Copilot Policy & Security
              </a>
            </li>
          </ul>
        </Box>

        <Box background="accent-soft" padding="space-16" borderRadius="8">
          <div className="flex items-center gap-2 mb-2">
            <BranchingIcon className="text-blue-600" aria-hidden />
            <Heading size="small" level="3">
              Nav-spesifikk
            </Heading>
          </div>
          <ul className="space-y-2">
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://utvikling.intern.nav.no/teknisk/github-copilot.html"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Om GitHub Copilot i Nav
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="https://mcp-registry.nav.no"
                className="text-blue-600 hover:underline text-sm"
                target="_blank"
                rel="noopener noreferrer"
              >
                Nav MCP Registry – godkjente MCP-servere
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a href="/ordliste" className="text-blue-600 hover:underline text-sm">
                Ordliste – begreper og forkortelser i Copilot-økosystemet
              </a>
            </li>
            <li className="flex gap-2">
              <span className="text-blue-600">▪</span>
              <a
                href="/verktoy?item=mcp-io.github.navikt%2Fmcp-onboarding"
                className="text-blue-600 hover:underline text-sm"
              >
                MCP Onboarding – sjekk agent-beredskap og generer tilpasningsfiler
              </a>
            </li>
          </ul>
        </Box>
      </HGrid>
    </Box>
  );
}
