# 📦 Lag en agentpakke

Denne siden er oppskrifta: veien fra et tomt repo til at et annet team kjører
`nav-pilot install` og får innholdet ditt. [README.agentpakke.md](README.agentpakke.md) er
referansen som sier hva hvert felt betyr. Begynn her, slå opp der.

En agentpakke er et git-repo som sender AI-artefakter og en fil som beskriver dem. Det er
alt. Det er ingen registrering, ingen godkjenning og ingen sentral liste å komme inn på.

## 1. Legg innholdet der manifestet sier

```
ditt-repo/
├── .nav-pilot/
│   └── agentpakke.json
├── agents/
│   └── grillmester.agent.md
└── skills/
```

Katalognavnene er dine egne. `layout` i manifestet peker på dem, så heter de `innhold/agenter`
hos deg, sier du det der. `agents` og `skills` må begge finnes i `layout`, også når den ene
er tom.

## 2. Skriv manifestet

`.nav-pilot/agentpakke.json`, minste form som validerer:

```json
{
  "contractVersion": "1",
  "name": "ditt-team",
  "description": "Hva pakka er til for",
  "layout": {
    "agents": "agents",
    "skills": "skills"
  },
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester"]
    }
  }
}
```

`primaryAgents` er de agentene brukeren kan starte klienten som. Resten er underagenter som
bare kan kalles av andre.

## 3. Valider før du pusher

```bash
nav-pilot validate --source .
```

```
Validating: ditt-team@c3f7ca3

  ℹ manifest: .nav-pilot/agentpakke.json
  ℹ agentpakke: ditt-team (contract version 1)
  ℹ clients: copilot (tier 1)

✓ . conforms to the agentpakke contract.
```

Kjør den i CI også. Skjemaet er publisert, så du kan linte mot det uten nav-pilot:
`cli/nav-pilot/schemas/agentpakke-v1.json`.

## 4. La noen installere den

```bash
nav-pilot install ditt-team --source navikt/ditt-repo --repo
```

Det er hele distribusjonen. Den som installerer får en `.nav-pilot/agentpakke.lock.json` i
sitt eget repo, med kilden og revisjonen, og committer den. Da installerer hele teamet fra
samme revisjon, og `nav-pilot sync` flytter pinnen som en linje i en pull request.

## 5. Gjenbruk framfor å kopiere

Vil du bygge på plattformpakka uten å vedlikeholde en kopi av den, committer du den samme
erklæringa i ditt eget pakkerepo:

`.nav-pilot/agentpakke.lock.json`

```json
{
  "contractVersion": "1",
  "source": "navikt/copilot",
  "sha": "b73a69e0000000000000000000000000000000aa"
}
```

Repoet ditt sender nå både et manifest og en erklæring. Den som installerer pakka di får
begge pakkenes innhold. Sender begge en agent med samme navn, vinner din.

`sha` er påkrevd for en kilde på formen `owner/repo`. Uten den ville gjenbruken hentet det
main tilfeldigvis holdt, og to installasjoner en uke fra hverandre fikk ulikt innhold.

## Hva du kan sende

Seks typer: `agents`, `skills`, `instructions`, `prompts`, `hooks` og `extensions`.

Hooks og extensions er kjørbar kode, ikke tekst en modell leser. En hook kjører ved
verktøykall, en extension lastes av klienten. Den som installerer pakka di kjører koden din
på maskinen sin, så si i `description` hva den gjør.

## Når du fjerner noe

Slett aldri et artefakt uten å føre det opp i `.nav-pilot/retired-artifacts.json`. En kilde
hentes med `--depth 1`, så brukeren har ingen historikk å slå opp i, og en fil som bare
forsvinner blir liggende hos alle som installerte den. Generer lista og commit den. I
navikt/copilot gjør `mise run retired:generate` det, og `mise run retired:check` verifiserer
i CI at lista stemmer med historikken. Skriptet er rundt hundre linjer og kan kopieres.

## Se også

- [README.agentpakke.md](README.agentpakke.md), feltreferansen og kontrakten
- [README.nav-pilot.md](README.nav-pilot.md), hvordan brukerne installerer
