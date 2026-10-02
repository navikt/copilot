# Revisjon av eksisterende integrasjon

Bruk denne sjekklisten når teamet vil sjekke en integrasjon, eller når en feil bare dukker opp av
og til (for eksempel etter en deploy av Dekoratøren). Inspiser koden, og rapporter hvert funn med
fil, konsekvens og konkret retting. Endre bare det teamet har bedt om, og foreslå resten.

1. **Statisk generering** – finn sider eller layouts som bygges som statisk HTML (se 3.4 i
   SKILL.md). Dette er den vanligste årsaken til at headeren eller innlogget-menyen ser feil ut
   etter en deploy av Dekoratøren, og til at `teamName` mangler.
2. **Moduler-versjon** – sjekk `@navikt/nav-dekoratoren-moduler` i `package.json` og lockfilen.
   Versjoner før 4.5 sender ikke `teamName`. Anbefal nyeste versjon. Ber teamet om oppdatering,
   bruk appens pakkebehandler, for eksempel `npm install @navikt/nav-dekoratoren-moduler@latest`,
   og sjekk endringsloggen i [releases](https://github.com/navikt/nav-dekoratoren-moduler/releases)
   før du hopper over en major-versjon.
3. **`teamName`** – ved SSR på Nais skal den settes automatisk. Hardkodet `teamName` ved SSR er
   unødvendig. Ved CSR og ved direkte `/ssr`-kall uten moduler må den settes manuelt på formen
   `app.namespace`.
4. **Miljø** – sjekk at `env` følger miljøet appen kjører i. Et fast `env: "prod"` eller en
   fallback til `prod` gjør at dev-miljøet bruker prod-dekoratøren.
5. **Feilhåndtering** – se etter `.catch` rundt `fetchDecoratorReact` eller `fetchDecoratorHtml`
   som gir tomme komponenter. Da forsvinner headeren uten at noen merker det. Feilen bør minst
   logges. Moduler-pakken prøver allerede tre ganger og faller tilbake til klient-rendering.
6. **Egen cache** – cacher appen HTML med dekoratøren i (for eksempel i minne, Redis eller et CDN),
   må cachen tømmes med `addDecoratorUpdateListener` (se 3.5 i SKILL.md).
7. **CSR eller SSR** – bruker appen `injectDecoratorClientSide` eller direkte CSR der SSR er mulig,
   anbefal SSR.
8. **Nais** – `accessPolicy.outbound` må tillate `nav-dekoratoren` i `personbruker`, eller
   eksterne hosts ved `serviceDiscovery: false` (se 2.3 i SKILL.md).
9. **CSP** – finnes en egen CSP, bør den bygges med `buildCspHeader` slik at dekoratørens
   direktiver blir med.
10. **Utfaset API** – oppgrader én major-versjon om gangen og kjør typesjekk og bygg etter hver.

    **v2 → v3 (SSR):**

    - `DECORATOR_STYLES` og `<Decorator.Styles />` er fjernet. Bruk `DECORATOR_HEAD_ASSETS` og
      `<Decorator.HeadAssets />` i `<head>`. Uten dem mangler headeren CSS og favicon.
    - `injectDecoratorServerSideDom` er fjernet. Bruk `injectDecoratorServerSideDocument`, som tar
      et vanlig `Document`.
    - `parseDecoratorHTMLToReact` er fjernet. Bruk `fetchDecoratorReact`.
    - `<EnforceLoginLoader />`, parameteren `enforceLogin`, `getUrlFromLookupTable` og
      `urlLookupTable` er fjernet. Innlogging må håndteres i appen, for eksempel med Wonderwall.
    - Avhengighetene er peer dependencies. Installer `react` og `html-react-parser` selv ved bruk
      av `fetchDecoratorReact`.
    - Egen cache av dekoratøren kan tømmes med `addDecoratorUpdateListener` (se 3.5 i SKILL.md).

    **v3 → v4 (analytics):**

    - `getAmplitudeInstance()` og `logAmplitudeEvent()` er fjernet. De har ikke logget noe siden
      v3.5. Bytt til `getAnalyticsInstance()` og `logAnalyticsEvent()` (se Steg 5 i SKILL.md).
    - Eventnavn valideres mot `@navikt/analytics-types`. Bruk `Events.*` for taksonomi-events og
      `logger.custom()` (v4.1+) for egne events.
    - Typene `AmplitudeEvent`, `AmplitudeParams` og `AnalyticsEvent` og generiske typer på
      `getAnalyticsInstance<...>()` er fjernet. Ugyldige eventnavn gir typefeil.
    - SSR- og CSR-API-ene er ellers uendret fra v3.

Oppsummer funnene sortert etter alvorlighet: først feil brukerne merker, så manglende sporbarhet
og til slutt anbefalinger.
