export interface Term {
  term: string;
  definition: string;
  link?: { href: string; label: string };
}

// Termnavn: bruk engelsk for etablerte fagtermer (agent mode, hooks, tool calling).
// Bruk norsk når ordet er naturlig på norsk (hallusinasjon, kontekstvindu, modell).
// Definisjoner: alltid på norsk.
export const terms: Term[] = [
  {
    term: "Aksepteringsrate",
    definition:
      "Andelen kodeforslag fra Copilot som utviklere faktisk tar i bruk. Måles som forholdet mellom aksepterte og totalt viste forslag, og brukes til å vurdere hvor nyttig Copilot er i praksis.",
    link: { href: "/statistikk", label: "Se statistikk" },
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
    link: { href: "/praksis#vanlige-mønstre-for-agent-mode", label: "Mønstre for agent mode" },
  },
  {
    term: "Agentic loop",
    definition:
      "Arbeidssløyfen der en agent samler kontekst, planlegger, handler og verifiserer resultatet – i en kontinuerlig løkke til oppgaven er løst. Agenten gjentar syklusen og justerer kursen basert på resultater underveis.",
  },
  {
    term: "AGENTS.md",
    definition:
      "En konfigurasjonsfil i roten av et repository som gir AI-agenter kontekst om prosjektet – struktur, byggkommandoer, konvensjoner og grenser for hva agenten kan gjøre.",
    link: { href: "/praksis#skriv-effektive-tilpasninger", label: "Skriv effektive tilpasninger" },
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
    link: { href: "/praksis#wrap-metoden-for-coding-agent", label: "WRAP-metoden" },
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
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/copilot-cli", label: "GitHub Docs" },
  },
  {
    term: "Copilot code review",
    definition:
      "AI-genererte gjennomgangskommentarer på pull requests. Copilot analyserer endringene og foreslår forbedringer, på samme måte som en menneskelig reviewer.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/code-review", label: "GitHub Docs" },
  },
  {
    term: "Copilot Edits",
    definition:
      "Copilots redigeringsverktøy for å gjøre endringer på tvers av flere filer fra én enkelt prompt. Finnes i to moduser: edit mode (du velger filene) og agent mode (Copilot velger selv).",
    link: { href: "/praksis#verktøy-og-moduser", label: "Verktøy og moduser" },
  },
  {
    term: "Copilot Memory",
    definition:
      "Copilot lagrer innsikt om et repository – arkitekturbeslutninger, mønstre og konvensjoner – og bruker det til å gi mer presise forslag i fremtidige økter. Minnet er per repository og kan slås av.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/copilot-memory", label: "GitHub Docs" },
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
    link: { href: "/verktoy?type=agent", label: "Se agenter" },
  },
  {
    term: "Edit mode",
    definition:
      "Copilots redigeringsmodus der du beskriver en endring og Copilot redigerer relevante filer direkte, uten å utføre kommandoer eller bruke verktøy.",
  },
  {
    term: "Hallusinasjon",
    definition:
      "Når en AI-modell genererer informasjon som virker troverdig, men er feil eller oppdiktet. Copilot kan hallusinere API-navn, funksjoner eller biblioteker som ikke finnes.",
    link: { href: "/praksis#verifisering-nøkkelen-til-kvalitet", label: "Verifisering" },
  },
  {
    term: "Hooks",
    definition:
      "Egendefinerte shell-kommandoer som kjøres automatisk på bestemte punkter under en agent-kjøring – for eksempel før en commit eller etter en filendring. Lar deg tilpasse agentens oppførsel uten å endre selve agenten.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks", label: "GitHub Docs" },
  },
  {
    term: "Human-in-the-loop",
    definition:
      "Prinsippet om at et menneske godkjenner agentens handlinger underveis, i stedet for å la den kjøre helt autonomt. I Copilot styres dette med godkjenningsdialogene for terminal og filendringer.",
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
    link: { href: "/verktoy?type=instruction", label: "Se instruksjoner" },
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
    link: { href: "/praksis#forbered-for-suksess", label: "Forbered for suksess" },
  },
  {
    term: "MCP (Model Context Protocol)",
    definition:
      "En åpen standard for å koble AI-modeller til eksterne verktøy og datakilder. MCP lar agenter og Copilot bruke verktøy og data utenfor selve modellen.",
    link: { href: "/verktoy?type=mcp", label: "Se MCP-servere" },
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
    term: "Plan mode",
    definition:
      "Copilots planleggingsmodus der agenten først stiller oppklarende spørsmål og lager en steg-for-steg-plan før den begynner å skrive kode. Gir deg kontroll over retningen før agenten handler.",
  },
  {
    term: "Premium requests",
    definition:
      "Forespørsler til mer avanserte AI-modeller (for eksempel o3 eller Claude Opus) som trekker fra en separat kvote i Copilot-abonnementet.",
    link: { href: "/kostnad", label: "Se kostnad" },
  },
  {
    term: "Prompt",
    definition:
      "Instruksjonen, spørsmålet eller konteksten du gir til AI-modellen. Tydelig kontekst og presise instruksjoner gir bedre svar.",
    link: { href: "/praksis#prompt-engineering", label: "Prompt engineering" },
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
    link: { href: "/verktoy?type=skill", label: "Se skills" },
  },
  {
    term: "Subagent",
    definition:
      "En agent som startes av en annen agent for å utføre en avgrenset oppgave. Holder hovedkonteksten ren ved å isolere komplekse deloppgaver i en egen sesjon.",
  },
  {
    term: "Token",
    definition:
      "Den grunnleggende enheten AI-modeller bruker for å behandle tekst. Et token tilsvarer omtrent 3–4 tegn på norsk. Både input (din tekst) og output (Copilots svar) telles i tokens.",
  },
  {
    term: "Tool calling",
    definition:
      "Mekanismen der en agent velger og bruker verktøy underveis – som filoperasjoner, terminalen, MCP-servere eller websøk. Det er tool calling som gjør at agenten kan handle, ikke bare svare.",
  },
];
