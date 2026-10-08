---
name: nav-dekoratoren
description: Integrer, konfigurer og revider Nav Dekoratøren – felles header og footer for nav.no-applikasjoner. Bruk når et team skal ta i bruk Dekoratøren, sjekke en eksisterende integrasjon, oppdatere konfigurasjon, legge til breadcrumbs/språkvelger/analytics, håndtere samtykke (ekomloven), CSP eller feilsøke integrasjon mot dekoratøren.
license: MIT
compatibility: Web applications on nav.no (Next.js, Remix, Vite, Express/Node)
metadata:
  domain: frontend
  tags: nav dekoratoren header footer integration ssr analytics consent
---

# Nav Dekoratøren – integration

Dekoratøren is the shared header and footer for public-facing nav.no applications. It provides
SSR/CSR integration, login buttons via ID-porten, analytics (Umami) and consent and cookie handling
under the Norwegian Electronic Communications Act (ekomloven). Dekoratøren does not give the app
access to the user's login session. Set up authentication and access control in the app itself.

**Slack:** `#dekoratøren_på_navno`
**Repo:** https://github.com/navikt/nav-dekoratoren
**Documentation:** https://github.com/navikt/nav-dekoratoren/blob/main/README.md
**Modules package:** https://github.com/navikt/nav-dekoratoren-moduler
**Storybook:** https://navikt.github.io/nav-dekoratoren

---

## Step 1: Find out what the team needs

If you have access to the code, start by inspecting the repository. Look for:

1. **Purpose** – does the app already use Dekoratøren, or is this a first integration?
2. **Framework** – Next.js `app/` (App Router), Next.js `pages/` (Page Router), Remix / React Router
   framework mode, Vite SPA / React Router library mode, or plain Node/Express.
3. **Environment** – `prod`, `dev`, `localhost`, an existing `nais.yaml`, and whether service
   discovery is available.
4. **Needs** – breadcrumbs, language selector, analytics, chatbot, consent/cookies, CSP and skip link.

Only ask about what you cannot find in the repository or what requires a product decision. Then
follow steps 2–7 and the relevant references. If the app already uses Dekoratøren and the team wants
to check the setup or troubleshoot, go to [Auditing an existing integration](references/audit.md).

---

## Step 2: Installation and setup

### 2.1 Install the modules package

```bash
npm install --save @navikt/nav-dekoratoren-moduler
```

The package is published only on **GitHub Packages**. Add to `.npmrc`:

```text
@navikt:registry=https://npm.pkg.github.com
```

Log in with a PAT that has the `read:packages` scope and navikt SSO authorization:

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

### 2.3 Access policy in nais.yaml

With service discovery (recommended, used automatically on dev-gcp/prod-gcp):

```yaml
accessPolicy:
    outbound:
        rules:
            - application: nav-dekoratoren
              namespace: personbruker
```

With external ingresses (if `serviceDiscovery: false`):

```yaml
accessPolicy:
    outbound:
        external:
            - host: www.nav.no # prod
            - host: dekoratoren.ekstern.dev.nav.no # dev
```

---

## Step 3: SSR integration (recommended)

