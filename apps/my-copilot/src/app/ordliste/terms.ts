export interface Term {
  term: string;
  definition: string;
  link?: { href: string; label: string };
}

export const terms: Term[] = [
  {
    term: "Aksepteringsrate",
    definition:
      "Andelen kodeforslag fra Copilot som utviklere faktisk tar i bruk. Måles som forholdet mellom aksepterte og totalt viste forslag, og brukes til å vurdere hvor nyttig Copilot er i praksis.",
  },
  {
    term: "Agent",
    definition:
      "En AI-drevet assistent som kan utføre flertrinnsoppgaver autonomt – planlegge, bruke verktøy og ta beslutninger for å nå et mål uten at du trenger å styre hvert steg.",
  },
  {
    term: "Agent mode",
    definition:
      "Copilots modus der AI-en jobber autonomt i editoren. Agenten kan redigere filer, kjøre kommandoer og bruke verktøy for å løse oppgaver i flere steg.",
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
    term: "Coding agent",
    definition:
      "Copilots autonome agent på GitHub. Du tildeler en issue til Copilot, og agenten skriver kode, kjører tester og oppretter en pull request du kan gjennomgå.",
  },
  {
    term: "Completion",
    definition:
      "Svaret eller teksten Copilot genererer som svar på en prompt. Begrepet kommer fra API-en der modellen «fullfører» teksten du starter.",
  },
  {
    term: "Copilot CLI",
    definition:
      "Copilot i terminalen. Lar deg stille spørsmål, gjøre endringer i lokale filer og samhandle med GitHub – for eksempel opprette issues eller liste pull requests.",
  },
  {
    term: "Copilot code review",
    definition:
      "AI-genererte gjennomgangskommentarer på pull requests. Copilot analyserer endringene og foreslår forbedringer, på samme måte som en menneskelig reviewer.",
  },
  {
    term: "Copilot Edits",
    definition:
      "Copilots redigeringsverktøy for å gjøre endringer på tvers av flere filer fra én enkelt prompt. Finnes i to moduser: edit mode (du velger filene) og agent mode (Copilot velger selv).",
  },
  {
    term: "Copilot Extensions",
    definition:
      "Utvidelser som kobler GitHub Copilot til tredjepartstjenester og interne systemer. Lar deg bruke Copilot mot egne datakilder og verktøy direkte fra chat.",
  },
  {
    term: "Copilot Workspace",
    definition:
      "GitHubs agentdrevne utviklingsmiljø der du kan gå fra en GitHub issue til ferdig pull request med AI-hjelp.",
  },
  {
    term: "Custom agents",
    definition:
      "Spesialiserte Copilot-agenter definert i .agent.md-filer. Hver agent har egne instruksjoner, verktøytilgang og kontekst, og kan velges fra agent-menyen i editoren.",
  },
  {
    term: "Edit mode",
    definition:
      "Copilots redigeringsmodus der du beskriver en endring og Copilot redigerer relevante filer direkte, uten å utføre kommandoer eller bruke verktøy.",
  },
  {
    term: "Fine-tuning",
    definition:
      "Tilpasning av en AI-modell ved å trene den videre på spesifikke data. Gjør modellen mer presis for et domene eller en kodestil.",
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
      "Datoen for den siste treningsdataen en AI-modell er basert på. Hendelser og teknologier etter denne datoen er ukjente for modellen.",
  },
  {
    term: "Kontekstvindu",
    definition:
      "Mengden tekst (målt i tokens) en AI-modell kan ta inn og huske på én gang. Innhold utenfor kontekstvinduet er ikke tilgjengelig for modellen i en gitt forespørsel.",
  },
  {
    term: "MCP (Model Context Protocol)",
    definition:
      "En åpen standard for å koble AI-modeller til eksterne verktøy og datakilder. MCP lar agenter og Copilot bruke verktøy og data utenfor selve modellen.",
  },
  {
    term: "Modell",
    definition:
      "AI-systemet som genererer svarene, for eksempel GPT-4o eller Claude Sonnet. Ulike modeller har ulike styrker, kontekststørrelser og kostnader.",
  },
  {
    term: "Next Edit Suggestions (NES)",
    definition:
      "Copilot forutser hvor du mest sannsynlig vil gjøre neste endring, og foreslår koden på riktig sted. Til forskjell fra inline suggestions, som fullfører der markøren står, hopper NES til neste relevante posisjon.",
  },
  {
    term: "Premium requests",
    definition:
      "Forespørsler til mer avanserte AI-modeller (for eksempel o3 eller Claude Opus) som trekker fra en separat kvote i Copilot-abonnementet.",
  },
  {
    term: "Prompt",
    definition:
      "Instruksjonen, spørsmålet eller konteksten du gir til AI-modellen. Tydelig kontekst og presise instruksjoner gir bedre svar.",
  },
  {
    term: "Prompt-filer",
    definition:
      "Gjenbrukbare prompt-maler (.prompt.md) som du kan kjøre med en slash-kommando i Copilot Chat. Nyttig for oppgaver du gjør ofte, som kodegjennomgang eller generering av tester.",
  },
  {
    term: "RAG (Retrieval-Augmented Generation)",
    definition:
      "En teknikk der relevante dokumenter eller kodefragmenter hentes og legges inn i konteksten før modellen svarer. Gir mer presise svar fordi modellen har tilgang til konkret innhold.",
  },
  {
    term: "Session",
    definition:
      "En aktiv samtale eller arbeidsøkt med Copilot. Innenfor en session husker modellen tidligere meldinger og kontekst, inntil sesjonen avsluttes eller kontekstvinduet fylles opp.",
  },
  {
    term: "Skills",
    definition:
      "Evner eller verktøy en agent kan bruke, for eksempel å søke i kode, lese filer, kalle et API eller kjøre tester. Skillsene bestemmer hva agenten kan gjøre.",
  },
  {
    term: "System prompt",
    definition:
      "En skjult instruksjon som definerer modellens rolle og atferd. Settes av verktøyet eller leverandøren og er ikke synlig for deg. Gir blant annet Copilot kontekst om editoren og kodebasen.",
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
