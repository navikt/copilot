# Client-side functions in @navikt/nav-dekoratoren-moduler

Imported from `@navikt/nav-dekoratoren-moduler` (without the `/ssr` suffix).

## setBreadcrumbs

Sets the breadcrumb trail dynamically. Use `handleInApp: true` for SPA routing.
⚠️ `title` is logged to Umami as `[redacted]` by default. Use `analyticsTitle` to log a title
without personal data.

```ts
import { setBreadcrumbs } from "@navikt/nav-dekoratoren-moduler";

setBreadcrumbs([
    { title: "Ditt Nav", url: "https://www.nav.no/person/dittnav" },
    {
        title: "Opplysninger for Ola Nordmann",
        analyticsTitle: "Opplysninger for <Navn>", // no personal data
        url: "https://www.nav.no/min-side",
        handleInApp: true,
    },
]);
```

## onBreadcrumbClick

Called when the user clicks a breadcrumb with `handleInApp: true`.
Use the framework's router: `router.push(url)` in Next.js, `navigate(url)` in React Router, or the
equivalent in other SPA frameworks.

```ts
import { onBreadcrumbClick } from "@navikt/nav-dekoratoren-moduler";

onBreadcrumbClick((breadcrumb) => {
    navigateTo(breadcrumb.url);
});
```

## setAvailableLanguages

Updates the language selector. The URL must be on `nav.no` or a subdomain.

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

Called on language selection with `handleInApp: true`.
Use the same router function as for breadcrumbs.

```ts
import { onLanguageSelect } from "@navikt/nav-dekoratoren-moduler";

onLanguageSelect((language) => {
    navigateTo(language.url);
});
```

## getAnalyticsInstance

Gets a logger instance for analytics (Umami). Supports taxonomy events and custom events.
Pass `origin` when creating the logger instance to identify the app's own events. Use the same
value as `origin` in the decorator parameters for automatic `besøk` events.

```ts
import {
    getAnalyticsInstance,
    Events,
    isValidEventName,
} from "@navikt/nav-dekoratoren-moduler";

const logger = getAnalyticsInstance("min-app");

// Taxonomy event – strictly typed from @navikt/analytics-types
logger(Events.SKJEMA_STARTET, { skjemaId: "1234", skjemanavn: "aap" });

// Custom event
logger.custom("feedback åpnet", { komponent: "feedback-widget", steg: 2 });

// Choose the event type dynamically
if (isValidEventName(eventName)) {
    logger(eventName, eventData);
} else {
    logger.custom(eventName, eventData);
}
```

Import event types directly:

```ts
import type {
    NavigereEvent,
    SkjemaStartetEvent,
} from "@navikt/nav-dekoratoren-moduler";
```

> ⚠️ `getAmplitudeInstance()` was removed in v4+. Use `getAnalyticsInstance()`.

Without consent to analytics, the logger discards events locally. The app does not need its own
consent check before calling the logger, but must still keep personal data out of event data.

## setParams / getParams

Update or read all parameters dynamically.

```ts
import { setParams, getParams } from "@navikt/nav-dekoratoren-moduler";

// Update parameters
setParams({ simple: true, chatbot: false });

// Read current parameters
const current = getParams();
```

## openChatbot

Opens the Frida chatbot and sets `chatbotVisible=true`.

```ts
import { openChatbot } from "@navikt/nav-dekoratoren-moduler";

openChatbot();
```

## injectDecoratorClientSide

CSR fallback. Use only if SSR is not possible in the architecture.

```ts
import { injectDecoratorClientSide } from "@navikt/nav-dekoratoren-moduler";

injectDecoratorClientSide({
    env: "prod",
    params: {
        simple: true,
        chatbot: true,
        teamName: "teamnavn.namespace", // consumer logging, example: "team-navno.navno"
    },
});
```

Set `params.teamName` for consumer logging. If the value is missing, Dekoratøren falls back to the
browser's `Origin` header and the modules package warns in the console.

## Window events (low level)

Dekoratøren's Web Components communicate via `window.dispatchEvent`. Available events:

| Event                      | Payload                   | Description                          |
| -------------------------- | ------------------------- | ------------------------------------ |
| `activecontext`            | `{ context }`             | User switched context                |
| `paramsupdated`            | `{ params, changedKeys }` | Parameters were updated              |
| `authupdated`              | `AuthDataResponse`        | Auth status changed                  |
| `menuopened`               | –                         | Menu opened                          |
| `menuclosed`               | –                         | Menu closed                          |
| `consentAllWebStorage`     | –                         | User consented to all storage        |
| `refuseOptionalWebStorage` | –                         | User refused optional storage        |
| `closemenus`               | –                         | Close open menus (sent from outside) |
