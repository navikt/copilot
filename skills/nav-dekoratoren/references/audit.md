# Auditing an existing integration

Use this checklist when the team wants to check an integration, or when a bug shows up only now and
then (for example after a Dekoratøren deploy). Inspect the code and report each finding with the
file, the consequence and a concrete fix. Change only what the team asked for, and suggest the rest.

1. **Static generation** – find pages or layouts built as static HTML (see 3.4 in SKILL.md). This
   is the most common reason the header or logged-in menu looks wrong after a Dekoratøren deploy,
   and that `teamName` is missing.
2. **Modules version** – check `@navikt/nav-dekoratoren-moduler` in `package.json` and the lockfile.
   Versions before 4.5 do not send `teamName`. Recommend the latest version. If the team asks for an
   upgrade, use the app's package manager, for example
   `npm install @navikt/nav-dekoratoren-moduler@latest`, and read the changelog in
   [releases](https://github.com/navikt/nav-dekoratoren-moduler/releases) before skipping a major
   version.
3. **`teamName`** – with SSR on Nais it is set automatically. A hardcoded `teamName` with SSR is
   unnecessary. With CSR and with direct `/ssr` calls without the modules package it must be set
   manually in the form `app.namespace`.
4. **Environment** – check that `env` follows the environment the app runs in. A fixed
   `env: "prod"` or a fallback to `prod` makes the dev environment use the prod decorator.
5. **Error handling** – look for a `.catch` around `fetchDecoratorReact` or `fetchDecoratorHtml`
   that returns empty components. The header then disappears without anyone noticing. The error
   should at least be logged. The modules package already retries three times and falls back to
   client-side rendering.
6. **Own cache** – if the app caches HTML that contains Dekoratøren (for example in memory, Redis or
   a CDN), the cache must be cleared with `addDecoratorUpdateListener` (see 3.5 in SKILL.md).
7. **CSR or SSR** – if the app uses `injectDecoratorClientSide` or direct CSR where SSR is
   possible, recommend SSR.
8. **Nais** – `accessPolicy.outbound` must allow `nav-dekoratoren` in `personbruker`, or the
   external hosts when `serviceDiscovery: false` (see 2.3 in SKILL.md).
9. **CSP** – if the app has its own CSP, build it with `buildCspHeader` so Dekoratøren's directives
   are included.
10. **Deprecated API** – upgrade one major version at a time and run type checks and a build after
    each.

    **v2 → v3 (SSR):**

    - `DECORATOR_STYLES` and `<Decorator.Styles />` are removed. Use `DECORATOR_HEAD_ASSETS` and
      `<Decorator.HeadAssets />` in `<head>`. Without them the header lacks CSS and favicon.
    - `injectDecoratorServerSideDom` is removed. Use `injectDecoratorServerSideDocument`, which
      takes a regular `Document`.
    - `parseDecoratorHTMLToReact` is removed. Use `fetchDecoratorReact`.
    - `<EnforceLoginLoader />`, the `enforceLogin` parameter, `getUrlFromLookupTable` and
      `urlLookupTable` are removed. Login must be handled in the app, for example with Wonderwall.
    - Dependencies are peer dependencies. Install `react` and `html-react-parser` yourself when
      using `fetchDecoratorReact`.
    - Your own cache of Dekoratøren can be cleared with `addDecoratorUpdateListener` (see 3.5 in
      SKILL.md).

    **v3 → v4 (analytics):**

    - `getAmplitudeInstance()` and `logAmplitudeEvent()` are removed. They have not logged anything
      since v3.5. Switch to `getAnalyticsInstance()` and `logAnalyticsEvent()` (see Step 5 in
      SKILL.md).
    - Event names are validated against `@navikt/analytics-types`. Use `Events.*` for taxonomy
      events and `logger.custom()` (v4.1+) for custom events.
    - The types `AmplitudeEvent`, `AmplitudeParams` and `AnalyticsEvent`, and generic types on
      `getAnalyticsInstance<...>()`, are removed. Invalid event names cause type errors.
    - The SSR and CSR APIs are otherwise unchanged from v3.

Summarize the findings by severity: first errors users notice, then missing traceability, and
finally recommendations.
