"use client";

import { useState } from "react";
import { Box, BodyShort, Heading, VStack, Button } from "@navikt/ds-react";
import {
  LightBulbIcon,
  ChevronRightIcon,
  PaperplaneIcon,
  SparklesIcon,
  XMarkIcon,
  ChatIcon,
  TasklistStartIcon,
  ShieldLockIcon,
  FileCheckmarkIcon,
  MagnifyingGlassIcon,
  WrenchIcon,
  PuzzlePieceIcon,
  RobotIcon,
  TerminalIcon,
  BarChartIcon,
  BookIcon,
  PlayIcon,
  GlassesIcon,
} from "@navikt/aksel-icons";
import NextLink from "next/link";

const iconMap: Record<string, React.ElementType> = {
  ChatIcon,
  TasklistStartIcon,
  ShieldLockIcon,
  FileCheckmarkIcon,
  MagnifyingGlassIcon,
  WrenchIcon,
  PuzzlePieceIcon,
  RobotIcon,
  TerminalIcon,
  BarChartIcon,
  BookIcon,
  PlayIcon,
  GlassesIcon,
};
import { Category, Guide } from "@/app/praksis/data";

type ClientGuide = Omit<Guide, "components">;
type ClientCategory = Omit<Category, "guides"> & { guides: ClientGuide[] };

export function PraksisHub({ categories }: { categories: ClientCategory[] }) {
  const [query, setQuery] = useState("");

  const filteredCategories = categories
    .map((cat) => ({
      ...cat,
      guides: cat.guides.filter(
        (g) =>
          g.title.toLowerCase().includes(query.toLowerCase()) ||
          g.description.toLowerCase().includes(query.toLowerCase()) ||
          cat.title.toLowerCase().includes(query.toLowerCase()) ||
          g.keywords?.some((k) => k.toLowerCase().includes(query.toLowerCase()))
      ),
    }))
    .filter((cat) => cat.guides.length > 0);

  return (
    <VStack gap={{ xs: "space-24", md: "space-32" }}>
      {/* Pseudo AI Chat */}
      <Box className="w-full max-w-3xl mx-auto mb-4">
        <div className="relative group">
          <div className="absolute -inset-1 bg-gradient-to-r from-blue-600 to-indigo-600 rounded-2xl blur opacity-15 group-focus-within:opacity-30 transition duration-500"></div>
          <div className="relative bg-surface-default border-[1.5px] border-blue-500 rounded-xl shadow-lg flex flex-col p-2 transition-all">
            <div className="flex px-3 pt-2 pb-1">
              <textarea
                value={query}
                aria-label="Søk i guider eller beskriv oppgaven din"
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Spør om hva som helst, eller beskriv hva du vil gjøre..."
                className="w-full bg-transparent resize-none outline-none text-text-default placeholder-text-subtle min-h-[60px]"
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                  }
                }}
              />
              {query && (
                <button
                  onClick={() => setQuery("")}
                  className="p-1 h-fit rounded-full hover:bg-surface-hover text-text-subtle transition-colors"
                >
                  <XMarkIcon title="Tøm søk" />
                </button>
              )}
            </div>

            <div className="flex justify-between items-center px-2 pb-1 pt-2 border-t border-border-subtle/50 mt-1">
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="tertiary-neutral"
                  size="xsmall"
                  icon={<SparklesIcon aria-hidden />}
                  className="rounded-full font-normal"
                >
                  Alle guider
                </Button>
              </div>
              <Button
                type="button"
                variant="tertiary"
                size="small"
                icon={<PaperplaneIcon aria-hidden />}
                className="rounded-lg hover:bg-blue-50"
                onClick={() => {
                  if (query.trim()) {
                    alert("Interaktiv chat er under utvikling! Akkurat nå filtrerer dette bare guidene nedenfor.");
                  }
                }}
              />
            </div>
          </div>
        </div>

        <div className="flex gap-2 justify-center mt-6 flex-wrap">
          {["Skrive prompts", "Sette opp agenter", "Kostnadsoptimalisering", "Mønstre"].map((term) => (
            <button
              key={term}
              className="px-3 py-1.5 text-xs font-medium rounded-full border border-border-subtle bg-surface-default text-text-subtle hover:text-text-default hover:border-border-default hover:shadow-sm transition-all flex items-center gap-1.5"
              onClick={() => setQuery(term)}
            >
              <SparklesIcon aria-hidden /> {term}
            </button>
          ))}
        </div>
      </Box>

      {/* Render Filtered Cards */}
      {filteredCategories.length === 0 ? (
        <Box padding="space-32" className="text-center text-gray-500">
          Fant ingen guider som matchet &quot;{query}&quot;. Prøv et annet søkeord!
        </Box>
      ) : (
        <VStack gap="space-48">
          {filteredCategories.map((category) => (
            <section key={category.title}>
              <VStack gap="space-16">
                <div>
                  <Heading level="2" size="medium" className="text-gray-900">
                    {category.title}
                  </Heading>
                  <BodyShort className="text-gray-600 mt-2">{category.description}</BodyShort>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                  {category.guides.map((guide) => {
                    const Icon = iconMap[guide.iconName as keyof typeof iconMap] || SparklesIcon;
                    return (
                      <Box
                        key={guide.id}
                        asChild
                        borderColor="neutral-subtleA"
                        borderWidth="1"
                        borderRadius="8"
                        padding="space-24"
                        className="hover:border-blue-500 hover:shadow-md bg-surface-default hover:bg-blue-50/50 transition-all duration-300 group flex flex-col h-full relative overflow-hidden"
                      >
                        <NextLink href={`/praksis/guide/${guide.id}`} className="no-underline text-inherit">
                          <div className="absolute -right-4 -bottom-4 text-blue-600 opacity-5 group-hover:opacity-15 group-hover:scale-110 transition-all duration-500 transform -rotate-12 group-hover:-rotate-6 pointer-events-none">
                            <Icon style={{ fontSize: "140px" }} aria-hidden />
                          </div>
                          <div className="relative z-10 flex flex-col h-full">
                            <Heading level="3" size="small" className="group-hover:text-blue-600 mb-2">
                              {guide.title}
                            </Heading>
                            <BodyShort size="small" className="text-gray-600 flex-1">
                              {guide.description}
                            </BodyShort>
                            <div className="mt-4 text-blue-600 flex items-center gap-1 font-medium text-sm">
                              Les guide <ChevronRightIcon aria-hidden />
                            </div>
                          </div>
                        </NextLink>
                      </Box>
                    );
                  })}
                </div>
              </VStack>
            </section>
          ))}
        </VStack>
      )}

      <Box background="info-soft" padding="space-16" borderRadius="8" className="mt-8">
        <div className="flex items-center gap-2 mb-2">
          <LightBulbIcon className="text-blue-700" aria-hidden />
          <Heading size="small" level="3" className="text-blue-700">
            Tips
          </Heading>
        </div>
        <BodyShort className="text-gray-700 text-sm">
          Copilot utvikles raskt – hold deg oppdatert via GitHub Blog og awesome-copilot. Husk at agenten er et verktøy:
          du eier arkitekturen, den implementerer.
        </BodyShort>
      </Box>
    </VStack>
  );
}
