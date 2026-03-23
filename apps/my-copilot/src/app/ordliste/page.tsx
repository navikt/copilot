import { PageHero } from "@/components/page-hero";
import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";

const terms = [
  {
    term: "Agent",
    definition:
      "En AI-drevet assistent som kan utføre flertrinnsoppgaver autonomt. En agent kan planlegge, bruke verktøy og ta beslutninger for å nå et mål – uten at brukeren trenger å styre hvert steg.",
  },
  {
    term: "Agent mode",
    definition:
      "Copilots modus der AI-en utfører flertrinnsoppgaver autonomt i editoren. Agenten kan redigere filer, kjøre terminalkode og bruke verktøy for å nå et mål.",
  },
  {
    term: "AGENTS.md",
    definition:
      "En konfigurasjonsfil i roten av et repository som gir AI-agenter kontekst om prosjektet – struktur, byggkommandoer, konvensjoner og grenser for hva agenten kan gjøre.",
  },
  {
    term: "Ask mode",
    definition:
      "Copilots spørremodus der du kan stille spørsmål og få svar og forklaringer uten at Copilot gjør endringer i kodebasen.",
  },
  {
    term: "Chat",
    definition:
      "Copilots samtalebaserte grensesnitt der du kan stille spørsmål, be om forklaringer og diskutere kode i naturlig språk. Tilgjengelig i editor, nettleser og som frittstående app.",
  },
  {
    term: "Completion",
    definition:
      "Svaret eller teksten Copilot genererer som respons på en prompt. Begrepet kommer fra den underliggende API-en der modellen «fullfører» teksten du starter.",
  },
  {
    term: "Copilot Extensions",
    definition:
      "Utvidelser som kobler GitHub Copilot til tredjepartstjenester og interne systemer. Gjør det mulig å bruke Copilot mot egne datakilder og verktøy direkte fra chat.",
  },
  {
    term: "Copilot Workspace",
    definition:
      "GitHubs agentic utviklingsmiljø der du kan gå fra en GitHub issue til ferdig pull request med AI-hjelp gjennom hele prosessen.",
  },
  {
    term: "Edit mode",
    definition:
      "Copilots redigeringsmodus der du beskriver en endring og Copilot redigerer relevante filer direkte, uten å utføre kommandoer eller bruke verktøy.",
  },
  {
    term: "Fine-tuning",
    definition:
      "Tilpasning av en AI-modell ved å trene den videre på spesifikke data. Gjør modellen mer presis for et bestemt domene eller kodestil.",
  },
  {
    term: "Hallusinasjon",
    definition:
      "Når en AI-modell genererer informasjon som virker troverdig, men er feil eller oppdiktet. Copilot kan hallusinere API-navn, funksjoner eller biblioteker som ikke finnes.",
  },
  {
    term: "Inline suggestion",
    definition:
      "Kodeforslag som vises direkte i editoren mens du skriver, uten at du trenger å åpne chat. Du aksepterer forslaget med Tab, eller avviser det ved å fortsette å skrive.",
  },
  {
    term: "Instructions",
    definition:
      "Konfigurasjonsfiler (.instructions.md) som gir Copilot vedvarende kontekst og regler for en fil, mappe eller hele prosjektet – uten at du trenger å gjenta dem i hver prompt.",
  },
  {
    term: "Knowledge cutoff",
    definition:
      "Datoen for den siste treningsdataen en AI-modell er basert på. Hendelser og teknologier etter denne datoen er ukjent for modellen.",
  },
  {
    term: "Kontekstvindu",
    definition:
      "Mengden tekst (målt i tokens) en AI-modell kan ta inn og huske på én gang. Innhold utenfor kontekstvinduet er ikke tilgjengelig for modellen i en gitt forespørsel.",
  },
  {
    term: "MCP (Model Context Protocol)",
    definition:
      "En åpen standard for å koble AI-modeller til eksterne verktøy og datakilder. MCP lar agenter og Copilot kommunisere med systemer utenfor selve modellen på en strukturert måte.",
  },
  {
    term: "Modell",
    definition:
      "Det underliggende AI-systemet som genererer svarene, for eksempel GPT-4o eller Claude Sonnet. Ulike modeller har ulike styrker, kontekststørrelser og kostnadsprofiler.",
  },
  {
    term: "Premium requests",
    definition:
      "Forespørsler til mer avanserte AI-modeller (for eksempel o3 eller Claude Opus) som trekker fra en separat kvote i Copilot-abonnementet.",
  },
  {
    term: "Prompt",
    definition:
      "Instruksjonen, spørsmålet eller konteksten du gir til AI-modellen. En god prompt gir tydelig kontekst og beskriver hva du ønsker, noe som gir bedre og mer relevante svar.",
  },
  {
    term: "RAG (Retrieval-Augmented Generation)",
    definition:
      "En teknikk der relevante dokumenter eller kodefragmenter hentes og legges inn i konteksten før modellen svarer. Gir mer presise svar basert på faktisk innhold.",
  },
  {
    term: "Session",
    definition:
      "En aktiv samtale eller arbeidsøkt med Copilot. Innenfor en session husker modellen tidligere meldinger og kontekst, inntil sesjonen avsluttes eller kontekstvinduet fylles opp.",
  },
  {
    term: "Skills",
    definition:
      "Spesifikke evner eller verktøy en agent kan bruke, for eksempel å søke i kode, lese filer, kalle et API eller kjøre tester. Skills definerer hva en agent er i stand til å gjøre.",
  },
  {
    term: "System prompt",
    definition:
      "En skjult instruksjon som definerer modellens rolle og atferd. Settes av verktøyet eller leverandøren, og er ikke synlig for brukeren. Brukes blant annet til å gi Copilot kontekst om editoren og kodebasen.",
  },
  {
    term: "Temperature",
    definition:
      "En parameter som styrer hvor kreativ eller forutsigbar modellen er. Høy temperatur gir mer varierte svar; lav temperatur gir mer presise og konsistente svar.",
  },
  {
    term: "Token",
    definition:
      "Den grunnleggende enheten AI-modeller bruker for å behandle tekst. Et token tilsvarer omtrent 3–4 tegn på norsk. Både input (din tekst) og output (Copilots svar) telles i tokens.",
  },
];

export default function OrdlistePage() {
  return (
    <main>
      <PageHero
        title="Ordliste"
        description="Enkle forklaringer på begreper brukt i forbindelse med GitHub Copilot og AI-assistert utvikling."
      />
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap="space-8">
          {terms.map(({ term, definition }) => (
            <Box key={term} borderColor="neutral" borderWidth="1" borderRadius="8" padding="space-20">
              <VStack gap="space-4">
                <Heading size="xsmall" level="2">
                  {term}
                </Heading>
                <BodyShort>{definition}</BodyShort>
              </VStack>
            </Box>
          ))}
        </VStack>
      </Box>
    </main>
  );
}
