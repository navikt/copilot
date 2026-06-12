---
name: nav-dekoratoren
description: Integrer og konfigurer Nav Dekoratøren – felles header og footer for nav.no-applikasjoner. Bruk når et team skal ta i bruk Dekoratøren, oppdatere konfigurasjon, legge til breadcrumbs/språkvelger/analytics, håndtere samtykke (ekomloven), CSP eller feilsøke integrasjon mot dekoratøren.
license: MIT
compatibility: Web applications on nav.no (Next.js, Remix, Vite, Express/Node)
metadata:
  domain: frontend
  tags: nav dekoratoren header footer integration ssr analytics consent
---

# Nav Dekoratøren – integrasjon

Dekoratøren er felles header og footer for alle eksternt rettede nav.no-applikasjoner. Den tilbyr
SSR/CSR-integrasjon, innlogging via ID-porten, analytics (Umami), samtykke/cookie-håndtering iht.
ekomloven, og mer.

**Slack:** `#dekoratøren_på_navno`
**Repo:** https://github.com/navikt/nav-dekoratoren
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
4. **Behov** – breadcrumbs, språkvelger, analytics, chatbot, samtykke/cookies, CSP.

Spør bare om det du ikke kan finne i repoet eller som krever et produktvalg. Bruk deretter steg 2–7
og relevante referanser.

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
        params: { context: "privatperson", language: "nb" },
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
            params: { context: "privatperson", language: "nb" },
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
    params: { context: "privatperson" },
});
```

### 3.4 Cache-invalidering

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

---

## Steg 5: Klient-side funksjonalitet

Se [CLIENT-FUNCTIONS.md](references/client-functions.md) for fullstendige eksempler.

### 5.1 Breadcrumbs

Bruk `handleInApp: true` når appen selv skal håndtere navigasjonen. Bytt `navigateTo` med
rammeverkets router, for eksempel `router.push` i Next.js eller `navigate` fra React Router.

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

```ts
import { getAnalyticsInstance, Events } from "@navikt/nav-dekoratoren-moduler";

const logger = getAnalyticsInstance("min-app-origin");

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

// Sjekk om en nøkkel er tillatt
if (isStorageKeyAllowed("min-nøkkel")) {
    setNavCookie("min-nøkkel", "verdi");
}

// Bruk navLocalStorage (respekterer samtykke automatisk)
navLocalStorage.setItem("min-nøkkel", "verdi");
```

---

## Steg 7: CSP-header

```ts
import { buildCspHeader } from "@navikt/nav-dekoratoren-moduler/ssr";

const csp = await buildCspHeader(
    { "default-src": ["min-cdn.nav.no"], "style-src": ["css.nav.no"] },
    { env: "prod" },
);

res.setHeader("Content-Security-Policy", csp);
```

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

---

## Miljøer og ingresser

| Miljø      | Service host                                   | Ingress                                          |
| ---------- | ---------------------------------------------- | ------------------------------------------------ |
| `prod`     | `http://nav-dekoratoren.personbruker`          | `https://www.nav.no/dekoratoren`                 |
| `dev`      | `http://nav-dekoratoren.personbruker`          | `https://dekoratoren.ekstern.dev.nav.no`         |
| `beta`     | `http://nav-dekoratoren-beta.personbruker`     | `https://dekoratoren-beta.intern.dev.nav.no`     |
| `beta-tms` | `http://nav-dekoratoren-beta-tms.personbruker` | `https://dekoratoren-beta-tms.intern.dev.nav.no` |

> ⚠️ Beta-instanser er kun for intern testing av Team Nav.no / Team Min Side og kan være ustabile.
