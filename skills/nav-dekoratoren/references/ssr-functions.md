# SSR functions in @navikt/nav-dekoratoren-moduler

Imported from `@navikt/nav-dekoratoren-moduler/ssr`.

## fetchDecoratorHtml

Returns Dekoratøren as HTML fragments. Used for manual injection.

```ts
import { fetchDecoratorHtml } from "@navikt/nav-dekoratoren-moduler/ssr";

const {
    DECORATOR_HEAD_ASSETS, // CSS, favicons → <head>
    DECORATOR_HEADER, // Header HTML → right before app content
    DECORATOR_FOOTER, // Footer HTML → right after app content
    DECORATOR_SCRIPTS, // <script> elements → anywhere
} = await fetchDecoratorHtml({
    env: "dev",
    params: { context: "privatperson" },
});
```

## fetchDecoratorReact

Returns React components for SSR frameworks (Next.js, Remix and others).
Requires `react >=17.x` and `html-react-parser >=5.x`.

### Next.js App Router

Use the App Router example for new Next.js apps and apps that already have `app/`.

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

### Next.js Page Router

Use the Page Router example for existing Next.js apps with `pages/`.

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

## injectDecoratorServerSide

Reads an HTML file and returns an HTML string with Dekoratøren injected. The file must be a
complete HTML document with `</head>`, `<body>` and `</body>`. From version 3.7.0, `jsdom` is not
needed.

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

Inserts Dekoratøren into an existing `Document` object (mutated in place).

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

Registers a callback for new decorator versions. Used for cache invalidation.

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

// Remove again:
removeDecoratorUpdateListener({ env: "prod" }, onUpdate);
```

## getDecoratorVersionId

Gets the current version ID of Dekoratøren.

```ts
import { getDecoratorVersionId } from "@navikt/nav-dekoratoren-moduler/ssr";

const versionId = await getDecoratorVersionId({ env: "prod" });
```

## buildCspHeader

Builds a CSP header that combines the app's own directives with the directives Dekoratøren requires.

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

## Environments and service discovery

```ts
// Service discovery (default, works on dev-gcp/prod-gcp)
fetchDecoratorHtml({ env: "prod" });

// Always external ingresses
fetchDecoratorHtml({ env: "prod", serviceDiscovery: false });

// Local development
fetchDecoratorHtml({ env: "localhost", localUrl: "http://localhost:8089" });
```
