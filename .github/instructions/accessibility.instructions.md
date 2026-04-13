---
applyTo: "src/**/*.{tsx,jsx}"
---

# Accessibility (UU) Standards

Universell utforming er lovpålagt i Norge. All frontend-kode i Nav skal oppfylle WCAG 2.1 AA.

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

Overskriftsnivåer skal være logiske og uten hopp:

```tsx
// ✅ Korrekt — sammenhengende nivåer
<Heading size="large" level="1">Sidetittel</Heading>
  <Heading size="medium" level="2">Seksjon</Heading>
    <Heading size="small" level="3">Underseksjon</Heading>

// ❌ Feil — hopper fra h1 til h3
<Heading size="large" level="1">Sidetittel</Heading>
  <Heading size="small" level="3">Underseksjon</Heading>
```

## Skjemaer

```tsx
import { TextField, Select, Checkbox, ErrorSummary } from "@navikt/ds-react";

// ✅ Korrekt — Aksel-skjemaelementer har innebygd label-kobling
<TextField
  label="Fødselsnummer"
  description="11 siffer"
  error={errors.fnr}
  autoComplete="off"
/>

<Select label="Tema">
  <option value="">Velg tema</option>
  <option value="dagpenger">Dagpenger</option>
</Select>

// ✅ Feiloppsummering øverst i skjemaet
<ErrorSummary heading="Du må rette disse feilene før du kan sende inn">
  <ErrorSummary.Item href="#fnr">Fødselsnummer er påkrevd</ErrorSummary.Item>
</ErrorSummary>
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

## ARIA-attributter

Bruk kun ARIA når HTML-semantikk ikke er tilstrekkelig:

```tsx
// ✅ Navigasjonslandemerker
<nav aria-label="Brødsmulesti">...</nav>
<aside aria-label="Relatert innhold">...</aside>

// ✅ Live-regioner for dynamisk innhold
<Alert variant="success" role="status">
  Skjemaet ble sendt inn
</Alert>

// ✅ Expanding/collapsing
<Button
  aria-expanded={isOpen}
  aria-controls="panel-id"
  onClick={() => setIsOpen(!isOpen)}
>
  Vis detaljer
</Button>
<div id="panel-id" hidden={!isOpen}>
  Detaljert innhold
</div>

// ✅ Loading-tilstander
<Loader size="xlarge" title="Laster inn data" />
<div aria-busy={isLoading} aria-live="polite">
  {isLoading ? <Loader title="Laster" /> : <DataTable data={data} />}
</div>
```

## Fargekontrast

- **Tekst**: Minimum 4.5:1 kontrast mot bakgrunn (AA)
- **Stor tekst** (≥18px bold / ≥24px): Minimum 3:1
- **Ikke-tekst UI** (ikoner, knappekanter): Minimum 3:1
- **Bruk Aksel semantiske farger** — de oppfyller kontrastkrav automatisk
- **Aldri bruk farge alene** for å formidle informasjon — kombiner med ikon, tekst, eller mønster

## Keyboard-navigasjon

Alle interaktive elementer skal være tilgjengelige med tastatur:

- `Tab` / `Shift+Tab`: Naviger mellom elementer
- `Enter` / `Space`: Aktiver knapper og lenker
- `Escape`: Lukk modaler og menyer
- `Arrow keys`: Naviger i lister, tabs, og menyer

```tsx
// ✅ Fokusfelle i modal — Aksel Modal håndterer dette
<Modal open={isOpen} onClose={() => setIsOpen(false)} header={{ heading: "Bekreft sletting" }}>
  <Modal.Body>Er du sikker?</Modal.Body>
  <Modal.Footer>
    <Button onClick={handleDelete}>Slett</Button>
    <Button variant="secondary" onClick={() => setIsOpen(false)}>Avbryt</Button>
  </Modal.Footer>
</Modal>

// ✅ Skip-lenke (legg til øverst i layout)
<a href="#main-content" className="navds-sr-only navds-sr-only--focusable">
  Hopp til hovedinnhold
</a>
```

## Testing

```bash
# Automatisk a11y-sjekk med vitest-axe i eksisterende tester
npm install --save-dev vitest-axe

# Lighthouse accessibility audit
npx lighthouse http://localhost:3000 --only-categories=accessibility --output=json
```

```tsx
import { axe, toHaveNoViolations } from "vitest-axe";

expect.extend(toHaveNoViolations);

it("should have no accessibility violations", async () => {
  const { container } = render(<MyComponent />);
  const results = await axe(container);
  expect(results).toHaveNoViolations();
});
```

## Sjekkliste

Før du merger frontend-kode, verifiser:

- [ ] Heading-nivåer er logiske (h1 → h2 → h3, ingen hopp)
- [ ] Alle skjema-elementer har synlige labels
- [ ] Alle bilder har meningsfull `alt`-tekst eller `alt=""`
- [ ] Alle interaktive elementer har tilgjengelig navn
- [ ] Ingen informasjon formidles kun med farge
- [ ] Siden er fullt brukbar med kun tastatur
- [ ] Dynamisk innhold annonseres med `aria-live` eller `role="status"`
- [ ] Feilmeldinger er koblet til rett felt og oppsummert

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

## Related

| Resource | Use For |
|----------|---------|
| `@accessibility-agent` | Expert guidance on complex WCAG requirements |
| `@aksel-agent` | Aksel component patterns with built-in a11y |
| `playwright-testing` skill | E2E accessibility testing with axe-core |
