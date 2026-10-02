---
name: nav-dekoratoren
description: Integrer, konfigurer og revider Nav Dekoratøren – felles header og footer for nav.no-applikasjoner. Bruk når et team skal ta i bruk Dekoratøren, sjekke en eksisterende integrasjon, oppdatere konfigurasjon, legge til breadcrumbs/språkvelger/analytics, håndtere samtykke (ekomloven), CSP eller feilsøke integrasjon mot dekoratøren.
license: MIT
compatibility: Web applications on nav.no (Next.js, Remix, Vite, Express/Node)
metadata:
  domain: frontend
  tags: nav dekoratoren header footer integration ssr analytics consent
---

# Nav Dekoratøren – integrasjon

Dekoratøren er felles header og footer for eksternt rettede nav.no-applikasjoner. Den tilbyr
SSR/CSR-integrasjon, innloggingsknapper via ID-porten, analytics (Umami) og håndtering av samtykke
og cookies etter ekomloven. Dekoratøren gir ikke appen tilgang til brukerens innlogging. Sett opp
autentisering og tilgangskontroll i appen selv.

**Slack:** `#dekoratøren_på_navno`
**Repo:** https://github.com/navikt/nav-dekoratoren
**Dokumentasjon:** https://github.com/navikt/nav-dekoratoren/blob/main/README.md
**Moduler-pakke:** https://github.com/navikt/nav-dekoratoren-moduler
**Storybook:** https://navikt.github.io/nav-dekoratoren

---

## Steg 1: Kartlegg hva teamet trenger

Start med å inspisere repoet hvis du har tilgang til koden. Se etter:

1. **Formål** – bruker appen allerede Dekoratøren, eller skal den integreres for første gang?
2. **Rammeverk** – Next.js `app/` (App Router), Next.js `pages/` (Page Router), Remix / React Router
   framework mode, Vite SPA / React Router library mode, eller ren Node/Express.
3. **Miljø** – `prod`, `dev`, `localhost`, eksisterende `nais.yaml`, og om service discovery kan
   brukes.
4. **Behov** – breadcrumbs, språkvelger, analytics, chatbot, samtykke/cookies, CSP og skip-lenke.

Spør bare om det du ikke kan finne i repoet eller som krever et produktvalg. Bruk deretter steg 2–7
og relevante referanser. Bruker appen allerede Dekoratøren, og teamet vil sjekke oppsettet eller
feilsøke, gå til [Revisjon av eksisterende integrasjon](references/audit.md).

---

## Steg 2: Installasjon og oppsett

### 2.1 Installer moduler-pakken

```bash
npm install --save @navikt/nav-dekoratoren-moduler
```

Pakken publiseres kun på **GitHub Packages**. Legg til i `.npmrc`:

```text
@navikt:registry=https://npm.pkg.github.com
```

Logg inn med en PAT med `read:packages`-scope og navikt SSO-autorisasjon:

```bash
npm login --registry=https://npm.pkg.github.com --auth-type=legacy
```

### 2.2 GitHub Actions

```yaml
- uses: actions/setup-node@v4
  with:
      registry-url: "https://npm.pkg.github.com"

- run: npm ci
  env:
      NODE_AUTH_TOKEN: ${{ secrets.READER_TOKEN }}
```

### 2.3 Access policy i nais.yaml

Ved service discovery (anbefalt – brukes automatisk på dev-gcp/prod-gcp):

```yaml
accessPolicy:
    outbound:
        rules:
            - application: nav-dekoratoren
              namespace: personbruker
```

Ved eksterne ingresser (hvis `serviceDiscovery: false`):

```yaml
accessPolicy:
    outbound:
        external:
            - host: www.nav.no # prod
            - host: dekoratoren.ekstern.dev.nav.no # dev
```

---

## Steg 3: SSR-integrasjon (anbefalt)

