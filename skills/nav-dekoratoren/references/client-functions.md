# Klient-side funksjoner i @navikt/nav-dekoratoren-moduler

Importeres fra `@navikt/nav-dekoratoren-moduler` (uten `/ssr`-suffix).

## setBreadcrumbs

Setter brødsmulestien dynamisk. Bruk `handleInApp: true` for SPA-routing.
⚠️ `title` logges som `[redacted]` til Umami som standard. Bruk `analyticsTitle` for å logge uten
personopplysninger.

```ts
import { setBreadcrumbs } from "@navikt/nav-dekoratoren-moduler";

setBreadcrumbs([
    { title: "Ditt Nav", url: "https://www.nav.no/person/dittnav" },
    {
        title: "Opplysninger for Ola Nordmann",
        analyticsTitle: "Opplysninger for <Navn>", // ingen personopplysninger
        url: "https://www.nav.no/min-side",
        handleInApp: true,
    },
]);
```

## onBreadcrumbClick

Kalles når bruker klikker på breadcrumb med `handleInApp: true`.
Bruk rammeverkets router: `router.push(url)` i Next.js, `navigate(url)` i React Router, eller
tilsvarende i andre SPA-rammeverk.

```ts
import { onBreadcrumbClick } from "@navikt/nav-dekoratoren-moduler";

onBreadcrumbClick((breadcrumb) => {
    navigateTo(breadcrumb.url);
});
```

## setAvailableLanguages

Oppdaterer språkvelgeren. URL må være på `nav.no` eller underdomene.

```ts
import { setAvailableLanguages } from "@navikt/nav-dekoratoren-moduler";

setAvailableLanguages([
    { locale: "nb", url: "https://www.nav.no/kontakt-oss/nb" },
    {
        locale: "en",
        url: "https://www.nav.no/kontakt-oss/en",
        handleInApp: true,
    },
]);
```

## onLanguageSelect

Kalles ved språkvalg med `handleInApp: true`.
Bruk samme router-funksjon som for breadcrumbs.

```ts
import { onLanguageSelect } from "@navikt/nav-dekoratoren-moduler";

onLanguageSelect((language) => {
    navigateTo(language.url);
});
```

## getAnalyticsInstance

Henter logger-instans for analytics (Umami). Støtter taksonomi-events og custom events.

```ts
import {
    getAnalyticsInstance,
    Events,
    isValidEventName,
} from "@navikt/nav-dekoratoren-moduler";

const logger = getAnalyticsInstance("min-app-origin");

// Taksonomi-event – strengt typet fra @navikt/analytics-types
logger(Events.SKJEMA_STARTET, { skjemaId: "1234", skjemanavn: "aap" });

// Custom event
logger.custom("feedback åpnet", { komponent: "feedback-widget", steg: 2 });

// Dynamisk valg av event-type
if (isValidEventName(eventName)) {
    logger(eventName, eventData);
} else {
    logger.custom(eventName, eventData);
}
```

Importer event-typer direkte:

```ts
import type {
    NavigereEvent,
    SkjemaStartetEvent,
} from "@navikt/nav-dekoratoren-moduler";
```

> ⚠️ `getAmplitudeInstance()` er fjernet i v4+. Bruk `getAnalyticsInstance()`.

## setParams / getParams

Oppdater eller les alle parametre dynamisk.

```ts
import { setParams, getParams } from "@navikt/nav-dekoratoren-moduler";

// Oppdater parametre
setParams({ simple: true, chatbot: false });

// Les gjeldende parametre
const current = getParams();
```

## openChatbot

Åpner Chatbot Frida og setter `chatbotVisible=true`.

```ts
import { openChatbot } from "@navikt/nav-dekoratoren-moduler";

openChatbot();
```

## injectDecoratorClientSide

CSR-fallback. Bruk kun hvis SSR ikke er mulig i arkitekturen.

```ts
import { injectDecoratorClientSide } from "@navikt/nav-dekoratoren-moduler";

injectDecoratorClientSide({
    env: "prod",
    params: { simple: true, chatbot: true },
});
```

## Window-events (lavnivå)

Dekoratørens Web Components kommuniserer via `window.dispatchEvent`. Tilgjengelige events:

| Event                      | Payload                   | Beskrivelse                       |
| -------------------------- | ------------------------- | --------------------------------- |
| `activecontext`            | `{ context }`             | Bruker byttet kontekst            |
| `paramsupdated`            | `{ params, changedKeys }` | Parametre ble oppdatert           |
| `authupdated`              | `AuthDataResponse`        | Auth-status endret                |
| `menuopened`               | –                         | Meny åpnet                        |
| `menuclosed`               | –                         | Meny lukket                       |
| `consentAllWebStorage`     | –                         | Bruker samtykket til all lagring  |
| `refuseOptionalWebStorage` | –                         | Bruker avslo frivillig lagring    |
| `closemenus`               | –                         | Lukk åpne menyer (sendes utenfra) |
