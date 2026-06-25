---
applyTo: ".github/workflows/*.{yml,yaml}"
---

# GitHub Actions CI/CD Standards

Standarder for CI/CD-workflows med GitHub Actions på Nais: SHA-pinning, Nais deploy og caching.

## Action Pinning

Pin all actions to full commit SHA, never just a major tag:

```yaml
# ✅ Correct — pinned to SHA
- uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
- uses: nais/deploy/actions/deploy@bf80eb8dba46797adb4909901e629bca8595a027 # v2

# ❌ Wrong — unpinned tag can be compromised
- uses: actions/checkout@v4
- uses: nais/deploy/actions/deploy@v2
```

## Minimal Permissions

Always set explicit permissions — never rely on defaults:

```yaml
permissions:
  contents: read       # Only read repo content
  id-token: write      # For OIDC/Nais deploy

# ❌ Wrong — overly broad permissions
permissions: write-all
```

## Nais Deploy Workflow

Standard deploy workflow for Nav apps:

```yaml
name: Build and deploy

on:
  push:
    branches: [main]

permissions:
  contents: read
  id-token: write

jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      image: ${{ steps.docker-build-push.outputs.image }}
    steps:
      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1

      - uses: nais/docker-build-push@v0
        id: docker-build-push
        with:
          team: mitt-team
          identity_provider: ${{ secrets.NAIS_WORKLOAD_IDENTITY_PROVIDER }}
          project_id: ${{ vars.NAIS_MANAGEMENT_PROJECT_ID }}

  deploy-dev:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
      - uses: nais/deploy/actions/deploy@v2
        env:
          CLUSTER: dev-gcp
          RESOURCE: .nais/nais.yaml
          VAR: image=${{ needs.build.outputs.image }}

  deploy-prod:
    needs: [build, deploy-dev]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
      - uses: nais/deploy/actions/deploy@v2
        env:
          CLUSTER: prod-gcp
          RESOURCE: .nais/nais.yaml
          VAR: image=${{ needs.build.outputs.image }}
```

## Caching

```yaml
# Gradle
- uses: actions/setup-java@v4
  with:
    distribution: temurin
    java-version: 21
    cache: gradle

# Node/pnpm
- uses: actions/setup-node@v4
  with:
    node-version: 22
    cache: pnpm

# Go
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
    cache: true
```

## Matrix Builds

```yaml
strategy:
  fail-fast: false
  matrix:
    app: [my-app, my-other-app]
steps:
  - run: cd apps/${{ matrix.app }} && ./gradlew build
```

## Reusable Workflows

```yaml
# .github/workflows/deploy-nais.yml (reusable)
on:
  workflow_call:
    inputs:
      cluster:
        required: true
        type: string
      resource:
        required: true
        type: string
      image:
        required: true
        type: string

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
      - uses: nais/deploy/actions/deploy@v2
        env:
          CLUSTER: ${{ inputs.cluster }}
          RESOURCE: ${{ inputs.resource }}
          VAR: image=${{ inputs.image }}
```

## Secrets

```yaml
# ✅ Korrekt — bruk GitHub Secrets
env:
  API_KEY: ${{ secrets.MY_API_KEY }}

# ❌ Feil — hardkodet hemmelighet
env:
  API_KEY: "sk-1234567890"

# ❌ Feil — logg hemmeligheter
- run: echo ${{ secrets.MY_API_KEY }}
```

## Workflow Security

```yaml
# Begrens concurrency — unngå parallelle deploys
concurrency:
  group: deploy-${{ github.ref }}
  cancel-in-progress: true

# Timeout — unngå hengende jobs
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 15
```

## Scanning

```yaml
# Vulnerability scanning med trivy
- uses: aquasecurity/trivy-action@0.28.0
  with:
    scan-type: fs
    severity: HIGH,CRITICAL
    exit-code: 1

# GitHub Actions security scanning
- run: pipx run zizmor .github/workflows/
```

## Boundaries

### ✅ Always

- Pin actions til SHA med kommentar for versjon
- Sett eksplisitte `permissions` per job
- Bruk `timeout-minutes` på alle jobs
- Bruk `concurrency` for deploy-workflows

### ⚠️ Ask First

- Nye secrets eller environment variables
- Endringer i deploy-rekkefølge (dev → prod)
- Nye reusable workflows

### 🚫 Never

- `permissions: write-all`
- Upinnede action-versjoner (`@v4`)
- Logg secrets i workflow-output
- `pull_request_target` med `actions/checkout` av PR-branch (code injection)
