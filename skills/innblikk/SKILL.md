---
name: innblikk
description: Sett opp Innblikk (Team ResearchOps' nettstatistikk, basert på Umami) i en Nav-app som ikke bruker dekoratøren. Bruk når et team vil telle sidevisninger med sporing.js, trenger samtykke etter ekomloven, CSP for Innblikk, en before-send-hook som holder private sider ute, eller vil sjekke en eksisterende Innblikk-integrasjon. Bruker appen dekoratøren, bruk nav-dekoratoren i stedet.
license: MIT
compatibility: Web applications without Nav Dekoratøren (Next.js, Vite, plain HTML)
metadata:
  domain: frontend
  tags: innblikk umami analytics consent ekomloven csp researchops
---

# Innblikk without dekoratøren

Innblikk is Team ResearchOps' web analytics, based on Umami. It was called Umami Start before.
Apps that use Nav Dekoratøren already get Innblikk through it, with consent handled. For those,
use the `nav-dekoratoren` skill instead. This skill is for apps that load the script themselves.

**Slack:** `#researchops`
**Worked example:** `apps/my-copilot` in navikt/copilot (ki-utvikling.nav.no)

## Step 1: Tracking code and approval

1. Get a tracking code (website ID) from https://innblikk.ansatt.nav.no/sporingskoder, or ask in
   `#researchops` on Slack.
2. Before prod: fill in the reusable Innblikk etterlevelsesdokument (linked from the "Kom i gang"
   page on innblikk.ansatt.nav.no) and describe the use on the app's privacy page.

The website ID is public: it ends up in the page HTML. Pass it in as a runtime environment variable,
so it can be switched on and off without a rebuild:

```yaml
# .nais/app.yaml (Handlebars template, values per environment)
env:
  {{#if innblikk_website_id}}
  - name: INNBLIKK_WEBSITE_ID
    value: "{{innblikk_website_id}}"
  {{/if}}
```

Render nothing when the variable is unset.

## Step 2: The script tag

Use this exact URL: `https://cdn.nav.no/team-researchops/sporing/sporing.js`.

```html
<script
  defer
  src="https://cdn.nav.no/team-researchops/sporing/sporing.js"
  data-website-id="YOUR-WEBSITE-ID"
  data-domains="minapp.nav.no"
  data-exclude-search="true"
  data-do-not-track="true"
  data-before-send="innblikkBeforeSend"
></script>
```

| Attribute | Effect in sporing.js |
| --- | --- |
| `data-website-id` | The tracking code. No ID, no sends. |
| `data-domains` | Comma-separated hostnames. On any other host nothing is sent. Set only the prod host, so dev and preview send nothing. |
| `data-exclude-search="true"` | Strips the query string from the URL and referrer. The hash is kept. |
| `data-do-not-track="true"` | Respects the browser's Do Not Track setting. |
| `data-before-send` | Name of a function on `window`. It gets `(type, payload)` before every send. Return the payload to send it, or a falsy value to cancel. |

The script also reads `data-tag`, `data-auto-track`, `data-fetch-credentials` (default `omit`)
and `data-opt-out-filters`. Do not use attributes beyond these. It ignores them.

Behaviour to know before writing the hook:

- Page views are sent on load, and about 300 ms after a `history.pushState`/`replaceState` that
  changes the URL, so client-side navigation is tracked once the script is loaded. Back and
  forward (`popstate`) are not tracked.
- On navigation, `referrer` is the previous URL in the app. On first load it is
  `document.referrer`, or empty when that is the same origin.
- The hash stays in `url` and `referrer`.
- Clicks on elements with `data-umami-event` or `data-sporing-event` attributes send custom
  events; every `data-umami-event-<key>` attribute on the element is sent as event data, and the
  hook below passes `name` and `data` through unchanged. Never put personal data in them, and do
  not add the attributes at all if the app counts page views only.

### The before-send hook

Define it on `window` before the script loads. sporing.js looks up `window[name]` at every send,
which is why withdrawn consent takes effect without a reload. It must drop private pages, drop
private referrers and strip the hash:

```ts
export function innblikkBeforeSend(_type: string, payload: { url?: string; referrer?: string }) {
  if (readConsent() !== true) return false; // withdrawn consent stops sends without a reload
  const path = (u: string) => decodePath(new URL(u, location.href).pathname);
  if (!payload.url || isPrivatePath(path(payload.url))) return false;
  const noHash = (u?: string) => u?.split("#")[0];
  const referrer = payload.referrer && !isPrivatePath(path(payload.referrer)) ? noHash(payload.referrer) : undefined;
  return { ...payload, url: noHash(payload.url), referrer };
}
if (typeof window !== "undefined") {
  (window as unknown as Record<string, unknown>).innblikkBeforeSend = innblikkBeforeSend;
}
```

`decodePath` is `decodeURIComponent` with a fallback, so `/%61dmin` counts as `/admin`. Use the
same private-path list that your access control enforces, so a new protected page is excluded
automatically.

## Step 3: Consent

Analytics storage and reading on the user's device needs consent under ekomloven § 3-15. This is
the reason for the flow below, not legal advice. Ask your privacy contact if in doubt.

- Load nothing before an explicit yes: no script tag, no request to cdn.nav.no.
- Show a yes button and a no button with the same size and style.
- Store the choice locally with a version and a date, for example in `localStorage` as
  `{ analytics: true, version: 1, at: "2026-10-31T09:00:00.000Z" }`. Treat an unknown version as
  no choice.
- On no or withdrawal, set both `umami.disabled` and `sporing.disabled` in `localStorage`.
  sporing.js checks both before every send. On yes, remove them.
- sporing.js also checks the flags once at load. If a flag is set then, it never installs its
  navigation hooks, so remove the flags before loading the script, as `setConsent` does.
- Make withdrawal as easy as consent, for example with the same two buttons on the privacy page.

```ts
export function setConsent(analytics: boolean) {
  localStorage.setItem(CONSENT_KEY, JSON.stringify({ analytics, version: 1, at: new Date().toISOString() }));
  for (const flag of ["umami.disabled", "sporing.disabled"]) {
    if (analytics) localStorage.removeItem(flag);
    else localStorage.setItem(flag, "1");
  }
  window.dispatchEvent(new Event(CONSENT_EVENT));
}
```

The reference (`innblikk-consent.ts`) wraps the storage calls in `try`/`catch`. Do the same: if
storage throws, keep the choice in memory for the page
load so the banner does not come back on every render.

## Step 4: CSP and Referrer-Policy

```text
script-src  'self' https://cdn.nav.no/team-researchops/sporing/sporing.js
connect-src 'self' https://reops-event-proxy.nav.no
```

- Allow the exact script URL, not all of `cdn.nav.no`.
- sporing.js posts to `https://reops-event-proxy.nav.no/api/send`. On hosts containing
  `.dev.nav.no` or `.dev.intern.nav.no` it posts to `https://reops-event-proxy.ekstern.dev.nav.no`
  instead. Add that host to connect-src in dev only if dev should send.
- Set `Referrer-Policy: strict-origin-when-cross-origin`, so the full URL with query string is not
  sent in the `Referer` header to cdn.nav.no and the event proxy.

## Step 5: Data rules

- Page views only.
- No personal data in events, page titles or query strings. The title is sent with every page
  view.
- Never load the script on logged-in or admin pages, and cancel events from them in the hook.
- Do not call `identify()` (`window.umami.identify` / `window.sporing.identify`). The id is
  attached to every later event.
- Tracking free-text or search content is out of scope here. Ask #researchops and your privacy
  contact first.

## Framework notes

### Next.js App Router

- A client component (`"use client"`) renders the banner or the script with `next/script`
  (`strategy="afterInteractive"`).
- Read the choice with `useSyncExternalStore` and an `undefined` server snapshot, so the server
  HTML has neither the banner nor the script.
- Read `process.env.INNBLIKK_WEBSITE_ID` in a server component that renders per request, and pass
  it as a prop. A `NEXT_PUBLIC_` variable would be baked in at build time.
- Assign `window.innblikkBeforeSend` at module scope in the client component, so it exists
  before sporing.js runs. Guard it with `typeof window !== "undefined"`, because the module also
  runs on the server.
- Set CSP and Referrer-Policy in `headers()` in `next.config.ts`.

### Plain HTML and other frameworks

Define `window.innblikkBeforeSend` first. After a yes, inject the tag:

```js
const s = document.createElement("script");
s.src = "https://cdn.nav.no/team-researchops/sporing/sporing.js";
Object.entries({
  websiteId: "YOUR-WEBSITE-ID",
  domains: "minapp.nav.no",
  excludeSearch: "true",
  doNotTrack: "true",
  beforeSend: "innblikkBeforeSend",
}).forEach(([k, v]) => (s.dataset[k] = v));
document.head.appendChild(s);
```

## Step 6: Verify

**Headless browser** (Playwright or similar) against prod or a host listed in `data-domains`.
Record every request:

| Case | Expected |
| --- | --- |
| Before a choice | No request to cdn.nav.no or reops-event-proxy |
| After yes | `sporing.js` returns 200, `POST /api/send` returns 2xx (201 in a headless check on ki-utvikling.nav.no, 2026-10-31) |
| After no, then navigate | No new `/api/send` |
| Private page, consent given | No script tag, no `/api/send` |

**Unit tests with controls.** Every negative test needs a positive control in the same setup, or
it passes when nothing renders at all. Examples from my-copilot:

- No ID renders nothing; control: the same page with an ID renders the script.
- No sends from a private page; control: a public page passes the hook.
- No sets the opt-out flags and never loads the script; control: yes loads it.
- The hook strips the hash; control: a plain URL is unchanged.
- A private referrer is dropped; control: a public referrer is kept.

Mock `next/script` as a plain `<script>` so the render is observable.

## Worked example: my-copilot

In navikt/copilot:

- `apps/my-copilot/src/components/innblikk.tsx`: script tag, banner, `innblikkBeforeSend`, the
  withdrawal control for the privacy page.
- `apps/my-copilot/src/lib/innblikk-consent.ts`: consent store and opt-out flags.
- `apps/my-copilot/src/lib/private-paths.ts`: the private paths shared with access control.
- `apps/my-copilot/src/components/site-shell.tsx`: passes `INNBLIKK_WEBSITE_ID` at runtime.
- `apps/my-copilot/next.config.ts`: CSP and Referrer-Policy.
- `apps/my-copilot/.nais/app.yaml`, `prod-gcp.yaml`, `dev-gcp.yaml`: how the ID gets in.
- `apps/my-copilot/src/components/innblikk.test.tsx`: tests with controls.
