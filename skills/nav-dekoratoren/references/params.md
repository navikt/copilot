# Dekoratøren – alle konfigurasjonsparametre

Kan settes som query-parametre ved direkte SSR-kall, eller som `params`-objekt i moduler-pakken.

| Parameter               | Type                                                  | Default        | Forklaring                                                                |
| ----------------------- | ----------------------------------------------------- | -------------- | ------------------------------------------------------------------------- |
| `context`               | `privatperson` / `arbeidsgiver` / `samarbeidspartner` | `privatperson` | Angir meny og kontekstvelger i headeren                                   |
| `simple`                | `boolean`                                             | `false`        | Enkel versjon av header og footer                                         |
| `simpleHeader`          | `boolean`                                             | `false`        | Enkel versjon av kun header                                               |
| `simpleFooter`          | `boolean`                                             | `false`        | Enkel versjon av kun footer                                               |
| `redirectToApp`         | `boolean`                                             | `false`        | Send bruker tilbake til gjeldende URL etter innlogging                    |
| `redirectToUrl`         | `string`                                              | `undefined`    | Send bruker til angitt URL etter innlogging (overstyrer redirectToApp)    |
| `redirectToUrlLogout`   | `string`                                              | `undefined`    | Send bruker til angitt URL etter utlogging                                |
| `language`              | `nb` / `nn` / `en` / `se` / `pl` / `uk` / `ru`        | `nb`           | Angir språk. Overstyres automatisk av URL-path (/no/, /en/ osv.)          |
| `availableLanguages`    | `{ locale, url, handleInApp? }[]`                     | `[]`           | Tilgjengelige språk i språkvelgeren                                       |
| `breadcrumbs`           | `{ title, url, analyticsTitle?, handleInApp? }[]`     | `[]`           | Brødsmulesti                                                              |
| `utilsBackground`       | `white` / `gray` / `transparent`                      | `transparent`  | Bakgrunnsfarge for brødsmulesti og språkvelger                            |
| `feedback`              | `boolean`                                             | `false`        | Vis tilbakemeldingskomponenten                                            |
| `chatbot`               | `boolean`                                             | `true`         | Aktiver chatboten Frida (false = aldri initialisert)                      |
| `chatbotVisible`        | `boolean`                                             | `false`        | Vis chatbot-ikonet alltid (true) eller kun ved aktiv økt (false)          |
| `shareScreen`           | `boolean`                                             | `true`         | Aktiver skjermdeling-knapp i footer                                       |
| `logoutUrl`             | `string`                                              | `undefined`    | Deleger all utlogging til angitt URL (teamet håndterer cookie-sletting)   |
| `logoutWarning`         | `boolean`                                             | `true`         | Vis advarsel etter 55 min (WCAG-krav – deaktiver kun med eget alternativ) |
| `redirectOnUserChange`  | `boolean`                                             | `false`        | Redirect til nav.no hvis annen bruker logger inn i annet vindu            |
| `origin`                | `string`                                              | `undefined`    | Appidentifikator på automatiske `besøk`-hendelser                         |
| `pageType`              | `string`                                              | `undefined`    | Sidetype for Analytics-logging                                            |
| `analyticsQueryParams`  | `string[]`                                            | `[]`           | Hviteliste av query-params som inkluderes i Analytics (ingen sensitive!)  |
| `analyticsRedactFilter` | `string[]`                                            | `['uuid']`     | Opt-out av automatisk redaction (UUID fjernes som standard)               |

`redirectToApp` gjelder både automatisk innlogging og innloggingsknappen. `redirectToUrl`
overstyrer den; `redirectToUrlLogout` er returadressen *etter* utlogging. De to URL-ene må ligge
på `nav.no` eller et underdomene, ellers forkastes de. `logoutUrl` er noe annet: appen må da selv
håndtere all utlogging. Ikke slå av `logoutWarning` uten å gi brukeren en egen mulighet til å
utsette utlogging. Økten kan maksimalt vare i 6 timer.

`language` kan overstyres av `/no/`, `/nb/`, `/nn/`, `/en/` og `/se/` i URL-en. Dekoratørens eget
grensesnitt har tekst på bokmål, engelsk og delvis samisk. URL-ene i `breadcrumbs` og
`availableLanguages` må ligge på `nav.no` eller et underdomene; andre URL-er gir 500 ved henting.
`analyticsTitle` kan brukes for brødsmuler hvis teksten ikke inneholder personopplysninger.

`origin` identifiserer appen i analytics, ikke i Dekoratørens konsumentlogger. Ved SSR via
moduler-pakken settes `teamName` automatisk fra `NAIS_APP_NAME.NAIS_NAMESPACE`. Ved direkte SSR
må `teamName` settes som query-parameter; ved CSR med moduler-pakken settes `params.teamName`.
Bruk formen `teamnavn.namespace` med små bokstaver og bare `a-z`, `0-9`, `-` og `.`.

Query-parametre fjernes fra analytics som standard. Inkluder bare ikke-sensitive parametre i
`analyticsQueryParams`, og risikovurder før du endrer `analyticsRedactFilter`.

## URL-eksempler (direkte kall)

```
# Sett kontekst
https://www.nav.no/dekoratoren/?context=arbeidsgiver

# Identifiser appen i besøk-hendelser
https://www.nav.no/dekoratoren/?origin=min-app

# Språkvelger
https://www.nav.no/dekoratoren/?availableLanguages=[{"locale":"nb","url":"https://www.nav.no/nb"},{"locale":"en","url":"https://www.nav.no/en"}]

# Brødsmuler
https://www.nav.no/dekoratoren/?breadcrumbs=[{"url":"https://www.nav.no/person/dittnav","title":"Ditt Nav"},
{"url":"https://www.nav.no/person/kontakt-oss","title":"Kontakt oss"}]
```

## TypeScript-type (full)

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
