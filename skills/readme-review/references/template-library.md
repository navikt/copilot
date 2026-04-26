# Template: Library / Package

README template for a shared library or package published to npm, Maven, or similar. Replace `{placeholders}` with actual values.

---

# {Library Name}

{One-sentence description of what this library does — under 120 characters.}

[![CI]({ci-badge-url})]({ci-url})
[![License: {license}]({license-badge-url})]({license-url})

## Installasjon

```bash
# npm
npm install {package-name}

# yarn
yarn add {package-name}
```

{Or for JVM:}

```kotlin
dependencies {
    implementation("{group-id}:{artifact-id}:{version}")
}
```

## Bruk

```{language}
import { {mainExport} } from '{package-name}'

{minimal-working-example}
```

## API

### `{functionName}({params})`

{Description of what it does.}

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| {name} | {type} | {description} |

**Returns:** {return type and description}

{Repeat for main exports. For large APIs, link to generated docs.}

## Kompatibilitet

| Platform / Runtime | Version |
|---|---|
| {Node.js} | {≥ 18} |
| {TypeScript} | {≥ 5.0} |

## Eksempler

### {Use case 1}

```{language}
{code example}
```

### {Use case 2}

```{language}
{code example}
```

{For more examples, see [`examples/`](examples/).}

## Utvikling

```bash
git clone https://github.com/navikt/{library-name}
cd {library-name}
{install-command}
{test-command}
{build-command}
```

## Bidra

1. Fork og klon repoet
2. Lag branch fra `main`
3. Kjør `{check-command}` før du pusher
4. Opprett PR

Se [CONTRIBUTING.md](CONTRIBUTING.md) for detaljer.

## Lisens

[{License}](LICENSE)