Server-side rendering gives the best performance and avoids layout shift.
See [SSR-FUNCTIONS.md](references/ssr-functions.md) for full API details.
The modules package tries to fetch Dekoratøren three times before it falls back to static
placeholders rendered on the client. With SSR on Nais, the package sets `teamName` to
`NAIS_APP_NAME.NAIS_NAMESPACE` automatically for consumer logging. The variables are read when
Dekoratøren is fetched, so they exist only at runtime on Nais, not in the build step. Without them
the package warns once and uses `params.teamName` if you set it manually. See
[consumer logging](https://github.com/navikt/nav-dekoratoren/blob/main/README.md#10-innebygde-funksjoner-i-dekorat%C3%B8ren)
for direct SSR calls or CSR.

### 3.1 Next.js App Router

Use this for new Next.js apps and apps that already have `app/`. The root layout can be async and
fetch Dekoratøren directly.

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

Use this only for existing Next.js apps with `pages/`. In the Page Router, Dekoratøren must be
fetched in `pages/_document.tsx`, because `_document` owns `<html>`, `<head>` and the
server-rendered HTML shell.

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

### 3.3 Express/Node (HTML fragments)

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

For direct calls without the modules package, send `teamName` as a query parameter on `/ssr`, for
example `https://www.nav.no/dekoratoren/ssr?teamName=team-navno.navno`. With service discovery the
address is `http://nav-dekoratoren.personbruker/ssr?teamName=team-navno.navno`, without
`/dekoratoren` before `/ssr`. `teamName` must have the form `teamnavn.namespace`: lowercase, at
least one dot, and only `a-z`, `0-9`, `-` and `.`. `origin` is used for analytics and does not
replace `teamName`.

### 3.4 Avoid static generation of pages with Dekoratøren

Dekoratøren must be fetched at runtime, per request or from the modules package cache. If Next.js
builds the page as static HTML, Dekoratøren's HTML, CSS and version ID are frozen at build time.
After the next Dekoratøren deploy, the page mixes old CSS with new HTML from `/auth` and other
endpoints, and the logged-in menu, for example, can render incorrectly. Rebuilding the app fixes it
until the next deploy.
`teamName` is also missing: the modules package reads `NAIS_APP_NAME` and `NAIS_NAMESPACE` when
Dekoratøren is fetched, and they exist only in the pod on Nais, not in the build step (for example
GitHub Actions or Docker build). When the page is rendered per request, for example with
`getServerSideProps`, `_document` runs in the pod and `teamName` is set automatically.

When you inspect a repository, look for:

- **Page Router:** `_document` also runs for statically generated pages. That covers pages with
  `getStaticProps` and pages without data fetching (Automatic Static Optimization). Replace
  `getStaticProps` with `getServerSideProps`. Pages without data fetching can be made dynamic one by
  one with `getServerSideProps`, or for the whole app with `getInitialProps` in `pages/_app.tsx`.
  The latter does not apply to pages with `getStaticProps`. `next build` marks static pages with
  `○` or `●` and dynamic pages with `ƒ`.
- **App Router:** routes without dynamic APIs are prerendered in the build step. Make the root
  layout dynamic with `export const dynamic = "force-dynamic";`, or call `await connection()` from
  `next/server` before `fetchDecoratorReact`.

```ts
// pages/minside.tsx (Page Router)
export async function getServerSideProps({ locale }: GetServerSidePropsContext) {
    return { props: { ...(await serverSideTranslations(locale ?? "nb", ["common"])) } };
}
```

If the app cannot be made dynamic yet, set `teamName` manually in `params` (for example
`teamName: "min-app.mitt-namespace"`) so the requests can at least be traced. This does not fix the
version problem.

### 3.5 Cache invalidation

```ts
import { addDecoratorUpdateListener } from "@navikt/nav-dekoratoren-moduler/ssr";

addDecoratorUpdateListener({ env: "prod" }, (versionId) => {
    console.log(`Ny dekoratørversjon: ${versionId} – tømmer cache`);
    myHtmlCache.clear();
});
```

---

## Step 4: Configuration

See [PARAMS.md](references/params.md) for every parameter with type and default value.

The most important:

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

`language` accepts every language in the table, but text in Dekoratøren's own interface exists
only in Norwegian Bokmål, English and partly Northern Sami. URLs containing `/no/`, `/nb/`, `/nn/`,
`/en/` or `/se/` can override `language`. URLs in `breadcrumbs` and `availableLanguages` must be on
`nav.no` or a subdomain; otherwise Dekoratøren responds with 500.

Use `redirectToUrl` or `redirectToUrlLogout` for the return address after login and logout
respectively. Both reject URLs outside `nav.no` and its subdomains. Do not confuse them with
`logoutUrl`: it hands *all* of logout, including deleting cookies and sessions, to the app.
If you turn off `logoutWarning`, the app must let the user postpone logout itself.
See [all parameters](references/params.md).

---

## Step 5: Client-side features

See [CLIENT-FUNCTIONS.md](references/client-functions.md) for complete examples.

### 5.1 Breadcrumbs

Use `handleInApp: true` when the app handles navigation itself. Replace `navigateTo` with the
framework's router, for example `router.push` in Next.js or `navigate` from React Router.
`title` is logged to Umami as `[redacted]`. Set `analyticsTitle` only to text without personal
data if you want to log a title.

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

### 5.2 Language selector

The same pattern applies to language selection with `handleInApp: true`.

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

Pass the app's technical name as `origin` in the decorator parameters. The value is added to
automatic `besøk` events, so page views can be filtered per app. When the parameter is omitted,
`besøk` events use the value `nav-dekoratoren`.
Dekoratøren does not send Umami events without consent. The logger still accepts the calls and
discards them locally. Query parameters are stripped from page views by default; use
`analyticsQueryParams` only for names whose values cannot contain personal data.
Changing `analyticsRedactFilter` requires a separate risk assessment.

```ts
import { getAnalyticsInstance, Events } from "@navikt/nav-dekoratoren-moduler";

const logger = getAnalyticsInstance("min-app");

// Taxonomy event (strictly typed from @navikt/analytics-types)
logger(Events.SKJEMA_STARTET, { skjemaId: "1234", skjemanavn: "aap" });

// Custom event
logger.custom("feedback åpnet", { komponent: "feedback-widget", steg: 2 });
```

> ⚠️ `getAmplitudeInstance()` was removed in v4+. Use `getAnalyticsInstance()`.

### 5.4 Chatbot

```ts
import { openChatbot } from "@navikt/nav-dekoratoren-moduler";

openChatbot(); // opens Frida and sets chatbotVisible=true
```

---

## Step 6: Consent and cookies (ekomloven)

See [CONSENT.md](references/consent.md) for details.
Keys must be on the allowlist. Consent alone does not make an unknown key allowed.

```ts
import {
    awaitDecoratorData,
    isStorageKeyAllowed,
    setNavCookie,
    getNavCookie,
    navLocalStorage,
} from "@navikt/nav-dekoratoren-moduler";

// Wait until Dekoratøren has loaded consent
await awaitDecoratorData();

// Replace with a key registered as a cookie on the allowlist
if (isStorageKeyAllowed("registrert-cookie")) {
    setNavCookie("registrert-cookie", "verdi");
}

// Use a key registered for localStorage
navLocalStorage.setItem("registrert-localstorage-nøkkel", "verdi");
```

---

## Step 7: CSP header

Use `buildCspHeader` to combine the app's CSP with the directives Dekoratøren needs.
With a direct integration, the app must keep its CSP in sync with
[`/dekoratoren/api/csp`](https://www.nav.no/dekoratoren/api/csp) itself.

```ts
import { buildCspHeader } from "@navikt/nav-dekoratoren-moduler/ssr";

const csp = await buildCspHeader(
    { "default-src": ["min-cdn.nav.no"], "style-src": ["css.nav.no"] },
    { env: "prod" },
);

res.setHeader("Content-Security-Policy", csp);
```

## Skip link

Dekoratøren shows a link to the main content when the document has an element with
`id="maincontent"`. Make the element focusable so keyboard focus moves there:

```html
<main id="maincontent" tabindex="-1">Appens innhold</main>
```

---

## Auditing an existing integration

When the team wants to check an existing integration, or a bug shows up only now and then (for
example after a Dekoratøren deploy), follow the checklist in [AUDIT.md](references/audit.md).
Start with static generation (3.4), the most common cause.

---

## Common errors and fixes

| Problem                                                          | Cause                       | Fix                                                        |
| ---------------------------------------------------------------- | --------------------------- | ---------------------------------------------------------- |
| `403` on npm install                                             | Missing PAT or SSO          | Create a PAT with `read:packages`, enable navikt SSO       |
| Dekoratøren does not show                                        | Missing access policy       | Add `nav-dekoratoren` to `accessPolicy.outbound.rules`     |
| Layout shift                                                     | CSR is used                 | Switch to SSR via `fetchDecoratorReact`                    |
| Cookie not set                                                   | Consent not obtained        | Use `awaitDecoratorData()` + `setNavCookie`                |
| `getAmplitudeInstance is not a function`                         | Modules v4+ (API changed)   | Upgrade to v4+ and switch to `getAnalyticsInstance`        |
| `availableLanguages` URL error                                   | URL outside nav.no          | Only `nav.no` and subdomains are allowed                   |
| Consumer shows as `unknown`                                      | Missing `teamName`          | Set `teamName` for direct SSR calls and CSR with modules   |
| `unknown` even with SSR and modules 4.5+                         | The page is built statically | Make the page dynamic, see 3.4                            |
| Header or logged-in menu wrong after a decorator deploy, fixed by redeploy | The page is built statically | Make the page dynamic, see 3.4                |

---

## Environments and ingresses

| Environment | Service host                                   | Ingress                                          |
| ----------- | ---------------------------------------------- | ------------------------------------------------ |
| `prod`      | `http://nav-dekoratoren.personbruker`          | `https://www.nav.no/dekoratoren`                 |
| `dev`       | `http://nav-dekoratoren.personbruker`          | `https://dekoratoren.ekstern.dev.nav.no`         |
| `beta`      | `http://nav-dekoratoren-beta.personbruker`     | `https://dekoratoren-beta.intern.dev.nav.no`     |
| `beta-tms`  | `http://nav-dekoratoren-beta-tms.personbruker` | `https://dekoratoren-beta-tms.intern.dev.nav.no` |

> ⚠️ Beta instances are only for internal testing by Team Nav.no / Team Min Side and may be unstable.
