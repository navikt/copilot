---
applyTo: "src/**/*.{tsx,jsx}"
---

# Accessibility (UU) Standards

Universell utforming er lovpålagt i Norge. All frontend-kode i Nav skal oppfylle WCAG 2.1 AA.

> For comprehensive WCAG tables, Aksel component patterns, and manual testing checklists, use `@accessibility-agent`. This instruction covers the essential code rules applied automatically when editing React/TSX files.

## Aksel-komponenter har innebygd UU

Aksel-komponenter (`@navikt/ds-react`) håndterer mange a11y-krav automatisk:

- Korrekt rolle/aria-attributter
- Keyboard-navigasjon
- Fokus-håndtering
- Fargekontrast

**Bruk alltid Aksel-komponenter fremfor egne `<div>`/`<button>`-løsninger.**

## Semantisk HTML

```tsx
// ✅ Korrekt — semantiske elementer
<main>
  <nav aria-label="Hovednavigasjon">...</nav>
  <article>
    <Heading size="large" level="1">Tittel</Heading>
    <section aria-labelledby="seksjon-id">...</section>
  </article>
</main>

// ❌ Feil — div-suppe uten semantikk
<div className="main">
  <div className="nav">...</div>
  <div className="content">
    <div className="title">Tittel</div>
  </div>
</div>
```

## Heading-hierarki

```tsx
// ✅ h1 → h2 → h3, ingen hopp
<Heading size="large" level="1">Sidetittel</Heading>
  <Heading size="medium" level="2">Seksjon</Heading>

// ❌ Hopper fra h1 til h3
<Heading size="large" level="1">Sidetittel</Heading>
  <Heading size="small" level="3">Underseksjon</Heading>
```

## Bilder og ikoner

```tsx
// ✅ Meningsbærende bilder — alt-tekst som beskriver innholdet
<img src="/chart.png" alt="Bruksstatistikk siste 30 dager: 450 aktive brukere" />

// ✅ Dekorative bilder — tom alt, eller aria-hidden
<img src="/decoration.svg" alt="" />
<DecorativeIcon aria-hidden="true" />

// ✅ Ikoner med mening — bruk title eller sr-only tekst
<Button variant="tertiary" icon={<TrashIcon title="Slett element" />} />

// ❌ Feil — ikonknapp uten tilgjengelig navn
<Button variant="tertiary" icon={<TrashIcon />} />
```

## Interaktive elementer

```tsx
// ✅ Korrekt — synlig fokusindikator, tilgjengelig navn
<Button variant="primary">Send inn</Button>
<Link href="/oversikt">Gå til oversikt</Link>

// ✅ Korrekt — lenkebeskrivelse med kontekst
<Link href={`/sak/${id}`}>
  Se detaljer for sak {saksnummer}
</Link>

// ❌ Feil — generisk lenketekst
<Link href={`/sak/${id}`}>Klikk her</Link>
<Link href={`/sak/${id}`}>Les mer</Link>

// ❌ Feil — onClick på div uten rolle/keyboard
<div onClick={handleClick}>Klikk meg</div>
```

## ARIA — kun når nødvendig

```tsx
// ✅ Live-regioner for dynamisk innhold
<Alert variant="success" role="status">
  Skjemaet ble sendt inn
</Alert>

// ✅ Loading-tilstand
<div aria-busy={isLoading} aria-live="polite">
  {isLoading ? <Loader title="Laster" /> : <DataTable data={data} />}
</div>

// ✅ Expanding/collapsing
<Button aria-expanded={isOpen} aria-controls="panel-id">
  Vis detaljer
</Button>
```

## Fargekontrast

- **Tekst**: 4.5:1 (AA), **stor tekst**: 3:1
- **UI-komponenter**: 3:1
- Bruk Aksel semantiske farger — de oppfyller kontrastkrav
- **Aldri farge alene** for å formidle informasjon

## Testing

```tsx
import { axe, toHaveNoViolations } from "vitest-axe";

expect.extend(toHaveNoViolations);

it("should have no accessibility violations", async () => {
  const { container } = render(<MyComponent />);
  const results = await axe(container);
  expect(results).toHaveNoViolations();
});
```

## Boundaries

### ✅ Always

- Bruk Aksel-komponenter — de har innebygd a11y
- Test med tastatur (Tab gjennom hele siden)
- Sjekk heading-hierarki

### ⚠️ Ask First

- Custom ARIA-roller utover standard HTML-semantikk
- Avvik fra Aksel-mønster for tilgjengelighet

### 🚫 Never

- `<div onClick>` uten `role="button"` og `tabIndex`
- Ikonknapper uten tilgjengelig navn (title eller sr-only tekst)
- Fjern fokus-indikator (`outline: none`) uten erstatning
- `tabIndex` > 0

## Related

| Resource | Use For |
|----------|---------|
| `@accessibility-agent` | Expert guidance on complex WCAG requirements |
| `@aksel-agent` | Aksel component patterns with built-in a11y |
| `playwright-testing` skill | E2E accessibility testing with axe-core |
