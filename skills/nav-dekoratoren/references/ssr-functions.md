# SSR-funksjoner i @navikt/nav-dekoratoren-moduler

Importeres fra `@navikt/nav-dekoratoren-moduler/ssr`.

## fetchDecoratorHtml

Returnerer dekoratøren som HTML-fragmenter. Brukes for manuell injeksjon.

```ts
import { fetchDecoratorHtml } from "@navikt/nav-dekoratoren-moduler/ssr";

const {
    DECORATOR_HEAD_ASSETS, // CSS, favicons → <head>
    DECORATOR_HEADER, // Header HTML → rett før app-innhold
    DECORATOR_FOOTER, // Footer HTML → rett etter app-innhold
    DECORATOR_SCRIPTS, // <script>-elementer → hvor som helst
} = await fetchDecoratorHtml({
    env: "dev",
    params: { context: "privatperson" },
});
```

## fetchDecoratorReact

Returnerer React-komponenter for SSR-rammeverk (Next.js, Remix m.m.).
Krever `react >=17.x` og `html-react-parser >=5.x`.

### Next.js App Router

Bruk App Router-eksempelet for nye Next.js-apper og apper som allerede har `app/`.

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
        <html lang="no">
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

### Next.js Page Router

Bruk Page Router-eksempelet for eksisterende Next.js-apper med `pages/`.

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
            <Html lang="no">
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

## injectDecoratorServerSide

Parser en HTML-fil med JSDOM og returnerer HTML-string med dekoratøren injisert.
Krever `jsdom >=16.x`.

```ts
import { injectDecoratorServerSide } from "@navikt/nav-dekoratoren-moduler/ssr";

const html = await injectDecoratorServerSide({
    env: "prod",
    filePath: "index.html",
    params: { context: "privatperson", simple: true },
});

res.send(html);
```

## injectDecoratorServerSideDocument

Setter inn dekoratøren i et eksisterende `Document`-objekt (muteres).

```ts
import { injectDecoratorServerSideDocument } from "@navikt/nav-dekoratoren-moduler/ssr";

const document = await injectDecoratorServerSideDocument({
    env: "prod",
    document: myDocument,
    params: { context: "privatperson" },
});

res.send(document.documentElement.outerHTML);
```

## addDecoratorUpdateListener / removeDecoratorUpdateListener

Registrer callback ved ny dekoratørversjon. Brukes for cache-invalidering.

```ts
import {
    addDecoratorUpdateListener,
    removeDecoratorUpdateListener,
} from "@navikt/nav-dekoratoren-moduler/ssr";

const onUpdate = (versionId: string) => {
    console.log(`Ny versjon: ${versionId}`);
    myCache.clear();
};

addDecoratorUpdateListener({ env: "prod" }, onUpdate);

// Fjern igjen:
removeDecoratorUpdateListener({ env: "prod" }, onUpdate);
```

## getDecoratorVersionId

Henter nåværende versjons-ID for dekoratøren.

```ts
import { getDecoratorVersionId } from "@navikt/nav-dekoratoren-moduler/ssr";

const versionId = await getDecoratorVersionId({ env: "prod" });
```

## buildCspHeader

Bygger CSP-header som kombinerer appens egne direktiver med dekoratørens påkrevde direktiver.

```ts
import { buildCspHeader } from "@navikt/nav-dekoratoren-moduler/ssr";

const csp = await buildCspHeader(
    {
        "default-src": ["min-cdn.nav.no"],
        "style-src": ["css.nav.no"],
    },
    { env: "prod" },
);

res.setHeader("Content-Security-Policy", csp);
```

## Miljøer og service discovery

```ts
// Service discovery (default, fungerer på dev-gcp/prod-gcp)
fetchDecoratorHtml({ env: "prod" });

// Alltid eksterne ingresser
fetchDecoratorHtml({ env: "prod", serviceDiscovery: false });

// Lokal utvikling
fetchDecoratorHtml({ env: "localhost", localUrl: "http://localhost:8089" });
```
