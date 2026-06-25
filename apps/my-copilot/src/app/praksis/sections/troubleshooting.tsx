import { Heading, BodyShort, Box, Accordion, VStack } from "@navikt/ds-react";
import { ExclamationmarkTriangleIcon, WrenchIcon } from "@navikt/aksel-icons";

export default function Troubleshooting() {
  return (
    <VStack gap="space-32">
      <section>
        <div className="flex items-center gap-2 mb-3">
          <WrenchIcon className="text-blue-700" aria-hidden fontSize="1.5rem" />
          <Heading size="small" level="3" className="text-blue-700">
            Klassisk Feilsøking (Når Copilot stopper opp)
          </Heading>
        </div>
        <BodyShort className="text-gray-700 mb-4">
          Av og til vil GitHub Copilot slutte å gi forslag, miste konteksten, eller rett og slett krasje. Her er
          løsningene på de aller vanligste problemene vi ser i Nav.
        </BodyShort>
      </section>

      <Box background="neutral-soft" padding="space-16" borderRadius="8">
        <Accordion>
          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                1. Jeg får ingen forslag lenger (Inline Autocomplete virker ikke)
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  Dette er det desidert vanligste problemet og skyldes nesten alltid at lisensen din ikke kan
                  verifiseres, eller at nettverksforbindelsen har hengt seg opp.
                </BodyShort>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">Slik fikser du det i VS Code:</BodyShort>
                  <ol className="list-decimal pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>
                      Trykk på konto-ikonet (nederst til venstre) og velg <strong>Sign Out</strong> fra GitHub.
                    </li>
                    <li>Lukk VS Code helt.</li>
                    <li>
                      Åpne VS Code, trykk på konto-ikonet, og velg <strong>Sign in to Sync Settings</strong>.
                    </li>
                    <li>
                      Når du er logget inn, sjekk Copilot-ikonet nederst til høyre. Det skal ikke ha en strek over seg.
                    </li>
                  </ol>
                </div>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">Slik fikser du det i IntelliJ:</BodyShort>
                  <ol className="list-decimal pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>
                      Gå til <strong>Tools &gt; GitHub Copilot &gt; Logout</strong>.
                    </li>
                    <li>Restart IDE-en.</li>
                    <li>
                      Gå til <strong>Tools &gt; GitHub Copilot &gt; Login to GitHub</strong>.
                    </li>
                  </ol>
                </div>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>

          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                2. Agenten "spinner i det uendelige" eller svarer med feilmelding
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  Når du bruker Copilot Edits eller Agent-modus, og den plutselig slutter å gi svar eller klager på
                  kontekst-lengde, har du sannsynligvis sprengt <strong>Token Limit</strong>.
                </BodyShort>
                <BodyShort>
                  Agentene kan bare holde en viss mengde kode i minnet om gangen. Hvis du har lagt til hele
                  `/src`-mappen som kontekst, klarer ikke AI-modellen å prosessere alt.
                </BodyShort>
                <div className="pl-4 border-l-2 border-gray-300">
                  <BodyShort weight="semibold">Løsning:</BodyShort>
                  <ul className="list-disc pl-5 space-y-1 mt-2 text-gray-700 text-sm">
                    <li>
                      Start en <strong>helt ny chat-sesjon</strong> (bruk "New Thread" eller "+" knappen).
                    </li>
                    <li>
                      Vær mye mer selektiv med hvilke filer du legger til (bruk <code>@</code> for å kun ta med 1-3
                      relevante filer).
                    </li>
                  </ul>
                </div>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>

          <Accordion.Item>
            <Accordion.Header>
              <Heading size="xsmall" level="4">
                3. Copilot Edits ignorerer instruksjonene mine (.copilotignore feil)
              </Heading>
            </Accordion.Header>
            <Accordion.Content>
              <VStack gap="space-16">
                <BodyShort>
                  Hvis du opplever at agenten endrer filer du spesifikt har bedt den om å ignorere, sjekk at du har
                  konfigurert <code>.copilotignore</code> riktig.
                </BodyShort>
                <BodyShort>
                  Pass på at <code>.copilotignore</code> ligger i <strong>roten av prosjektet ditt</strong> (samme sted
                  som <code>.gitignore</code>). Hvis editoren din åpner en sub-mappe som root, må{" "}
                  <code>.copilotignore</code> ligge der editoren ser den.
                </BodyShort>
              </VStack>
            </Accordion.Content>
          </Accordion.Item>
        </Accordion>
      </Box>

      <Box background="danger-soft" padding="space-16" borderRadius="8">
        <div className="flex items-center gap-2 mb-3">
          <ExclamationmarkTriangleIcon className="text-red-700" aria-hidden />
          <Heading size="small" level="3" className="text-red-700">
            Fremdeles problemer?
          </Heading>
        </div>
        <BodyShort className="text-gray-800">
          Hvis ut-og-innlogging ikke løser problemet ditt, spør i <strong>#copilot-hjelp</strong> på Slack. Legg gjerne
          ved skjermbilde av feilmeldingen og nevn hvilken Editor og versjon du sitter på.
        </BodyShort>
      </Box>
    </VStack>
  );
}
