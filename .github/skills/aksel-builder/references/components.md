# Choosing & composing components

Pick a candidate from the tables below (a fast router), then **confirm its API with
`aksel_get_component_info` before non-trivial usage** — component APIs evolve, so don't
trust the table alone for props.

## Workflow for any component

1. **Pick** from the tables (or `aksel_find_docs({ query })` if it's not listed).
2. **Confirm props** with `aksel_get_component_info({ component })`.
3. **Read usage/when-to-use** with `aksel_get_doc({ path })` if the choice isn't obvious.
4. **Build**, then validate (a11y props, tokens, imports).

> Names move over time — some are new (`Dialog`, `LinkCard`, `GlobalAlert`, `LocalAlert`,
> `InfoCard`, `InlineMessage`), some carry prefixes (`UNSAFE_Combobox`), some are deprecated
> (`LinkPanel`, `Panel`, `Box.New`). Let MCP confirm the current name when unsure.

## Need → component

### Typography

| Need                   | Use                                                 |
| ---------------------- | --------------------------------------------------- |
| Page/section heading   | `Heading` (`size` + `level`)                        |
| Lead/intro paragraph   | `Ingress`                                           |
| Body text              | `BodyLong` (paragraphs) / `BodyShort` (single line) |
| Small print / metadata | `Detail`                                            |
| Form-ish label text    | `Label`                                             |
| Inline error text      | `ErrorMessage`                                      |

### Actions

| Need                              | Use                                                                               |
| --------------------------------- | --------------------------------------------------------------------------------- |
| Primary/secondary/tertiary action | `Button` (`variant`)                                                              |
| Destructive action                | `Button data-color="danger"` (see note)                                           |
| Icon-only action                  | `Button` with `icon` + accessible name (see [accessibility.md](accessibility.md)) |
| Copy-to-clipboard                 | `CopyButton`                                                                      |
| Navigation link                   | `Link`                                                                            |
| Menu of actions                   | `ActionMenu`                                                                      |
| Toggle between options            | `ToggleGroup`                                                                     |

> **v8 variant/color split:** emphasis lives on `variant`
> (`primary` / `secondary` / `tertiary`), color lives on `data-color`
> (`accent` default, `neutral`, `danger`). A destructive button is
> `<Button variant="primary" data-color="danger">`, **not** `variant="danger"` — that's the
> pre-v8 pattern ([migrations.md](migrations.md)). The same split applies to `Tag`,
> `Chips`, `ToggleGroup`, `Accordion`, and `Link`.

### Form inputs

| Need                        | Use                          |
| --------------------------- | ---------------------------- |
| Single-line text            | `TextField`                  |
| Multi-line text             | `Textarea`                   |
| Dropdown (native)           | `Select`                     |
| Autocomplete / typeahead    | `UNSAFE_Combobox`            |
| On/off toggle               | `Switch`                     |
| One of many (visible)       | `RadioGroup` + `Radio`       |
| Many of many                | `CheckboxGroup` + `Checkbox` |
| Search field                | `Search`                     |
| Date / month picking        | `DatePicker` / `MonthPicker` |
| File upload                 | `FileUpload`                 |
| Group related fields        | `Fieldset`                   |
| Confirm before submit       | `ConfirmationPanel`          |
| Summarize validation errors | `ErrorSummary`               |
| Review submitted answers    | `FormSummary`                |
| Multi-step form progress    | `FormProgress`               |

### Feedback & status

| Need                 | Use                                                                        |
| -------------------- | -------------------------------------------------------------------------- |
| Inline page alert    | `Alert` (or `LocalAlert` / `GlobalAlert` / `InfoCard` — verify which fits) |
| Short inline message | `InlineMessage`                                                            |
| Loading spinner      | `Loader`                                                                   |
| Determinate progress | `ProgressBar`                                                              |
| Loading placeholder  | `Skeleton`                                                                 |
| Status label / pill  | `Tag`                                                                      |
| Step indicator       | `Stepper`                                                                  |

### Overlays & disclosure

| Need                      | Use                                                 |
| ------------------------- | --------------------------------------------------- |
| Blocking dialog           | `Modal` or `Dialog` (verify which the project uses) |
| Contextual floating panel | `Popover`                                           |
| Hover/focus hint          | `Tooltip`                                           |
| Inline expand/collapse    | `ReadMore`, `Accordion`, or `ExpansionCard`         |
| Dropdown menu             | `Dropdown` / `ActionMenu`                           |
| Render outside DOM flow   | `Portal`                                            |

### Data & navigation

| Need                     | Use              |
| ------------------------ | ---------------- |
| Tabular data             | `Table`          |
| Tabs                     | `Tabs`           |
| Pagination               | `Pagination`     |
| Clickable card/link card | `LinkCard`       |
| Internal App header      | `InternalHeader` |
| Chips / removable tags   | `Chips`          |
| Timeline                 | `Timeline`       |
| Guidance panel           | `GuidePanel`     |
| Contextual help bubble   | `HelpText`       |

If nothing fits: compose from primitives ([primitives-layout.md](primitives-layout.md)) and
only then drop to custom elements — but search the docs first; Aksel covers more than this
table lists.

## Form patterns

Aksel form fields share a consistent, accessible API: a built-in label, optional
description, and an error string that wires up `aria` and validation styling for you.
**Confirm the exact prop names with `aksel_get_component_info`** — the shape below is the
common convention, but don't assume beyond what MCP shows.

```tsx
// Single field — label and error belong to the component (don't add a separate <label>)
<TextField
  label="E-postadresse"
  description="Vi bruker denne kun til å kontakte deg."
  error={errors.email}            // string shows the error state + message
  value={email}
  onChange={(e) => setEmail(e.target.value)}
/>

// Grouped choice inputs — the group owns the legend and the value
<RadioGroup legend="Velg kontaktmåte" value={method} onChange={setMethod}>
  <Radio value="email">E-post</Radio>
  <Radio value="sms">SMS</Radio>
</RadioGroup>

<CheckboxGroup legend="Hva ønsker du å motta?" value={list} onChange={setList}>
  <Checkbox value="news">Nyhetsbrev</Checkbox>
  <Checkbox value="status">Statusoppdateringer</Checkbox>
</CheckboxGroup>
```

Patterns to follow:

- **Let the component own the label.** Pass `label`/`legend`; don't wrap fields in your own
  `<label>` (that double-labels for screen readers).
- **Errors are strings on the field.** A non-empty `error` triggers the error state and
  announces the message. For form-level summaries, use `ErrorSummary` linked to each field.
- **Controlled inputs** use `value` + `onChange` together.
- **Group with `Fieldset`** when fields form one logical unit — it provides the legend and
  error wiring.

## Composition patterns

- Many overlays and menus are **compound components** (e.g. `Modal` with `Modal.Header` /
  `Modal.Body` / `Modal.Footer`; `ActionMenu` with trigger + content subparts). Read the
  component's doc to get the exact subcomponent names — don't guess them.
- **Put real action elements in footers.** Modal/Dialog footers expect `Button` elements,
  not bare click handlers.
- **Compose layout with primitives**, not ad-hoc `div`s with inline styles — a modal body
  laid out with `<VStack gap="space-16">` stays token-consistent.
