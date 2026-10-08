# All Dekoratøren configuration parameters

Set them as query parameters on direct SSR calls, or as the `params` object in the modules package.

| Parameter               | Type                                                  | Default        | Description                                                                |
| ----------------------- | ----------------------------------------------------- | -------------- | -------------------------------------------------------------------------- |
| `context`               | `privatperson` / `arbeidsgiver` / `samarbeidspartner` | `privatperson` | Sets the menu and context selector in the header                           |
| `simple`                | `boolean`                                             | `false`        | Simple version of header and footer                                        |
| `simpleHeader`          | `boolean`                                             | `false`        | Simple version of the header only                                          |
| `simpleFooter`          | `boolean`                                             | `false`        | Simple version of the footer only                                          |
| `redirectToApp`         | `boolean`                                             | `false`        | Send the user back to the current URL after login                          |
| `redirectToUrl`         | `string`                                              | `undefined`    | Send the user to the given URL after login (overrides redirectToApp)       |
| `redirectToUrlLogout`   | `string`                                              | `undefined`    | Send the user to the given URL after logout                                |
| `language`              | `nb` / `nn` / `en` / `se` / `pl` / `uk` / `ru`        | `nb`           | Sets the language. Overridden automatically by the URL path (/no/, /en/ etc.) |
| `availableLanguages`    | `{ locale, url, handleInApp? }[]`                     | `[]`           | Languages available in the language selector                               |
| `breadcrumbs`           | `{ title, url, analyticsTitle?, handleInApp? }[]`     | `[]`           | Breadcrumb trail                                                           |
| `utilsBackground`       | `white` / `gray` / `transparent`                      | `transparent`  | Background color for breadcrumbs and language selector                     |
| `feedback`              | `boolean`                                             | `false`        | Show the feedback component                                                |
| `chatbot`               | `boolean`                                             | `true`         | Enable the Frida chatbot (false = never initialized)                       |
| `chatbotVisible`        | `boolean`                                             | `false`        | Always show the chatbot icon (true) or only during an active session (false) |
| `shareScreen`           | `boolean`                                             | `true`         | Enable the screen sharing button in the footer                             |
| `logoutUrl`             | `string`                                              | `undefined`    | Delegate all logout to the given URL (the team handles cookie deletion)    |
| `logoutWarning`         | `boolean`                                             | `true`         | Show a warning after 55 min (WCAG requirement, disable only with an alternative) |
| `redirectOnUserChange`  | `boolean`                                             | `false`        | Redirect to nav.no if another user logs in in another window               |
| `origin`                | `string`                                              | `undefined`    | App identifier on automatic `besøk` events                                 |
| `pageType`              | `string`                                              | `undefined`    | Page type for analytics logging                                            |
| `analyticsQueryParams`  | `string[]`                                            | `[]`           | Allowlist of query params included in analytics (nothing sensitive!)       |
| `analyticsRedactFilter` | `string[]`                                            | `['uuid']`     | Opt out of automatic redaction (UUIDs are removed by default)              |

`redirectToApp` applies to both automatic login and the login button. `redirectToUrl` overrides
it; `redirectToUrlLogout` is the return address *after* logout. Both URLs must be on `nav.no` or a
subdomain, otherwise they are discarded. `logoutUrl` is something else: the app must then handle
all of logout itself. Do not turn off `logoutWarning` without giving the user another way to
postpone logout. A session lasts at most 6 hours.

`language` can be overridden by `/no/`, `/nb/`, `/nn/`, `/en/` and `/se/` in the URL. Dekoratøren's
own interface has text in Norwegian Bokmål, English and partly Northern Sami. URLs in `breadcrumbs`
and `availableLanguages` must be on `nav.no` or a subdomain; other URLs return 500 on fetch.
`analyticsTitle` can be used for breadcrumbs if the text contains no personal data.

`origin` identifies the app in analytics, not in Dekoratøren's consumer logs. With SSR via the
modules package, `teamName` is set automatically from `NAIS_APP_NAME.NAIS_NAMESPACE`. With direct
SSR, set `teamName` as a query parameter; with CSR and the modules package, set `params.teamName`.
Use the form `teamnavn.namespace` in lowercase with only `a-z`, `0-9`, `-` and `.`.

Query parameters are stripped from analytics by default. Include only non-sensitive parameters in
`analyticsQueryParams`, and do a risk assessment before changing `analyticsRedactFilter`.

## URL examples (direct calls)

```
# Set context
https://www.nav.no/dekoratoren/?context=arbeidsgiver

# Identify the app in besøk events
https://www.nav.no/dekoratoren/?origin=min-app

# Language selector
https://www.nav.no/dekoratoren/?availableLanguages=[{"locale":"nb","url":"https://www.nav.no/nb"},{"locale":"en","url":"https://www.nav.no/en"}]

# Breadcrumbs
https://www.nav.no/dekoratoren/?breadcrumbs=[{"url":"https://www.nav.no/person/dittnav","title":"Ditt Nav"},
{"url":"https://www.nav.no/person/kontakt-oss","title":"Kontakt oss"}]
```

## TypeScript type (full)

```ts
type DecoratorParams = Partial<{
    context: "privatperson" | "arbeidsgiver" | "samarbeidspartner";
    simple: boolean;
    simpleHeader: boolean;
    simpleFooter: boolean;
    redirectToApp: boolean;
    redirectToUrl: string;
    redirectToUrlLogout: string;
    language: "nb" | "nn" | "en" | "se" | "pl" | "uk" | "ru";
    availableLanguages: {
        locale: string;
        url: string;
        handleInApp?: boolean;
    }[];
    breadcrumbs: {
        title: string;
        url: string;
        analyticsTitle?: string;
        handleInApp?: boolean;
    }[];
    utilsBackground: "white" | "gray" | "transparent";
    feedback: boolean;
    chatbot: boolean;
    chatbotVisible: boolean;
    shareScreen: boolean;
    logoutUrl: string;
    logoutWarning: boolean;
    redirectOnUserChange: boolean;
    origin: string;
    pageType: string;
    analyticsQueryParams: string[];
    analyticsRedactFilter: string[];
}>;
```
