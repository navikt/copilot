# Consent, cookies and ekomloven

Since 1 January 2025, the Norwegian Electronic Communications Act (ekomloven) requires Nav to obtain
consent before analytics and statistics tools are activated. Dekoratøren shows the consent banner and
handles storage across apps. The modules package provides helpers for your app.
Without consent, only necessary storage is allowed. Umami and Skyra do not start.

Imported from `@navikt/nav-dekoratoren-moduler`.

---

## awaitDecoratorData

Waits until Dekoratøren has loaded consent data. Always use this before reading or writing cookies
at startup.

```ts
import { awaitDecoratorData } from "@navikt/nav-dekoratoren-moduler";

const initMyApp = async () => {
    await awaitDecoratorData();
    doMyAppStuff();
};
```

---

## isStorageKeyAllowed(key)

Checks whether a key is:

1. on the allowlist, and
2. approved by the user's consent (for optional keys)

```ts
import { isStorageKeyAllowed } from "@navikt/nav-dekoratoren-moduler";

// Returns false: "jabberwocky" is not on the allowlist
const ok = isStorageKeyAllowed("jabberwocky");

// Returns false: the key is optional and the user has not consented
const ok2 = isStorageKeyAllowed("usertest-229843829");
```

Applies to cookies, localStorage and sessionStorage.

---

## getAllowedStorage

Returns a list of all allowed storage based on current consent.

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

Set and read cookies. The functions check the allowlist and consent automatically. Necessary
cookies on the list can be set without consent; optional cookies require consent.

```ts
import { setNavCookie, getNavCookie } from "@navikt/nav-dekoratoren-moduler";

setNavCookie("decorator-language", "en");
const lang = getNavCookie("decorator-language");
```

---

## navSessionStorage / navLocalStorage

Replacements for `window.sessionStorage` and `window.localStorage` that check the allowlist and
consent automatically. Use keys registered for the right storage type.

```ts
import {
    navLocalStorage,
    navSessionStorage,
} from "@navikt/nav-dekoratoren-moduler";

navLocalStorage.setItem("registrert-localstorage-nøkkel", "verdi");
const val = navLocalStorage.getItem("registrert-localstorage-nøkkel");
navLocalStorage.removeItem("registrert-localstorage-nøkkel");

navSessionStorage.setItem("registrert-sessionstorage-nøkkel", "data");
```

---

## Recommended startup pattern

```ts
import {
    awaitDecoratorData,
    isStorageKeyAllowed,
    setNavCookie,
    navLocalStorage,
} from "@navikt/nav-dekoratoren-moduler";

async function init() {
    await awaitDecoratorData(); // always first

    if (isStorageKeyAllowed("registrert-cookie")) {
        setNavCookie("registrert-cookie", "aktiv");
    }

    navLocalStorage.setItem("registrert-localstorage-nøkkel", new Date().toISOString());
}
```

Replace the example names with keys that are actually on the allowlist. Unknown keys are not
allowed even if the user has consented.

---

## Missing a helper function?

Report the need in `#dekoratøren_på_navno` on Slack. The team extends the modules package continuously.
