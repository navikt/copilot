# Effort-sveip for GPT-6.1 Sol på koding

Kjørt 12. oktober 2026 med Copilot CLI 1.0.95: `--suite coding --model gpt-6.1-sol`, Low og High, fem kjøringer per arm, via [`2026-10-12-effort-sol61.matrix`](../2026-10-12-effort-sol61.matrix). Bruksradene viser `gpt-6.1-sol` med riktig innsatsnivå i alle 192 kall, så ingen kjøring er forkastet. Max er ikke kjørt. Transkriptene ligger i [`transkripter/`](transkripter/). Stier til den midlertidige arbeidsmappa er byttet ut med `<arbeidsmappe>`.

Bakgrunn: /modeller sier «Vi har ikke målt effort selv ennå».

## Resultater

| Arm | Bestått | Median credits per kjøring | Spenn |
| --- | --- | --- | --- |
| Low | 30/30 | 25,69 | 23,75–26,83 |
| High | 30/30 | 27,85 | 27,36–28,67 |

Ingen kjøringer feilet. Totalt 266,21 credits (Low 126,97, High 139,24).

## Sammen med eldre målinger på samme testpakke

| Dato | Modell | Low | Medium | High |
| --- | --- | --- | --- | --- |
| 30. september | GPT-6 Sol | 30/30, 26,92 | 30/30, 27,77 | 30/30, 29,35 |
| 30. september | GPT-6.1 Sol | 30/30, 28,87 | | 30/30, 31,24 |
| 12. oktober | GPT-6.1 Sol | 30/30, 25,69 | | 30/30, 27,85 |
| 30. september | GPT-6 Luna | | 30/30, 1,65 | 30/30, 1,84 |
| 8. oktober | Claude Haiku 5.5 | 30/30, 1,57 | 30/30, 1,66 | |

Tallene er median credits per kjøring.

## Hva dette viser

- High gir ingen gevinst på `coding`: begge nivåene består alt, og High koster om lag 8 prosent mer. Det samme mønsteret står i målingen fra 30. september.
- Vi har altså målt effort selv, både før og nå. Setningen på /modeller er utdatert. På `planning` 30. september besto GPT-6 Sol High 14 av 20 mot 18 av 20 på Low, og på `review` besto Opus 5.5 alle 40 på Low, Medium og High.
- `coding` er mettet: hver modell og hvert nivå som er kjørt, har bestått 30 av 30. Testpakken kan derfor ikke vise at et høyere nivå hjelper, bare hva det koster.

## Hvorfor andre modeller ikke er kjørt

- GPT-6 Sol: Low, Medium og High er alt målt 30. september, alle 30/30.
- GPT-6 Luna og Claude Haiku 5.5: to nivåer hver er alt målt, alle 30/30, og prisforskjellen er under 0,2 credits per kjøring.
- Claude Opus 5.5 og Sonnet 5.5: med en mettet testpakke kan et sveip bare vise kostnad, ikke kvalitet. Det endrer ingen anbefaling og ville kostet om lag 250–500 credits.
