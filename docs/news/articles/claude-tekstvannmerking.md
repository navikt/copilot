---
title: "Claude vannmerker teksten sin — men den sporer ikke deg"
date: 2026-08-24
author: starefossen
category: praksis
excerpt: "Anthropic merker Claude-generert tekst med et statistisk vannmerke. Det legger ingenting til i teksten, påvirker kode nesten ikke, og inneholder ingen bruker-ID. Her er mekanismen, de uavhengige kildene — og hva vi faktisk ikke kan verifisere."
tags:
  - claude
  - privacy
  - security
  - models
  - governance
---

Anthropic har begynt å merke tekst generert av Claude med et vannmerke. Flere har spurt om dette betyr at Nav-utviklere kan spores gjennom koden og teksten de får fra Claude i Copilot.

Kortversjonen: **nei.** Vannmerket inneholder ingen bruker-, konto- eller sesjons-ID. Det legger ingenting til i teksten, koster ingenting ekstra, treffer kode nesten ikke, og du trenger ikke gjøre noe. Under følger mekanismen, de uavhengige kildene som underbygger det, og det vi ikke kan verifisere.

## Hva vannmerket faktisk er

Vannmerket er statistisk og settes mens modellen genererer tekst. Når Claude skal velge neste token, finnes det ofte flere valg som er omtrent like gode. Vannmerket vrir dette valget systematisk, slik at teksten får et mønster noen med nøkkelen kan kjenne igjen statistisk.

Anthropic er eksplisitte på hva dette *ikke* innebærer: «Nothing is added to the text and there are no hidden characters.» Det er ingen usynlige Unicode-tegn, ingen metadata i teksten, ingen skjult streng. Merkingen «doesn't require extra tokens, and will not be more expensive» ([Anthropic](https://www.anthropic.com/news/claude-text-watermark)).

