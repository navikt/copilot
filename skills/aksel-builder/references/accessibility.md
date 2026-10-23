# Accessibility (universell utforming / uu)

Accessibility is non-negotiable. Follow the project's applicable WCAG requirements;
using Aksel alone does not establish page-level compliance. Preserve its built-in
behavior, supply labels and structure, and test the complete user flow. Many docs have a
uu section — read it with `aksel_get_doc` when accessible usage isn't obvious.

## Required props that activate accessibility

These props are how Aksel components become accessible. They're easy to omit because the
code still compiles — but the result is silently inaccessible. Confirm the exact prop names
per component with `aksel_get_component_info`; the common conventions are:

| Component kind                                            | Provide                                         | Why                                                          |
| --------------------------------------------------------- | ----------------------------------------------- | ------------------------------------------------------------ |
| Text inputs (`TextField`, `Textarea`, `Select`, `Search`) | `label` (and optional `description`)            | Programmatic label for screen readers                        |
| Choice groups (`RadioGroup`, `CheckboxGroup`, `Fieldset`) | `legend`                                        | Names the group                                              |
| Icon-only `Button`                                        | Prefer `title` on the icon                       | Gives the button an accessible name                          |
| `Modal` / `Dialog`                                        | a heading/title                                 | Names the dialog                                             |
| `FileUpload`                                              | label/description text                          | Names the upload control                                     |
| Meaningful icons                                          | `title`                                         | Gives the icon an accessible name (see [icons.md](icons.md)) |
| Decorative icons                                          | `aria-hidden`                                   | Removes noise from the accessibility tree                    |

```tsx
// ❌ Unlabelled
<TextField />
<Button icon={<TrashIcon />} />

// ✅ Labelled
<TextField label="Fødselsnummer" description="11 siffer" />
<Button icon={<TrashIcon aria-hidden />}>Slett</Button>          // text labels the button
<Button icon={<TrashIcon title="Slett" />} />
```

For a button without visible text, prefer the icon's `title` prop as shown above.
Do not hide that icon with `aria-hidden`: it supplies the button's name. When the button
has visible text, that text names the button and the icon is decorative
([icon accessibility](icons.md#icon-accessibility-decide-meaningful-or-decorative)).

## Don't double-label

Aksel form fields render their own `<label>`/`<legend>` from the `label`/`legend` props.
Wrapping them in another `<label>`, or adding a separate visual label you also pass as a
prop, double-labels for screen readers. Pass the prop and let the component render it.

## Errors and validation

- A field's `error` prop supplies error styling and connects the message to the input.
  It does not by itself guarantee an immediate screen-reader announcement. Use the
  documented form pattern and verify focus/announcement behavior.
- For forms, collect messages in `ErrorSummary` and link each entry to its field so users
  can jump to the problem. Move focus to the summary on submit.

## Semantic structure (your responsibility, not Aksel's)

Aksel styles text, but document structure is up to you:

- **Headings:** one `<h1>` per page; never skip levels (h1 → h2 → h3). Use `Heading` with
  the correct semantic `level` and a separate visual `size` so structure and appearance can
  differ:

  ```tsx
  <Heading level="1" size="large">Søknad</Heading>
  <Heading level="2" size="medium">Om deg</Heading>   // level 2 follows level 1
  ```

- **Landmarks:** ensure the page has `header` / `nav` / `main` / `footer` regions. Give multiple `nav`s distinct labels.
- **Lists:** use `<ul>`/`<ol>` (or `List`) for groups of related items, not stacked `div`s.

## Language

Follow the project's content language. For Norwegian content, set `<html lang="nb">`
or `<html lang="nn">` and use a matching `Provider` locale; English apps should use their
matching language and locale ([setup-and-imports.md](setup-and-imports.md)).

## Color & contrast

Use role tokens ([tokens-styling.md](tokens-styling.md)) and check the actual foreground/
background pairing in both themes; arbitrary token combinations can fail contrast.
Don't convey meaning by color alone; pair status
color with text or an icon. Never remove focus outlines; Aksel's `border-focus` token and
component focus styles are there for keyboard users.

## Quick a11y checklist

- [ ] Every input/group has a `label`/`legend`; nothing is double-labelled.
- [ ] Icon-only buttons have an accessible name (prefer the icon's `title`); decorative icons use `aria-hidden`.
- [ ] Headings start at `h1` and don't skip levels; `level` ≠ `size` where needed.
- [ ] Landmarks present (`main`, `nav`, etc.); lists use real list elements.
- [ ] Errors use the field `error` prop and/or `ErrorSummary` with focus management.
- [ ] Don't override Aksel's built-in ARIA; don't remove focus outlines.
- [ ] Keyboard flow, focus management, contrast, and screen-reader output verified where affected.
- [ ] Document `lang`, UI text, and `Provider` locale match the project's language.
