# Samtykke, cookies og ekomloven

Fra 1. januar 2025 krever ekomloven at Nav innhenter samtykke før analyse- og statistikkverktøy
aktiveres. Dekoratøren viser samtykkebanneret og håndterer lagring på tvers av apper. Moduler-pakken
gir helpers for din app.

Importeres fra `@navikt/nav-dekoratoren-moduler`.

---

## awaitDecoratorData

Vent til dekoratøren har lastet samtykke-data. Bruk alltid dette før du leser/skriver cookies ved
oppstart.

```ts
import { awaitDecoratorData } from "@navikt/nav-dekoratoren-moduler";

const initMyApp = async () => {
    await awaitDecoratorData();
    doMyAppStuff();
};
```

---

## isStorageKeyAllowed(key)

Sjekker om en nøkkel er:

1. på tillatt-listen, og
2. godkjent av brukerens samtykke (for frivillige nøkler)

```ts
import { isStorageKeyAllowed } from "@navikt/nav-dekoratoren-moduler";

// Returnerer false – "jabberwocky" er ikke i tillatt-listen
const ok = isStorageKeyAllowed("jabberwocky");

// Returnerer false – nøkkel er frivillig og bruker har ikke samtykket
const ok2 = isStorageKeyAllowed("usertest-229843829");
```

Gjelder for cookies, localStorage og sessionStorage.

---

## getAllowedStorage

Returnerer liste over all tillatt lagring basert på gjeldende samtykke.

```ts
import { getAllowedStorage } from "@navikt/nav-dekoratoren-moduler";

const allowed = getAllowedStorage();
// [
//   { name: "min-cookie", type: "cookie", optional: false },
//   { name: "min-key", type: "localstorage", optional: true },
//   ...
// ]
```

---

## setNavCookie / getNavCookie

Sett og les cookies – sjekker tillatt-liste og samtykke automatisk.

```ts
import { setNavCookie, getNavCookie } from "@navikt/nav-dekoratoren-moduler";

setNavCookie("decorator-language", "en");
const lang = getNavCookie("decorator-language");
```

---

## navSessionStorage / navLocalStorage

Drop-in replacement for `window.sessionStorage` og `window.localStorage` – respekterer samtykke
automatisk.

```ts
import {
    navLocalStorage,
    navSessionStorage,
} from "@navikt/nav-dekoratoren-moduler";

navLocalStorage.setItem("min-nøkkel", "verdi");
const val = navLocalStorage.getItem("min-nøkkel");
navLocalStorage.removeItem("min-nøkkel");

navSessionStorage.setItem("session-key", "data");
```

---

## Anbefalt oppstartsmønster

```ts
import {
    awaitDecoratorData,
    isStorageKeyAllowed,
    setNavCookie,
    navLocalStorage,
} from "@navikt/nav-dekoratoren-moduler";

async function init() {
    await awaitDecoratorData(); // alltid først

    if (isStorageKeyAllowed("min-analyse-cookie")) {
        setNavCookie("min-analyse-cookie", "aktiv");
    }

    navLocalStorage.setItem("sist-besøkt", new Date().toISOString());
}
```

---

## Mangler du en hjelpefunksjon?

Meld behov i `#dekoratøren_på_navno` på Slack. Teamet utvider moduler-pakken fortløpende.