Anthropic beskriver løsningen som «a version of the SynthID-Text approach». Den metoden er publisert og fagfellevurdert: Dathathri m.fl., [«Scalable watermarking for identifying large language model outputs»](https://pmc.ncbi.nlm.nih.gov/articles/PMC11499265/), *Nature* 634, 818–823 (2024). Hvem som helst kan altså lese og etterprøve grunnkonstruksjonen. Du er ikke henvist til å ta leverandøren på ordet.

## Gjelder det oss i Copilot?

Anthropic sier merkingen skjer «at the model level» og dekker Claude Platform (API), Claude, Claude Code, Claude Cowork og Claude Tag — og i tillegg «when supported Claude models are accessed through AWS, Google Cloud, or Microsoft Foundry».

GitHub oppgir på sin side at Copilots modeller «are hosted by Amazon Web Services, Anthropic PBC, and Google Cloud Platform» ([GitHub Docs](https://docs.github.com/en/copilot/reference/ai-models/model-hosting)). Det er nøyaktig de kanalene Anthropic lister opp.

**Dette er en slutning, ikke et sitat.** Anthropic nevner ikke GitHub Copilot noe sted. Men når merkingen skjer på modellnivå og dekker akkurat de vertskapene Copilot bruker, er det rimelig å konkludere med at Claude-output gjennom Copilot CLI, IDE-ene og OpenCodes `github-copilot`-provider blir merket på lik linje.

**Tidspunkt, også en slutning:** modeller lansert i EU 2. august 2026 eller senere merkes fra lansering. Eldre modeller etterfylles «over the coming months». Alle Claude-modellene som i dag ligger i Copilot, ble lansert før den datoen. Sannsynligvis er merkingen derfor ikke aktiv ennå. Det finnes ingen statusside per modell, så regn med at det slås på uten at noen sier fra.

## Hva med koden min?

Lite skjer. Vannmerket krever at det finnes flere likeverdige valg å vri mellom. Anthropic sier merkingen ikke brukes «where there isn't a choice, and something would be factually wrong or a piece of code would break». Kode har «generally less watermarking», og effekten på faktisk kode beskrives som «negligible» ([Anthropic](https://support.claude.com/en/articles/16266773-how-claude-marks-ai-generated-content)).

Der det får noe utslag, er i prosa: kommentarer, docstrings, README-er, ADR-er, commit-meldinger og PR-beskrivelser. Kompilert kode, tester og API-er er upåvirket.

## «Sporer det meg?» — mekanismen og de uavhengige kildene

Dette er spørsmålet artikkelen finnes for.

### Slik ser seed-funksjonen ut

Den fagfellevurderte *Nature*-artikkelen beskriver hvordan tilfeldigheten ved hvert token bestemmes: seed er «a hash of the most recent H tokens (we use H = 4) along with the watermarking key».

Funksjonen tar altså **nøyaktig to argumenter**: de foregående tokenene i teksten, og en fast hemmelig nøkkel. Ingen bruker-ID, ingen konto, ingen sesjon og ingen samtale-ID inngår. Vannmerket har ingen payload og ingen dekoder. Deteksjon gir én tallverdi som måles mot en terskel. Utfallet er «sannsynligvis maskingenerert» eller «sannsynligvis ikke». Det er alt konstruksjonen kan produsere.

Anthropic formulerer konsekvensen selv: vannmerket «carries no identifying information and can't be traced to a specific person, organization, or chat». Du trenger ikke stole på den setningen. Den følger av algoritmen som er beskrevet i *Nature*.

### Det kan etterprøves uavhengig

Referanseimplementasjonen er åpen kildekode ([google-deepmind/synthid-text](https://github.com/google-deepmind/synthid-text)) og er tatt inn i Hugging Face Transformers som `SynthIDTextWatermarkLogitsProcessor`. [Parameterlista er dokumentert](https://huggingface.co/docs/transformers/en/internal/generation_utils): `ngram_len`, `keys`, `sampling_table_size` og noen flere. Det finnes **ingen** parameter for melding, payload eller bruker-ID. `keys` er én nøkkel per turneringslag i algoritmen, ikke én nøkkel per bruker.

### Det ærlige forbeholdet: sporende vannmerker finnes faktisk

Her skal vi ikke overselge. Det er fullt mulig å lage vannmerker som bærer informasjon om hvem som genererte teksten, og det er aktiv forskning på det:

- Yoo m.fl., NAACL 2024: [«Advancing Beyond Identification: Multi-bit Watermark for Large Language Models»](https://arxiv.org/abs/2308.00221) — meldinger på 32 bit og oppover.
- Qu m.fl., USENIX Security '25: [«Provably Robust Multi-bit Watermarking for AI-generated Text»](https://arxiv.org/abs/2401.16820) — beskriver eksplisitt «embedding the user ID … we can trace generated texts to the user», med 20 bit i 200 tokens og 97,6 % treffrate.
- Jiang m.fl., ICML 2025: [«StealthInk: A Multi-bit and Stealthy Watermark for Large Language Models»](https://arxiv.org/abs/2506.05502) — koder blant annet «userID, TimeStamp, and modelID».

Så påstanden «denne teknologiklassen kan aldri identifisere noen» er feil, og vi skal ikke fremme den.

Den riktige påstanden er snevrere og sterkere: dette er **multi-bit**-vannmerking, en strukturelt annen konstruksjon enn den SynthID-Text bruker. SynthID-Text er **zero-bit** — den koder ingen melding, bare tilstedeværelse. Å gjøre om et zero-bit-vannmerke til et sporende multi-bit-vannmerke er ikke en konfigurasjonsendring, men en annen algoritme.

Og det koster: de multi-bit-artiklene vi har gjennomgått, rapporterer at deteksjonspåliteligheten går ned når vannmerket i tillegg skal bære en melding. [Three Bricks](https://arxiv.org/abs/2308.00113) sier det rett ut: «by giving the possibility to encode several messages, we trade some accuracy of detection against the ability to identify users». Zero-bit-linja er godt etablert i litteraturen — se [Kirchenbauer m.fl., ICML 2023](https://arxiv.org/abs/2301.10226), [Kuditipudi m.fl.](https://arxiv.org/abs/2307.15593) og [Christ, Gunn og Zamir](https://arxiv.org/abs/2306.09194).

### Hva kritikken faktisk handler om

Et nyttig signal: forskningsmiljøet som er skeptisk til LLM-vannmerking, er opptatt av noe helt annet enn sporing. Kritikken dreier seg om robusthet og fjerning ([«Watermarks in the Sand»](https://arxiv.org/abs/2311.04378), [«Can AI-Generated Text be Reliably Detected?»](https://arxiv.org/abs/2303.11156)), om forfalskning ([«Watermark Stealing in Large Language Models»](https://arxiv.org/abs/2402.19361)) og om falske positiver.

Ingen av disse handler om at vannmerket identifiserer brukeren. At hovedkritikken peker et helt annet sted, er i seg selv et argument. Vi har heller ikke funnet noen dokumenterte tilfeller (verken reelle eller demonstrerte) der et LLM-vannmerke har identifisert en bruker.

### En vanlig sammenblanding, relevant for norske utviklere

Det er godt kjent at AI-detektorer slår ut skjevt mot folk som skriver engelsk som andrespråk ([Liang m.fl.](https://arxiv.org/abs/2304.02819)). Det funnet siteres ofte i vannmerkediskusjoner, og det er feil bruk: studien handler om **post-hoc-klassifikatorer** som gjetter ut fra stil, ikke om vannmerker.

En nøkkelbasert statistisk test har en falsk positiv-rate som kan regnes ut på forhånd og som ikke avhenger av forfatterens morsmål. Testen gir «interpretable p-values» ([Kirchenbauer m.fl.](https://arxiv.org/abs/2301.10226)). For et norsk fagmiljø som skriver mye dokumentasjon på engelsk, er dette en reell forskjell.

### Hva vi ikke kan verifisere

To ting, sagt rett ut:

1. **Nøkkelen er hemmelig.** Det er selve poenget med konstruksjonen, men det betyr at ingen utenforstående kan revidere Anthropics konkrete produksjonsoppsett. Vi kan verifisere at *SynthID-Text-metoden* er zero-bit. Vi kan ikke inspisere det som faktisk kjører hos Anthropic.
2. **Anthropic sier «a version of» SynthID-Text.** Hva som ligger i «a version of» er ikke spesifisert i detalj.

Det er grensene for hva som kan etterprøves.

### «Hva hindrer dem i å bytte til et sporende vannmerke i det stille?»

Det er det egentlige spørsmålet, og det fortjener et ordentlig svar. Tre ting står i veien:

1. **Det er en annen algoritme, ikke en innstilling.** Å gå fra zero-bit til multi-bit betyr å bygge om konstruksjonen: du må ha en melding å kode, kanalkoding som tåler redigering, og en dekoder i deteksjonen. Ingen av delene finnes i SynthID-Text. Det er ikke et flagg noen skrur på.
2. **Det koster deteksjonskvalitet.** Se sitatet fra Three Bricks over — kapasiteten til å identifisere betales med presisjon i deteksjonen. Anthropics uttalte formål er å kjenne igjen Claude-tekst. Et sporende vannmerke ville gjort produktet dårligere til nettopp det de bygde det for.
3. **Det ville stått i direkte motstrid til publiserte påstander.** «Carries no identifying information» er en konkret påstand fra et selskap som selv varsler et deteksjons-API — altså en påstand andre etter hvert får anledning til å teste.

Sannsynligheten er derfor lav. Men nøkkelen er fortsatt hemmelig, og «lav» er ikke «utelukket». Det skal ikke fremstilles som noe annet.

## Hva vannmerket ikke er

- **Ikke en AI-detektor.** Anthropic er tydelig på at det «cannot distinguish 'Claude wrote this' from 'Claude heavily edited this'».
- **Ikke bevis på menneskelig forfatterskap ved fravær.** Fravær av merking betyr ikke at teksten er skrevet av et menneske.
- **Ikke skjulte tegn.** Ingen usynlig Unicode, ingen metadata i teksten.
- **Ikke lisens, attribusjon eller eierskap.** Det sier ingenting om rettigheter til teksten.
- **Ikke tregere eller dyrere.** Ingen ekstra tokens.

**Og ikke det samme som C2PA.** C2PA er provenance-metadata som legges på *filer* (Anthropic navngir `.svg`, `.png` og `.jpg`), ikke på kildekode eller `.md`. Anthropic beskriver selv C2PA som «very different from a watermark». C2PA er faktisk personvernsensitivt, fordi slike metadata kan bære GPS-, enhets- og skaperinformasjon. Tekstvannmerket er det ikke. Skill mellom de to i enhver diskusjon.

## Deteksjon

Det finnes ingen offentlig detektor i dag. Bare de som har nøkkelen, kan kontrollere teksten. Anthropic sier et deteksjons-API kommer, uten nærmere detaljer.

Vannmerket overlever kopiering og liming. Anthropic skriver at «light editing probably won't remove the watermark completely», mens en full omskriving der hvert ord byttes ut, fjerner det.

## Jussen, kort og adskilt

Dette avsnittet er juridisk kontekst, ikke teknisk vurdering — hold delene fra hverandre.

**Nav er deployer, ikke provider.** EU AI Act artikkel 50(2), som pålegger maskinlesbar merking av AI-generert innhold, retter seg mot *tilbyderne* — Anthropic og GitHub. Navs plikt som deployer etter artikkel 50(4) er snever: den gjelder deepfakes og AI-generert tekst som «publiseres med formål å informere allmennheten om saker av allmenn interesse», og den slår ikke inn når et menneske har gått gjennom innholdet og noen har redaksjonelt ansvar ([artikkel 50](https://artificialintelligenceact.eu/article/50/)). Kode og intern dokumentasjon faller utenfor.

**I Norge gjelder ikke artikkel 50 ennå.** AI Act er verken innlemmet i EØS-avtalen eller gjennomført i norsk rett, og artikkel 50 «gjelder derfor foreløpig ikke generelt i Norge» ([Nkom](https://nkom.no/ki/regulering/nye-krav-til-ki-merking-i-eu--hva-betyr-dette-for-norge)). Ett unntak: norske virksomheter kan likevel bli omfattet «dersom de tilbyr KI-systemer i EU, eller dersom resultatet fra et KI-system brukes i EU» — det treffer ikke intern utvikling. Regjeringen tar sikte på å fremme en norsk KI-lov for Stortinget våren 2027. GDPR gjelder uansett.

**GDPR:** et zero-bit-vannmerke koder en egenskap ved modellen, ikke ved en person. Det er ikke en identifikator, og dermed ikke personopplysninger. Et multi-bit-vannmerke per bruker *ville* vært pseudonyme personopplysninger — nok en grunn til at skillet betyr noe. Det britiske ICO behandler spørsmålet betinget i Tech Horizons Report 2025: personopplysninger oppstår hvis identiteten til den som skapte innholdet registreres, eller hvis lokasjonsdata registreres. Ingen EU/EØS-tilsynsmyndighet har publisert veiledning spesifikt om tekstvannmerking.

**Kommisjonens egen retning peker samme vei.** Europakommisjonens retningslinjer til artikkel 50 (C(2026) 5054, 20. juli 2026, punkt 94) sier at plikten «focuses on how the content has been created and its artificial origin, not on who created the content», og at det ved merking og deteksjon «should not be processed» informasjon om den som skapte innholdet. Innholdet er godkjent; formell vedtakelse avventer oversettelser.

## Hva Nav-team bør gjøre

**For vannmerket: ingenting.** Ingen konfigurasjon, ingen policy, ingen tiltak. Det påvirker ikke kode, kostnad eller ytelse.

To ting bør du likevel kjenne til:

**1. Et deteksjons-API er personvernspørsmålet — ikke vannmerket.** Fordi deteksjon krever leverandørens nøkkel, betyr det å sjekke et dokument i praksis at dokumentet **sendes til leverandøren**. EFF har pekt på dette som den reelle svakheten ved vannmerkeordninger ([EFF](https://www.eff.org/deeplinks/2024/01/ai-watermarking-wont-curb-disinformation)). Skulle Nav noen gang ta i bruk en slik tjeneste på interne eller innbyggerrettede dokumenter, hører det hjemme i en DPIA. Dette er stikk motsatt av frykten folk har: risikoen ligger i å *bruke* detektoren, ikke i å bli merket.

**2. Et deteksjonstreff er ikke bevis på forfatterskap.** Et positivt utslag sier «denne teksten er sannsynligvis generert av en Claude-modell» — ikke hvem som ba om den, ikke hvor mye et menneske har bearbeidet den, og ikke om noe kritikkverdig har skjedd. Det må aldri brukes som bevis i personalsaker eller i saksbehandling.

**En helt annen sak, som er større:** GitHubs avtaler om null datalagring dekker Claude-modellene i Copilot — med unntak av Claude Fable 5, der Anthropic beholder prompts og output for å kjøre sikkerhetsklassifikatorer. Det er et vesentlig større styringsspørsmål enn vannmerket, og et annet tema. Sjekk Navs [retningslinjer](/retningslinjer) for hvilke Copilot-modeller som er godkjent.

## Kortversjonen

**Vannmerket inneholder ingen bruker-ID. Seed-funksjonen tar bare de foregående tokenene og en fast nøkkel — det står i den fagfellevurderte artikkelen, ikke bare i leverandørens markedsføring. Det er zero-bit: det kan svare «merket» eller «ikke merket», ingenting mer. Sporende multi-bit-vannmerker finnes i forskningen, men det er en annen algoritme. Nøkkelen er hemmelig, så Anthropics konkrete oppsett kan ikke revideres utenfra — det er grensen for hva vi kan verifisere. Kode påvirkes nesten ikke. Du trenger ikke gjøre noe.**

**Kilder:**

- [A new way to identify Claude's writing](https://www.anthropic.com/news/claude-text-watermark) (Anthropic)
- [How Claude marks AI-generated content](https://support.claude.com/en/articles/16266773-how-claude-marks-ai-generated-content) (Anthropic Support)
- Dathathri m.fl., [Scalable watermarking for identifying large language model outputs](https://pmc.ncbi.nlm.nih.gov/articles/PMC11499265/), *Nature* 634, 818–823 (2024), [DOI 10.1038/s41586-024-08025-4](https://doi.org/10.1038/s41586-024-08025-4)
- [google-deepmind/synthid-text](https://github.com/google-deepmind/synthid-text) (referanseimplementasjon, åpen kildekode)
- [AI model hosting for GitHub Copilot](https://docs.github.com/en/copilot/reference/ai-models/model-hosting) (GitHub Docs)
- [A Watermark for Large Language Models](https://arxiv.org/abs/2301.10226) (Kirchenbauer m.fl., ICML 2023)
- [Robust Distortion-free Watermarks for Language Models](https://arxiv.org/abs/2307.15593) (Kuditipudi m.fl.)
- [Undetectable Watermarks for Language Models](https://arxiv.org/abs/2306.09194) (Christ, Gunn og Zamir)
- [Three Bricks to Consolidate Watermarks for Large Language Models](https://arxiv.org/abs/2308.00113) (Fernandez m.fl., WIFS 2023)
- [Advancing Beyond Identification: Multi-bit Watermark for Large Language Models](https://arxiv.org/abs/2308.00221) (Yoo m.fl., NAACL 2024)
- [Provably Robust Multi-bit Watermarking for AI-generated Text](https://arxiv.org/abs/2401.16820) (Qu m.fl., USENIX Security '25)
- [StealthInk: A Multi-bit and Stealthy Watermark for Large Language Models](https://arxiv.org/abs/2506.05502) (Jiang m.fl., [ICML 2025](https://icml.cc/virtual/2025/poster/44621))
- [Watermarks in the Sand: Impossibility of Strong Watermarking for Generative Models](https://arxiv.org/abs/2311.04378)
- [Can AI-Generated Text be Reliably Detected?](https://arxiv.org/abs/2303.11156)
- [Watermark Stealing in Large Language Models](https://arxiv.org/abs/2402.19361)
- [GPT detectors are biased against non-native English writers](https://arxiv.org/abs/2304.02819) (Liang m.fl.)
- [AI Watermarking Won't Curb Disinformation](https://www.eff.org/deeplinks/2024/01/ai-watermarking-wont-curb-disinformation) (EFF, januar 2024)
- [EU AI Act artikkel 50](https://artificialintelligenceact.eu/article/50/)
- Europakommisjonens retningslinjer til artikkel 50, C(2026) 5054, 20. juli 2026 (innhold godkjent, formell vedtakelse avventer oversettelser)
- [Nye krav til KI-merking i EU — hva betyr dette for Norge?](https://nkom.no/ki/regulering/nye-krav-til-ki-merking-i-eu--hva-betyr-dette-for-norge) (Nkom)
