export type Category = "konsepter" | "copilot" | "verktoy" | "sikkerhet" | "maling";

// Fixed order; the token colours the nodes on /ordbok and is read at runtime, so the theme decides.
export const categories: { id: Category; label: string; token: string }[] = [
  { id: "konsepter", label: "Konsepter", token: "--ax-bg-accent-strong" },
  { id: "copilot", label: "Copilot-funksjoner", token: "--ax-bg-success-strong" },
  { id: "verktoy", label: "Verktøy og protokoller", token: "--ax-bg-meta-purple-strong" },
  { id: "sikkerhet", label: "Sikkerhet og styring", token: "--ax-bg-danger-strong" },
  { id: "maling", label: "Måling og kostnad", token: "--ax-bg-warning-strong" },
];

export interface Term {
  term: string;
  category: Category;
  definition: string;
  link?: { href: string; label: string };
  // Links the definition text does not capture; feeds the network on /ordbok. Use sparingly.
  related?: string[];
}

// Termnavn: bruk engelsk for etablerte fagtermer (agent mode, hooks, tool calling).
// Bruk norsk når ordet er naturlig på norsk (hallusinasjon, kontekstvindu, modell).
// Definisjoner: alltid på norsk.
export const terms: Term[] = [
  {
    term: "Aksepteringsrate",
    category: "maling",
    definition:
      "Andelen kodeforslag fra Copilot som utviklere faktisk tar i bruk. Måles som forholdet mellom aksepterte og totalt viste forslag, og brukes til å vurdere hvor nyttig Copilot er i praksis.",
    link: { href: "/innsikt/bruk", label: "Se bruk og kostnad" },
    related: ["Inline suggestion"],
  },
  {
    term: "Agent",
    category: "konsepter",
    definition:
      "En KI-drevet assistent som kan utføre flertrinnsoppgaver autonomt – planlegge, bruke verktøy og ta beslutninger for å nå et mål uten at du trenger å styre hvert steg.",
  },
  {
    term: "Agentisk KI",
    category: "konsepter",
    definition:
      "Samlebegrep for KI-systemer som handler på egen hånd mot et mål, i stedet for å svare på ett og ett spørsmål. Brukes som adjektiv, for eksempel «agentisk arbeidsflyt».",
    related: ["Agent"],
  },
  {
    term: "Agency",
    category: "konsepter",
    definition:
      "Hvor stor handlefrihet agenten har, altså hvilke verktøy den får bruke og hvilke beslutninger den tar selv. Vi beholder som regel termen på engelsk i fagkontekst.",
  },
  {
    term: "Agent mode",
    category: "copilot",
    definition:
      "Copilots modus der KI-en jobber autonomt i editoren. Agenten kan redigere filer, kjøre kommandoer og bruke verktøy for å løse oppgaver i flere steg.",
    link: { href: "/praksis/guide/orkestrere-agenter", label: "Mønstre for agent mode" },
  },
  {
    term: "Agent harness",
    category: "konsepter",
    definition:
      "Kjøretidsmiljøet som kjører en KI-agent – for eksempel Copilot CLI eller OpenCode. Harnessen styrer hvilke verktøy agenten har tilgang til og hvordan den samhandler med operativsystemet.",
  },
  {
    term: "Allowlist (MCP)",
    category: "sikkerhet",
    definition:
      "Listen over godkjente MCP-servere i Nav. Kun servere på denne listen kan brukes med Copilot. Styres via org policy på GitHub-organisasjonsnivå.",
    link: { href: "/verktoy?type=mcp", label: "Se godkjente MCP-servere" },
  },
  {
    term: "Agentic loop",
    category: "konsepter",
    definition:
      "Arbeidssløyfen der en agent samler kontekst, planlegger, handler og verifiserer resultatet – i en kontinuerlig løkke til oppgaven er løst. Agenten gjentar syklusen og justerer kursen basert på resultater underveis.",
  },
  {
    term: "AGENTS.md",
    category: "verktoy",
    definition:
      "En konfigurasjonsfil i roten av et repository som gir KI-agenter kontekst om prosjektet – struktur, byggkommandoer, konvensjoner og grenser for hva agenten kan gjøre.",
    link: { href: "/praksis/guide/skreddersy-med-skills-og-rules", label: "Skriv effektive tilpasninger" },
  },
  {
    term: "Ask mode",
    category: "copilot",
    definition:
      "Copilots spørremodus der du kan stille spørsmål og få svar og forklaringer uten at Copilot gjør endringer i kodebasen.",
    related: ["Agent mode", "Edit mode"],
  },
  {
    term: "Autonomi",
    category: "konsepter",
    definition:
      "Hvor selvstendig agenten kjører uten at et menneske godkjenner hvert steg. Høy autonomi betyr færre stopp for bekreftelse underveis.",
  },
  {
    term: "Chat",
    category: "copilot",
    definition:
      "Copilots samtalebaserte grensesnitt der du kan stille spørsmål, be om forklaringer og diskutere kode i naturlig språk. Tilgjengelig i editor, nettleser og som frittstående app.",
  },
  {
    term: "Coding agent",
    category: "copilot",
    definition:
      "Copilots autonome agent på GitHub. Du tildeler en issue til Copilot, og agenten skriver kode, kjører tester og oppretter en pull request du kan gjennomgå.",
    link: { href: "/praksis/guide/skrive-presise-prompts", label: "WRAP-metoden" },
  },
  {
    term: "Completion",
    category: "konsepter",
    definition:
      "Svaret eller teksten Copilot genererer som svar på en prompt. Begrepet kommer fra API-en der modellen «fullfører» teksten du starter.",
  },
  {
    term: "Copilot CLI",
    category: "verktoy",
    definition:
      "Copilot i terminalen. Lar deg stille spørsmål, gjøre endringer i lokale filer og samhandle med GitHub – for eksempel opprette issues eller liste pull requests.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/copilot-cli", label: "GitHub Docs" },
  },
  {
    term: "Copilot code review",
    category: "copilot",
    definition:
      "KI-genererte gjennomgangskommentarer på pull requests. Copilot analyserer endringene og foreslår forbedringer, på samme måte som en menneskelig reviewer.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/code-review", label: "GitHub Docs" },
  },
  {
    term: "Copilot Edits",
    category: "copilot",
    definition:
      "Copilots redigeringsverktøy for å gjøre endringer på tvers av flere filer fra én enkelt prompt. Finnes i to moduser: edit mode (du velger filene) og agent mode (Copilot velger selv).",
    link: { href: "/praksis/guide/velge-riktig-verktoy", label: "Verktøy og moduser" },
  },
  {
    term: "Copilot Memory",
    category: "copilot",
    definition:
      "Copilot lagrer innsikt om et repository – arkitekturbeslutninger, mønstre og konvensjoner – og bruker det til å gi mer presise forslag i fremtidige økter. Minnet er per repository og kan slås av.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/copilot-memory", label: "GitHub Docs" },
    related: ["Session"],
  },
  {
    term: "Copilot Extensions",
    category: "verktoy",
    definition:
      "Utvidelser som kobler GitHub Copilot til tredjepartstjenester og interne systemer. Lar deg bruke Copilot mot egne datakilder og verktøy direkte fra chat.",
  },
  {
    term: "Copilot Workspace",
    category: "copilot",
    definition:
      "GitHubs agentdrevne utviklingsmiljø der du kan gå fra en GitHub issue til ferdig pull request med KI-hjelp.",
    related: ["Coding agent"],
  },
  {
    term: "Context exclusion",
    category: "sikkerhet",
    definition:
      "Regler som ekskluderer bestemte filer fra konteksten som sendes til KI-modellen. I Nav bruker vi dette til å holde .env-filer og andre hemmeligheter unna inference context. Kan settes per repo eller globalt på org-nivå.",
  },
  {
    term: "Custom agents",
    category: "copilot",
    definition:
      "Spesialiserte Copilot-agenter definert i .agent.md-filer. Hver agent har egne instruksjoner, verktøytilgang og kontekst, og kan velges fra agent-menyen i editoren.",
    link: { href: "/verktoy?type=agent", label: "Se agenter" },
  },
  {
    term: "Edit mode",
    category: "copilot",
    definition:
      "Copilots redigeringsmodus der du beskriver en endring og Copilot redigerer relevante filer direkte, uten å utføre kommandoer eller bruke verktøy.",
  },
  {
    term: "Excessive agency",
    category: "sikkerhet",
    definition:
      "Sikkerhetsbegrep: agenten har fått mer handlefrihet, tilgang eller autonomi enn oppgaven krever, og kan derfor gjøre utilsiktet skade.",
  },
  {
    term: "Hallusinasjon",
    category: "konsepter",
    definition:
      "Når en KI-modell genererer informasjon som virker troverdig, men er feil eller oppdiktet. Copilot kan hallusinere API-navn, funksjoner eller biblioteker som ikke finnes.",
    link: { href: "/praksis/guide/skrive-og-kjore-tester", label: "Verifisering" },
  },
  {
    term: "Hooks",
    category: "verktoy",
    definition:
      "Egendefinerte shell-kommandoer som kjøres automatisk på bestemte punkter under en agent-kjøring – for eksempel før en commit eller etter en filendring. Lar deg tilpasse agentens oppførsel uten å endre selve agenten.",
    link: { href: "https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-hooks", label: "GitHub Docs" },
  },
  {
    term: "Human-in-the-loop",
    category: "konsepter",
    definition:
      "Prinsippet om at et menneske godkjenner agentens handlinger underveis, i stedet for å la den kjøre helt autonomt. I Copilot styres dette med godkjenningsdialogene for terminal og filendringer.",
    related: ["Autonomi"],
  },
  {
    term: "Inline suggestion",
    category: "copilot",
    definition:
      "Kodeforslag som vises direkte i editoren mens du skriver, uten at du trenger å åpne chat. Du aksepterer forslaget med Tab, eller avviser det ved å fortsette å skrive.",
  },
  {
    term: "Instructions",
    category: "verktoy",
    definition:
      "Konfigurasjonsfiler (.instructions.md) som gir Copilot vedvarende kontekst og regler for en fil, mappe eller hele prosjektet – uten at du trenger å gjenta dem i hver prompt.",
    link: { href: "/verktoy?type=instruction", label: "Se instruksjoner" },
  },
  {
    term: "Knowledge cutoff",
    category: "konsepter",
    definition:
      "Datoen for den siste treningsdataen en KI-modell er basert på. Hendelser og teknologier etter denne datoen er ukjente for modellen.",
  },
  {
    term: "Kontekstvindu",
    category: "konsepter",
    definition:
      "Mengden tekst (målt i tokens) en KI-modell kan ta inn og huske på én gang. Innhold utenfor kontekstvinduet er ikke tilgjengelig for modellen i en gitt forespørsel.",
    link: { href: "/praksis/guide/forberede-prosjektet", label: "Forbered for suksess" },
  },
  {
    term: "Inference context",
    category: "sikkerhet",
    definition:
      "Dataene som sendes til KI-modellen i en forespørsel – kode, filer, instruksjoner og samtalehistorikk. Innholdet kastes etter at svaret er generert og brukes ikke til trening.",
  },
  {
    term: "MCP (Model Context Protocol)",
    category: "verktoy",
    definition:
      "En åpen standard for å koble KI-modeller til eksterne verktøy og datakilder. MCP-servere kan sende kode og kontekst til eksterne tjenester, og krever derfor godkjenning via org policy i Nav.",
    link: { href: "/verktoy?type=mcp", label: "Se MCP-servere" },
  },
  {
    term: "Model provider",
    category: "verktoy",
    definition:
      "Tjenesten som kjører KI-modellen – for eksempel OpenAI, Anthropic eller Google. GitHub Copilot API fungerer som gateway og ruter forespørsler til riktig provider. Navs databehandleravtale er med GitHub, ikke direkte med providerne.",
  },
  {
    term: "Modell",
    category: "konsepter",
    definition: "KI-systemet som genererer svarene. Ulike modeller har ulike styrker, kontekststørrelser og kostnader.",
  },
  {
    term: "Next Edit Suggestions (NES)",
    category: "copilot",
    definition:
      "Copilot forutser hvor du mest sannsynlig vil gjøre neste endring, og foreslår koden på riktig sted. Til forskjell fra inline suggestions, som fullfører der markøren står, hopper NES til neste relevante posisjon.",
  },
  {
    term: "OpenCode",
    category: "verktoy",
    definition:
      "En av to godkjente agent-harnesser i Nav (sammen med Copilot CLI). OpenCode er en uavhengig open source-agent som bruker GitHub Copilot som model provider. Kjøres i terminalen.",
    link: { href: "https://opencode.ai", label: "opencode.ai" },
  },
  {
    term: "Plan mode",
    category: "copilot",
    definition:
      "Copilots planleggingsmodus der agenten først stiller oppklarende spørsmål og lager en steg-for-steg-plan før den begynner å skrive kode. Gir deg kontroll over retningen før agenten handler.",
  },
  {
    term: "Premium requests",
    category: "maling",
    definition:
      "Forespørsler til mer avanserte KI-modeller (for eksempel o3 eller Claude Opus) som trekker fra en separat kvote i Copilot-abonnementet.",
    link: { href: "/innsikt/bruk#kostnad", label: "Se kostnad" },
  },
  {
    term: "Prompt",
    category: "konsepter",
    definition:
      "Instruksjonen, spørsmålet eller konteksten du gir til KI-modellen. Tydelig kontekst og presise instruksjoner gir bedre svar.",
    link: { href: "/praksis/guide/skrive-presise-prompts", label: "Prompt engineering" },
  },
  {
    term: "RAG (Retrieval-Augmented Generation)",
    category: "konsepter",
    definition:
      "En teknikk der relevante dokumenter eller kodefragmenter hentes og legges inn i konteksten før modellen svarer. Gir mer presise svar fordi modellen har tilgang til konkret innhold.",
  },
  {
    term: "Session",
    category: "konsepter",
    definition:
      "En aktiv samtale eller arbeidsøkt med Copilot. Innenfor en session husker modellen tidligere meldinger og kontekst, inntil sesjonen avsluttes eller kontekstvinduet fylles opp.",
  },
  {
    term: "Sandbox (cplt)",
    category: "sikkerhet",
    definition:
      "Kernel-nivå isolasjon som begrenser hva en KI-agent kan gjøre på utviklermaskinen. cplt blokkerer tilgang til hemmeligheter, nøkler og .env-filer, og kontrollerer nettverkstrafikk. Operativsystemet håndhever reglene – det avhenger ikke av tillit til agenten.",
    link: { href: "/cplt", label: "Om cplt" },
  },
  {
    term: "Skills",
    category: "verktoy",
    definition:
      "Instruksjoner (prompts) som gir agenten domenekunnskap og mønstre for å løse bestemte oppgaver. Skills gir ikke agenten ekstra tilgang – de styrer bare hvordan agenten bruker verktøyene den allerede har.",
    link: { href: "/verktoy?type=skill", label: "Se skills" },
  },
  {
    term: "Subagent",
    category: "konsepter",
    definition:
      "En agent som startes av en annen agent for å utføre en avgrenset oppgave. Holder hovedkonteksten ren ved å isolere komplekse deloppgaver i en egen sesjon.",
  },
  {
    term: "Token",
    category: "maling",
    definition:
      "Den grunnleggende enheten KI-modeller bruker for å behandle tekst. Norsk tekst bruker flere tokens enn engelsk. Både input (din tekst) og output (Copilots svar) telles i tokens.",
  },
  {
    term: "Tool calling",
    category: "konsepter",
    definition:
      "Mekanismen der en agent velger og bruker verktøy underveis – som filoperasjoner, terminalen, MCP-servere eller websøk. Det er tool calling som gjør at agenten kan handle, ikke bare svare.",
  },
  {
    term: "Org policy",
    category: "sikkerhet",
    definition:
      "Regler på organisasjonsnivå i GitHub som styrer hvilke MCP-servere og verktøy som er tillatt. Nav bruker org policy til å begrense agenter til en godkjent liste, slik at de ikke kan koble til vilkårlige eksterne tjenester.",
  },
  {
    term: "Prompt injection",
    category: "sikkerhet",
    definition:
      "Et angrep der ondsinnet tekst i kode, dokumenter eller input manipulerer KI-agenten til å utføre handlinger den ikke skal. Risikoen øker med verktøytilgang – en agent med skrivetilgang kan gjøre mer skade enn en som bare svarer.",
  },
  {
    term: "nav-pilot",
    category: "verktoy",
    definition:
      "Navs verktøy som gir Copilot og andre KI-agenter kunnskap om Nav, for eksempel Nais-manifester og TokenX. nav-pilot installerer agentpakker og sjekker MCP-servere mot Navs MCP-register.",
    link: { href: "/nav-pilot", label: "Om nav-pilot" },
  },
  {
    term: "cplt",
    category: "verktoy",
    definition:
      "Kommandolinjeverktøy som kjører Copilot CLI, OpenCode og andre agenter i en sandkasse. Agenten kan jobbe i prosjektet ditt, men får ikke lese hemmelighetene dine.",
    link: { href: "/cplt", label: "Om cplt" },
  },
  {
    term: "Sandkasse",
    category: "sikkerhet",
    definition:
      "Et avgrenset miljø der agenten bare når filene, nettverksadressene og verktøyene du har åpnet for. Operativsystemet håndhever grensene, ikke agenten.",
    related: ["Sandbox (cplt)"],
  },
  {
    term: "Agentpakke",
    category: "verktoy",
    definition:
      "Et git-repo med agenter, skills, instructions og hooks for én type arbeid. nav-pilot installerer pakken med én kommando, og du kan ta delene du trenger eller bygge videre på den.",
    link: { href: "/nav-pilot/agentpakker", label: "Se agentpakkene" },
  },
  {
    term: "Plugin",
    category: "verktoy",
    definition:
      "En utvidelse som gir en KI-klient nye funksjoner, for eksempel hooks eller verktøy. En plugin kjører med agentens rettigheter, så bruk bare plugins du stoler på.",
  },
  {
    term: "MCP-register",
    category: "sikkerhet",
    definition:
      "Navs liste over godkjente MCP-servere. Copilot CLI henter listen fra GitHub og kjører bare servere som står der. OpenCode har ingen slik sjekk, så nav-pilot gjør den ved oppstart.",
    link: { href: "/nav-pilot/klienter#mcp-register", label: "Om MCP-registeret" },
  },
  {
    term: "AI Credits",
    category: "maling",
    definition:
      "GitHubs måleenhet for Copilot-bruk: 1 credit er 0,01 dollar. Nav får en månedlig kvote per bruker, samlet i én pott for hele organisasjonen, og modellene har ulik pris.",
    link: { href: "/priser", label: "Se priser" },
    related: ["Premium requests"],
  },
  {
    term: "BYOK (Bring Your Own Key)",
    category: "verktoy",
    definition:
      "Du kobler Copilot til en modelleverandør med din egen API-nøkkel i stedet for å bruke modellene GitHub leverer. Les retningslinjene før du tar det i bruk.",
    link: { href: "/retningslinjer", label: "Se retningslinjer" },
    related: ["Model provider"],
  },
  {
    term: "Lokal modell",
    category: "konsepter",
    definition:
      "En KI-modell som kjører på din egen maskin i stedet for i skyen, så dataene ikke forlater maskinen. nav-pilot bruker godkjente lokale modeller til noen oppgavetyper, resten blir i skyen.",
    link: { href: "/innsikt/lokale-modeller", label: "Om lokale modeller" },
  },
];