Server-side rendering gir best ytelse og unngår layout shift.
Se [SSR-FUNCTIONS.md](references/ssr-functions.md) for fullstendige API-detaljer.
Moduler-pakken prøver å hente dekoratøren tre ganger før den viser statiske plassholdere som
rendres på klienten. Ved SSR på Nais setter pakken `teamName` automatisk til
`NAIS_APP_NAME.NAIS_NAMESPACE` for konsumentlogging. Variablene leses når dekoratøren hentes, så
de finnes bare i runtime på Nais, ikke i byggesteget. Uten dem varsler pakken én gang og bruker en
eventuell manuelt satt `params.teamName`. Se
[konsumentlogging](https://github.com/navikt/nav-dekoratoren/blob/main/README.md#10-innebygde-funksjoner-i-dekorat%C3%B8ren)
ved direkte SSR-kall eller CSR.

### 3.1 Next.js App Router

Bruk dette for nye Next.js-apper og apper som allerede har `app/`. Root layout kan være async og
hente dekoratøren direkte.

```tsx
// app/layout.tsx
import { fetchDecoratorReact } from "@navikt/nav-dekoratoren-moduler/ssr";
import type { ReactNode } from "react";
import Script from "next/script";

export default async function RootLayout({
    children,
}: {
    children: ReactNode;
}) {
    const Decorator = await fetchDecoratorReact({
        env: "prod",
        params: {
            context: "privatperson",
            language: "nb",
            origin: "min-app",
        },
    });

    return (
        <html lang="nb">
            <head>
                <Decorator.HeadAssets />
            </head>
            <body>
                <Decorator.Header />
                {children}
                <Decorator.Footer />
                <Decorator.Scripts loader={Script} />
            </body>
        </html>
    );
}
```

### 3.2 Next.js Page Router

Bruk dette bare for eksisterende Next.js-apper med `pages/`. I Page Router må dekoratøren hentes i
`pages/_document.tsx`, fordi `_document` eier `<html>`, `<head>` og den server-renderede
HTML-shellen.

```tsx
// pages/_document.tsx
import {
    fetchDecoratorReact,
    type DecoratorComponentsReact,
} from "@navikt/nav-dekoratoren-moduler/ssr";
import Document, {
    Head,
    Html,
    Main,
    NextScript,
    type DocumentContext,
    type DocumentInitialProps,
} from "next/document";

type MyDocumentProps = DocumentInitialProps & {
    Decorator: DecoratorComponentsReact;
};

class MyDocument extends Document<MyDocumentProps> {
    static async getInitialProps(
        ctx: DocumentContext,
    ): Promise<MyDocumentProps> {
        const initialProps = await Document.getInitialProps(ctx);
        const Decorator = await fetchDecoratorReact({
            env: "prod",
            params: {
                context: "privatperson",
                language: "nb",
                origin: "min-app",
            },
        });

        return { ...initialProps, Decorator };
    }

    render() {
        const { Decorator } = this.props;
        return (
            <Html lang="nb">
                <Head>
                    <Decorator.HeadAssets />
                </Head>
                <body>
                    <Decorator.Header />
                    <Main />
                    <Decorator.Footer />
                    <Decorator.Scripts />
                    <NextScript />
                </body>
            </Html>
        );
    }
}

export default MyDocument;
```

### 3.3 Express/Node (HTML-fragmenter)

```ts
import { fetchDecoratorHtml } from "@navikt/nav-dekoratoren-moduler/ssr";

const {
    DECORATOR_HEAD_ASSETS,
    DECORATOR_HEADER,
    DECORATOR_FOOTER,
    DECORATOR_SCRIPTS,
} = await fetchDecoratorHtml({
    env: "dev",
    params: { context: "privatperson", origin: "min-app" },
});
```

Ved direkte kall uten moduler-pakken: send `teamName` som query-parameter på `/ssr`, for eksempel
`https://www.nav.no/dekoratoren/ssr?teamName=team-navno.navno`. Med service discovery er adressen
`http://nav-dekoratoren.personbruker/ssr?teamName=team-navno.navno`, uten `/dekoratoren` foran
`/ssr`. `teamName` må være på formen `teamnavn.namespace`: små bokstaver, minst ett punktum og
bare `a-z`, `0-9`, `-` og `.`. `origin` brukes til analytics og erstatter ikke `teamName`.

### 3.4 Unngå statisk generering av sider med dekoratøren

Dekoratøren må hentes i runtime, per request eller fra moduler-pakkens cache. Hvis Next.js bygger
siden som statisk HTML, fryses dekoratørens HTML, CSS og versjons-ID fra byggetidspunktet. Etter
neste deploy av Dekoratøren blander siden gammel CSS med ny HTML fra blant annet `/auth`, og for
eksempel innlogget-menyen kan se feil ut. En ny bygg av appen retter det midlertidig.
`teamName` mangler også: moduler-pakken leser `NAIS_APP_NAME` og `NAIS_NAMESPACE` når
dekoratøren hentes, og de finnes bare i poden på Nais, ikke i byggesteget (for eksempel GitHub
Actions eller Docker build). Når siden rendres per request, for eksempel med `getServerSideProps`,
kjøres `_document` i poden, og `teamName` settes automatisk.

Når du inspiserer et repo, se etter dette:

- **Page Router:** `_document` kjøres også for statisk genererte sider. Det gjelder sider med
  `getStaticProps` og sider uten datahenting (Automatic Static Optimization). Bytt
  `getStaticProps` med `getServerSideProps`. Sider uten datahenting kan gjøres dynamiske med
  `getServerSideProps` hver for seg, eller for hele appen med `getInitialProps` i
  `pages/_app.tsx`. Det siste gjelder ikke sider med `getStaticProps`. `next build` markerer statiske sider med `○` eller `●` og
  dynamiske med `ƒ`.
- **App Router:** ruter uten dynamiske API-er forhåndsrendres i byggesteget. Gjør root layout
  dynamisk med `export const dynamic = "force-dynamic";`, eller kall `await connection()` fra
  `next/server` før `fetchDecoratorReact`.

```ts
// pages/minside.tsx (Page Router)
export async function getServerSideProps({ locale }: GetServerSidePropsContext) {
    return { props: { ...(await serverSideTranslations(locale ?? "nb", ["common"])) } };
}
```

Kan appen ikke gjøres dynamisk ennå, sett `teamName` manuelt i `params` (for eksempel
`teamName: "min-app.mitt-namespace"`) slik at forespørslene i det minste kan spores. Det retter
ikke versjonsproblemet.

### 3.5 Cache-invalidering

```ts
import { addDecoratorUpdateListener } from "@navikt/nav-dekoratoren-moduler/ssr";

addDecoratorUpdateListener({ env: "prod" }, (versionId) => {
    console.log(`Ny dekoratørversjon: ${versionId} – tømmer cache`);
    myHtmlCache.clear();
});
```

---

## Steg 4: Konfigurasjon

Se [PARAMS.md](references/params.md) for alle parametre med type og standardverdi.

De viktigste:

| Parameter            | Type                                                  | Default        |
| -------------------- | ----------------------------------------------------- | -------------- |
| `context`            | `privatperson` / `arbeidsgiver` / `samarbeidspartner` | `privatperson` |
| `language`           | `nb` / `nn` / `en` / `se` / `pl` / `uk` / `ru`        | `nb`           |
| `breadcrumbs`        | `{ title, url, handleInApp? }[]`                      | `[]`           |
| `availableLanguages` | `{ locale, url, handleInApp? }[]`                     | `[]`           |
| `simple`             | `boolean`                                             | `false`        |
| `chatbot`            | `boolean`                                             | `true`         |
| `redirectToApp`      | `boolean`                                             | `false`        |
| `logoutWarning`      | `boolean`                                             | `true`         |
| `feedback`           | `boolean`                                             | `false`        |
| `origin`             | `string`                                              | `undefined`    |

`language` kan settes til alle språkene i tabellen, men tekst i Dekoratørens eget grensesnitt
finnes bare på bokmål, engelsk og delvis samisk. URL-er med `/no/`, `/nb/`, `/nn/`, `/en/` eller
`/se/` kan overstyre `language`. URL-ene i `breadcrumbs` og `availableLanguages` må ligge på
`nav.no` eller et underdomene; ellers svarer Dekoratøren med 500.

Bruk `redirectToUrl` eller `redirectToUrlLogout` for returadresse etter henholdsvis innlogging
og utlogging. Begge avviser URL-er utenfor `nav.no` og underdomener. Ikke forveksle dem med
`logoutUrl`: den overlater *hele* utloggingen, inkludert sletting av cookies og økter, til appen.
Hvis du slår av `logoutWarning`, må appen selv gi brukeren mulighet til å utsette utlogging.
Se [alle parametrene](references/params.md).

---

## Steg 5: Klient-side funksjonalitet

Se [CLIENT-FUNCTIONS.md](references/client-functions.md) for fullstendige eksempler.

### 5.1 Breadcrumbs

Bruk `handleInApp: true` når appen selv skal håndtere navigasjonen. Bytt `navigateTo` med
rammeverkets router, for eksempel `router.push` i Next.js eller `navigate` fra React Router.
`title` logges som `[redacted]` til Umami. Sett `analyticsTitle` bare til tekst uten
personopplysninger hvis du vil logge en tittel.

```ts
import {
    setBreadcrumbs,
    onBreadcrumbClick,
} from "@navikt/nav-dekoratoren-moduler";

setBreadcrumbs([
    { title: "Ditt Nav", url: "https://www.nav.no/person/dittnav" },
    {
        title: "Kontakt oss",
        url: "https://www.nav.no/person/kontakt-oss",
        handleInApp: true,
    },
]);

onBreadcrumbClick((breadcrumb) => {
    navigateTo(breadcrumb.url);
});
```

### 5.2 Språkvelger

Samme mønster gjelder språkvalg med `handleInApp: true`.

```ts
import {
    setAvailableLanguages,
    onLanguageSelect,
} from "@navikt/nav-dekoratoren-moduler";

setAvailableLanguages([
    { locale: "nb", url: "https://www.nav.no/min-side/nb" },
    { locale: "en", url: "https://www.nav.no/min-side/en", handleInApp: true },
]);

onLanguageSelect((language) => {
    navigateTo(language.url);
});
```

### 5.3 Analytics (Umami)

Send appens tekniske navn som `origin` i dekoratørparameterne. Verdien legges til automatiske
`besøk`-hendelser, slik at sidevisninger kan filtreres per app. Når parameteren utelates, bruker
`besøk`-hendelser verdien `nav-dekoratoren`.
Dekoratøren sender ikke Umami-hendelser uten samtykke. Loggeren tar likevel imot kallene og
forkaster dem lokalt. Query-parametre fjernes fra sidevisninger som standard; bruk
`analyticsQueryParams` bare for navn der verdiene ikke kan inneholde personopplysninger.
Endring av `analyticsRedactFilter` krever en egen risikovurdering.

```ts
import { getAnalyticsInstance, Events } from "@navikt/nav-dekoratoren-moduler";

const logger = getAnalyticsInstance("min-app");

// Taksonomi-event (strengt typet fra @navikt/analytics-types)
logger(Events.SKJEMA_STARTET, { skjemaId: "1234", skjemanavn: "aap" });

// Custom event
logger.custom("feedback åpnet", { komponent: "feedback-widget", steg: 2 });
```

> ⚠️ `getAmplitudeInstance()` er fjernet i v4+. Bruk `getAnalyticsInstance()`.

### 5.4 Chatbot

```ts
import { openChatbot } from "@navikt/nav-dekoratoren-moduler";

openChatbot(); // åpner Frida og setter chatbotVisible=true
```

---

## Steg 6: Samtykke og cookies (ekomloven)

Se [CONSENT.md](references/consent.md) for detaljer.
Nøkler må stå på tillatt-listen. Samtykke alene gjør ikke en ukjent nøkkel tillatt.

```ts
import {
    awaitDecoratorData,
    isStorageKeyAllowed,
    setNavCookie,
    getNavCookie,
    navLocalStorage,
} from "@navikt/nav-dekoratoren-moduler";

// Vent til dekoratøren har lastet samtykke
await awaitDecoratorData();

// Erstatt med en nøkkel som er registrert som cookie på tillatt-listen
if (isStorageKeyAllowed("registrert-cookie")) {
    setNavCookie("registrert-cookie", "verdi");
}

// Bruk en nøkkel registrert for localStorage
navLocalStorage.setItem("registrert-localstorage-nøkkel", "verdi");
```

---

## Steg 7: CSP-header

Bruk `buildCspHeader` for å kombinere appens CSP med direktivene Dekoratøren trenger.
Ved direkte integrasjon må appen selv holde CSP oppdatert mot
[`/dekoratoren/api/csp`](https://www.nav.no/dekoratoren/api/csp).

```ts
import { buildCspHeader } from "@navikt/nav-dekoratoren-moduler/ssr";

const csp = await buildCspHeader(
    { "default-src": ["min-cdn.nav.no"], "style-src": ["css.nav.no"] },
    { env: "prod" },
);

res.setHeader("Content-Security-Policy", csp);
```

## Skip-lenke

Dekoratøren viser en lenke til hovedinnholdet når dokumentet har et element med
`id="maincontent"`. Gjør elementet fokuserbart slik at tastaturfokus flyttes dit:

```html
<main id="maincontent" tabindex="-1">Appens innhold</main>
```

---

## Revisjon av eksisterende integrasjon

Når teamet vil sjekke en eksisterende integrasjon, eller en feil bare dukker opp av og til (for
eksempel etter en deploy av Dekoratøren), følg sjekklisten i [AUDIT.md](references/audit.md).
Start med statisk generering (3.4), som er den vanligste årsaken.

---

## Vanlige feil og løsninger

| Problem                                  | Årsak                     | Løsning                                                    |
| ---------------------------------------- | ------------------------- | ---------------------------------------------------------- |
| `403` ved npm install                    | Mangler PAT eller SSO     | Generer PAT med `read:packages`, aktiver navikt SSO        |
| Dekoratøren vises ikke                   | Mangler access policy     | Legg til `nav-dekoratoren` i `accessPolicy.outbound.rules` |
| Layout shift                             | CSR brukes                | Bytt til SSR via `fetchDecoratorReact`                     |
| Cookie ikke satt                         | Samtykke ikke innhentet   | Bruk `awaitDecoratorData()` + `setNavCookie`               |
| `getAmplitudeInstance is not a function` | Moduler v4+ (API endret)   | Oppgrader til v4+ og bytt til `getAnalyticsInstance`       |
| `availableLanguages`-URL feil            | URL utenfor nav.no        | Kun `nav.no` og underdomener er tillatt                    |
| Konsument vises som `unknown`              | Mangler `teamName`        | Sett `teamName` ved direkte SSR-kall og CSR med moduler    |
| `unknown` selv med SSR og moduler 4.5+     | Siden er bygget statisk   | Gjør siden dynamisk, se 3.4                                |
| Header/innlogget-meny feil etter dekoratør-deploy, retter seg ved redeploy | Siden er bygget statisk | Gjør siden dynamisk, se 3.4 |

---

## Miljøer og ingresser

| Miljø      | Service host                                   | Ingress                                          |
| ---------- | ---------------------------------------------- | ------------------------------------------------ |
| `prod`     | `http://nav-dekoratoren.personbruker`          | `https://www.nav.no/dekoratoren`                 |
| `dev`      | `http://nav-dekoratoren.personbruker`          | `https://dekoratoren.ekstern.dev.nav.no`         |
| `beta`     | `http://nav-dekoratoren-beta.personbruker`     | `https://dekoratoren-beta.intern.dev.nav.no`     |
| `beta-tms` | `http://nav-dekoratoren-beta-tms.personbruker` | `https://dekoratoren-beta-tms.intern.dev.nav.no` |

> ⚠️ Beta-instanser er kun for intern testing av Team Nav.no / Team Min Side og kan være ustabile.
