---
name: aksel-spacing
description: Lag responsive layouts med Aksel Design System (v8+) - spacing tokens, layout primitives (Box, HStack, VStack, HGrid, Page, Bleed) og ResponsiveProp
license: MIT
compatibility: Next.js with @navikt/ds-react
metadata:
  domain: frontend
  tags: aksel design-system nav react spacing tokens layout responsive primitives
---

# Aksel: Responsivt layout med spacing tokens

Referanse for å bygge responsive layouts med Aksel v8+. Hent oppdatert dokumentasjon fra [https://aksel.nav.no/llm.md](https://aksel.nav.no/llm.md) ved behov.

> **v8-BRUDD:** Tokens = pixelverdi direkte. `--ax-space-4` = 4px (ikke 16px!). Bruk alltid tallverdien som matcher ønsket px.

---

## Spacing-tokens

Token name = eksakt pikselverdi. Full referanse: [design-tokens](https://aksel.nav.no/grunnleggende/styling/design-tokens.md)

### CSS-variabler

```css
.my-element {
  padding: var(--ax-space-16); /* 16px */
  gap: var(--ax-space-8); /* 8px */
  margin-block: var(--ax-space-32); /* 32px */
  padding-inline: var(--ax-space-24) var(--ax-space-48);
}
```

---

## Breakpoints og ResponsiveProp

Alle layout-primitives aksepterer responsiv objektverdi (mobil-first). Full referanse: [brekkpunkter](https://aksel.nav.no/grunnleggende/styling/brekkpunkter.md)

```tsx
// ResponsiveProp-syntaks – verdi per breakpoint (mobil-first)
<HStack gap={{ xs: "space-8", md: "space-24" }} />
<Box padding={{ xs: "space-16", md: "space-32", lg: "space-48" }} />
<HGrid columns={{ xs: 1, sm: 2, lg: 3 }} />
```

Utelatte breakpoints arver fra nærmeste definerte nedenfra.

---

## Layout Primitives

### Box

Fleksibelt layout-element. Brukes til padding, bakgrunn, border og posisjonering.

Props: `padding`, `paddingBlock`, `paddingInline`, `paddingBlockStart`, `paddingBlockEnd`, `paddingInlineStart`, `paddingInlineEnd`, `margin`, `marginBlock`, `marginInline`, `width`, `height`, `minWidth`, `maxWidth`, `overflow`, `background`, `borderColor`, `borderWidth`, `borderRadius`, `shadow`, `as`, `asChild`

```tsx
import { Box } from "@navikt/ds-react";

<Box padding="space-16">Innhold</Box>

// Responsiv padding
<Box
  padding={{ xs: "space-16", md: "space-32" }}
  paddingInline={{ xs: "space-16", lg: "space-48" }}
>
  Innhold
</Box>

// asChild – send props til eksisterende element
<Box asChild padding="space-24">
  <main>Innhold</main>
</Box>
```

### HStack

Horisontal flex-container.

Props: `gap`, `align`, `justify`, `wrap`, `as`, `asChild`

```tsx
import { HStack } from "@navikt/ds-react";

<HStack gap="space-16" align="center" justify="space-between">
  <Logo />
  <NavLinks />
</HStack>

// Responsiv gap
<HStack gap={{ xs: "space-8", md: "space-24" }} wrap={false}>
  <Item />
  <Item />
</HStack>
```

`align`: `"start"` | `"center"` | `"end"` | `"baseline"` | `"stretch"` (flex `align-items`)
`justify`: `"start"` | `"center"` | `"end"` | `"space-between"` | `"space-around"` | `"space-evenly"` (flex `justify-content`)

### VStack

Vertikal flex-container.

Props: `gap`, `align`, `justify`, `as`, `asChild`

```tsx
import { VStack } from "@navikt/ds-react";

<VStack gap="space-24">
  <Section />
  <Section />
</VStack>

// Responsiv gap
<VStack gap={{ xs: "space-16", md: "space-32" }}>
```

### Stack

Kombinert HStack/VStack med responsiv retning.

```tsx
import { Stack } from "@navikt/ds-react";

// Kolonne på mobil, rad på desktop
<Stack
  direction={{ xs: "column", sm: "row" }}
  gap={{ xs: "space-16", sm: "space-24" }}
  align={{ sm: "center" }}
>
  <LeftContent />
  <RightContent />
</Stack>;
```

### HGrid

CSS Grid-basert. For kolonnelayout.

Props: `columns`, `gap`, `align`, `as`, `asChild`

```tsx
import { HGrid } from "@navikt/ds-react";

// Fast antall kolonner
<HGrid columns={3} gap="space-24">

// Responsivt koloneantall
<HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap={{ xs: "space-16", md: "space-24" }}>

// CSS grid-string for avanserte oppsett
<HGrid columns="repeat(auto-fit, minmax(16rem, 1fr))" gap="space-16">

// Ulik gap horisontal/vertikal
<HGrid columns={2} gap={{ xs: "space-16 space-8" }}>
//                                   ↑ block  ↑ inline
```

`align`: `"start"` | `"center"` | `"end"` (CSS `align-items` på grid)

### Page og Page.Block

Standardisert sidelayout med header/footer-støtte. Full API: [page](https://aksel.nav.no/komponenter/primitives/page.md)

```tsx
import { Page } from "@navikt/ds-react";

<Page footer={<Footer />} footerPosition="belowFold" contentBlockPadding="end">
  <Header />
  <Page.Block as="main" width="xl" gutters>
    {/* Innhold */}
  </Page.Block>
</Page>;
```

`Page.Block` widths: `"text"` (576px) | `"md"` (768px) | `"lg"` (1024px) | `"xl"` (1280px) | `"2xl"` (1440px)
`gutters` = `padding-inline`: 3rem (>md), 1rem (≤md).

### Bleed

Negativ margin – bryt ut av container-padding.

Props: `marginInline`, `marginBlock`, `reflectivePadding`

```tsx
import { Bleed } from "@navikt/ds-react";

// Strekk bilde over container-padding
<Bleed marginInline="space-24">
  <FullWidthImage />
</Bleed>

// Kombinert bleed
<Bleed marginInline="space-32" marginBlock="space-16">
  <Banner />
</Bleed>
```

`reflectivePadding`: legg til tilsvarende padding inni, slik at innholdet ikke klemmes.

### Spacer

Fyller tilgjengelig plass i flex-container (`flex: 1 1 auto`).

```tsx
import { Spacer } from "@navikt/ds-react";

<HStack>
  <Logo />
  <Spacer /> {/* Skyver resten til høyre */}
  <NavItems />
</HStack>;
```

### Show og Hide

Conditionally skjul/vis innhold per breakpoint. Bruker CSS `display`-egenskap (ingen JS).

```tsx
import { Show, Hide } from "@navikt/ds-react";

<Hide below="md"><DesktopMenu /></Hide>   
<Show below="md"><MobileMenu /></Show>    

<Hide above="lg"><CompactView /></Hide>   
<Show above="lg"><FullView /></Show>      
```

Props: `above` | `below` (breakpoint-nøkkel: `sm` | `md` | `lg` | `xl` | `2xl`)

---

## Vanlige mønstre

### To-kolonne layout med sidebar

```tsx
<HGrid columns={{ xs: 1, lg: "280px 1fr" }} gap="space-32">
  <Box as="nav" padding="space-16">
    <Sidebar />
  </Box>
  <Box as="main">
    <Content />
  </Box>
</HGrid>
```

### Kort-grid

```tsx
<HGrid
  columns={{ xs: 1, sm: 2, xl: 3 }}
  gap={{ xs: "space-16", md: "space-24" }}
>
  {items.map((item) => (
    <Box key={item.id} padding="space-24" borderWidth="1" borderRadius="8">
      {item.content}
    </Box>
  ))}
</HGrid>
```

### Sentrert innhold med max-bredde

```tsx
// Med Page.Block
<Page.Block width="lg" gutters>
  <VStack gap="space-32">
    <Section />
    <Section />
  </VStack>
</Page.Block>

// Med Box og margin auto
<Box maxWidth="768px" marginInline="auto" paddingInline="space-24">
  <Content />
</Box>
```

### Mobil-first skjema-layout

```tsx
<VStack gap={{ xs: "space-16", md: "space-24" }}>
  <Stack direction={{ xs: "column", sm: "row" }} gap="space-16">
    <TextField label="Fornavn" />
    <TextField label="Etternavn" />
  </Stack>
  <TextField label="E-post" />
  <HStack justify="end" gap="space-8">
    <Button variant="secondary">Avbryt</Button>
    <Button>Lagre</Button>
  </HStack>
</VStack>
```

### Fullbredde-seksjon med bleed

```tsx
// Page.Block med gutters, men ett element som strekker seg full bredde
<Page.Block width="lg" gutters>
  <VStack gap="space-32">
    <Heading level="1" size="xlarge">
      Tittel
    </Heading>
    <Bleed marginInline="space-48">
      <FullWidthBanner />
    </Bleed>
    <BodyLong>Innhold</BodyLong>
  </VStack>
</Page.Block>
```

---

## v7 → v8 token-migrasjon

Tokens er ikke bakoverkompatible. Bruk codemod:

```bash
npx @navikt/aksel codemod v8-spacing-tokens ./src
```

| v7 (feil i v8) | v8 (korrekt) | px   |
| -------------- | ------------ | ---- |
| `space-4`      | `space-16`   | 16px |
| `space-8`      | `space-32`   | 32px |
| `space-2`      | `space-8`    | 8px  |
| `space-1`      | `space-4`    | 4px  |

> I v7 var tokenene multiplisert med 4. I v8 er det én-til-én med pikselverdi.
