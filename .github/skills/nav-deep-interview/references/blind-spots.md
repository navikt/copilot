# Vanlige blinde flekker i Nav-prosjekter

Basert på analyse av reelle Nav-repoer (dp-behandling, helse-spesialist, tiltakspenger, fp-sak) og vanlige feil som oppdages sent i utviklingsprosessen.

## Autentisering og autorisasjon

| Blinde flekk | Konsekvens | Spørsmål å stille |
|--------------|-----------|-------------------|
| Feil auth-mekanisme for caller-type | Token-validering feiler i prod | «Hvem kaller tjenesten — bruker via nettleser eller tjeneste-til-tjeneste?» |
| Azure client_credentials med brukerkontext | Mister bruker-audit trail, kan ikke spore hvem som gjorde hva | «Trenger du brukerens identitet nedover i kjeden?» |
| Manglende `accessPolicy.inbound` | Ingen kan kalle tjenesten, feiler stille | «Hvilke tjenester skal ha lov til å kalle deg?» |
| Manglende outbound-regler | Kan ikke kalle avhengigheter | «Hvilke tjenester kaller du, og i hvilket cluster?» |

## Database

| Blinde flekk | Konsekvens | Spørsmål å stille |
|--------------|-----------|-------------------|
| HikariCP default pool (10) | Pool exhaustion i containere med 2-4 replicas | «Hvor mange samtidige database-tilkoblinger trenger dere?» |
| Manglende indekser på foreign keys | Sakte queries ved JOIN, lås-eskalering | «Hvilke kolonner vil dere filtrere/joine på?» |
| Ingen retensjonsstrategi | Data vokser ubegrenset, GDPR-brudd | «Hvor lenge skal data lagres? Finnes det slettekrav?» |
| VARCHAR som primærnøkkel uten plan | Vanskelig å endre senere | «Hva er den naturlige identifikatoren? UUID eller domenespesifikk?» |

## Kafka og hendelser

| Blinde flekk | Konsekvens | Spørsmål å stille |
|--------------|-----------|-------------------|
| Ingen dead-letter-strategi | Poison pills stopper konsument | «Hva skjer med meldinger som ikke kan prosesseres?» |
| Manglende idempotens | Duplikate hendelser gir duplikate vedtak | «Kan tjenesten håndtere samme melding to ganger?» |
| Feil partisjonering | Rekkefølge-garanti brytes | «Er rekkefølgen på hendelser viktig?» |
| Manglende schema-evolusjon | Konsumenter brekker ved endring | «Hvordan håndterer dere endringer i hendelsesformat?» |

## Observerbarhet

| Blinde flekk | Konsekvens | Spørsmål å stille |
|--------------|-----------|-------------------|
| Kun tekniske metrikker | Vet ikke om forretningslogikken fungerer | «Hvilke forretningsmetrikker viser at tjenesten gjør jobben sin?» |
| Manglende correlation ID | Kan ikke spore forespørsler på tvers av tjenester | «Propagerer dere callId/correlationId?» |
| Logging av PII | GDPR-brudd, Personvernombudet tar kontakt | «Hva logges? Er fnr/navn filtrert ut?» |
| Ingen alerting | Oppdager feil først når brukere klager | «Hvem skal varsles, og ved hvilke terskelverdier?» |

## Frontend

| Blinde flekk | Konsekvens | Spørsmål å stille |
|--------------|-----------|-------------------|
| Manglende universell utforming | Lovbrudd (likestillingsloven) | «Er UU-krav ivaretatt? Bruker dere Aksel-komponenter?» |
| Direkte API-kall fra klient | CORS-problemer, token-eksponering | «Bruker dere BFF-mønster med server-side proxy?» |
| Tailwind p-/m- i stedet for Aksel tokens | Inkonsistent design, vanskelig vedlikehold | «Bruker dere Aksel spacing-tokens (Box, VStack)?» |
| Manglende error boundaries | Hvit side ved feil | «Hva ser brukeren når noe feiler?» |

## Sikkerhet

| Blinde flekk | Konsekvens | Spørsmål å stille |
|--------------|-----------|-------------------|
| SQL string concatenation | SQL injection | «Er alle database-queries parameteriserte?» |
| CORS `*` | Cross-site request forgery | «Hvilke domener skal ha CORS-tilgang?» |
| Manglende input-validering | Injection, crash | «Valideres all ekstern input?» |
| Secret i kode/config | Eksponert hemmelighet | «Hvor lagres secrets? Bruker dere Nais secrets/Vault?» |
