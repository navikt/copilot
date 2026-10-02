Legg til `POST /api/saker` som oppretter en sak med `tittel` og valgfri
`prioritet` (`normal` eller `hoy`). Returner 201 og saken med ny ID.
Avvis manglende eller blank tittel og ugyldig prioritet med 400, uten å
opprette en sak. Oversikten skal vise den nye saken etterpå.
