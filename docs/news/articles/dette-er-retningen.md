---
title: "Dette er retningen, og du trenger ikke kunne alt for å henge med"
date: 2026-10-12
category: nav
featured: true
excerpt: "Du trenger ikke lære alt for å bruke nav-pilot. Men du må ta stilling til når KI hjelper teamet ditt og produktet ditt, og når det ikke gjør det."
---

Det skjer mye innen KI, og vi deler mye av det. Det kan gi inntrykk av at du må lære alt for å bruke nav-pilot, og at Nav er på vei mot KI overalt. Ingen av delene stemmer. Men du må ta stilling.

nav-pilot er oppsettet vi i Nav har laget for å bruke KI i produktutvikling på en trygg måte. Det kommer ferdig satt opp. Du har god tid til å lære deg det, og du trenger ikke lære alt på en gang. Modellene, agentene og skillsene under panseret er fordypning for dem som vil.

Det vi forventer, er at du tar stilling. Prøv nav-pilot på reelle oppgaver i ditt eget produkt, og gjør deg opp en mening om når det hjelper og når det ikke gjør det. «Her passer det ikke å bruke KI, fordi …» er et helt greit svar. «Jeg har ikke sett på det» er det ikke. KI kommer til å påvirke produktene vi lager uansett, og som teknolog må du kunne vurdere verktøyet på samme måte som du vurderer et rammeverk eller andre utviklerverktøy.

## Retningen er ikke «KI overalt»

I utkastet til mål for 2027 har teknologiledelsen i Nav formulert et fokusområde for å ta i bruk kunstig intelligens der den gir størst nytte, innenfor akseptabel risiko. De to andre fokusområdene er produktutvikling og internkontroll. Det er altså ikke et mål at alle skal bruke KI til alt, og det er ikke et mål at hver enkelt skal kunne oppsettet i dybden. Målet om en felles grunnmur for KI og agenter handler om det motsatte: at Nav bygger noe felles, slik at du slipper å bygge det selv. nav-pilot er en del av den grunnmuren.

Hva «akseptabel risiko» betyr for deg, står i [retningslinjene](/retningslinjer): «Du kan bruke Copilot i alle GitHub-prosjekter som ikke inneholder personopplysninger eller skjermingsverdig informasjon. Kildekode inneholder normalt ikke personopplysninger. Personvern avklares i egne prosesser, uavhengig av Copilot.»

## Vi bommer også

Vi deler mye om nye modeller, verktøy og måter å jobbe på, både her og i [#github-copilot på Slack](https://nav-it.slack.com/archives/C055TNXBM17). Det kan gi inntrykk av at alle andre har skjønt det og bruker KI til alt. Men det meste som deles, er eksperimenter, ikke ting du må kunne.

Vi som lager oppsettet, prøver oss også fram. Vi integrerte RTK i nav-pilot for å redusere tokenbruk. Så oppdaget vi at det kunne skrive om kommandoer på uheldige måter, og i ett tilfelle fikk det agenten til å gjenta en full typesjekk helt til løkkevernet stoppet den ([#915](https://github.com/navikt/copilot/pull/915), [#1332](https://github.com/navikt/copilot/pull/1332)). Vi har fjernet integrasjonen. Når noe ikke fungerer, kan problemet ligge i det vi har levert, ikke i at du har skjønt for lite.

## Det er ikke bare deg

Hvis du opplever at KI både hjelper og lager merarbeid, er du ikke alene. I [Stack Overflows utviklerundersøkelse fra 2025](https://survey.stackoverflow.co/2025/ai) svarer de aller fleste at de bruker eller planlegger å bruke KI-verktøy. Samtidig svarer 66 % at de er frustrert over løsninger som nesten stemmer. Undersøkelsen handler om utviklere generelt, ikke om Nav, men den viser at begge deler kan være sant på én gang.

## Kvote og måling

Hver utvikler har en kvote på 800 USD per måned. Den har vært 250 og 400, og den kommer til å endre seg igjen, også nedover, for at vi skal holde oss innenfor budsjett. Kvoten er et tak, ikke et mål.

Vi samler inn mye data om bruken, men den er teknisk: hvilke modeller som brukes, hvor mange tokens som går med og hva det koster. Tallene finnes per person, og du kan se ditt eget forbruk på [din side](/abonnement). Arbeidsgruppen for KI i produktutvikling bruker dem til å forbedre nav-pilot. RTK-eksempelet over er et resultat av akkurat det: vi så tokenforbruk som ikke ga mening, fant årsaken og fjernet den.

Forbruket per team er åpent for alle i Nav under [Teaminnsikt](/innsikt/team). Det er med vilje. Team i Nav har ansvar og myndighet for sine egne valg, og KI koster penger på lik linje med apper og databaser i Nais. Da bør KI-bruken også være synlig, slik at hvert team kan svare for sin egen: står forbruket i et rimelig forhold til det vi leverer?

Teknologidirektøren har vært tydelig på at KI-bruk i seg selv ikke er et mål. Vi i arbeidsgruppen for KI i produktutvikling mener det samme, og det er slik vi leser lista: den er til for transparens og ansvar, ikke for å rangere. Et tall alene sier ingenting. Et team med høyt forbruk kan ha automatisert mye, og et team med lavt forbruk kan ha vurdert at KI ikke hjelper dem akkurat nå. Begge deler er greit, så lenge det er et valg teamet har tatt. Opplever du at lista blir brukt på en annen måte, vil vi vite det.

Det vi ikke måler, er verdien for brukerne. Vi vet hva bruken koster, men ikke hva den gir, og det har vi fått kritikk for: vi gir alle like mye uten å vite hvordan det brukes. Det er et bevisst valg. Vi har ikke funnet en måte å måle verdien av KI i et produkt som er bedre enn vurderingen til teamet som eier produktet. Den åpne lista er vår måte å flytte det spørsmålet dit det hører hjemme.

## Noen ting vi vil være tydelige på

- Du må ta stilling til om og hvordan KI kan gi verdi for teamet ditt og produktet ditt. Å ha prøvd og valgt bort er like gyldig som å bruke det daglig. Det eneste vi ikke vil, er at du lar være å finne det ut.
- Du trenger ikke forstå hele oppsettet for å komme i gang. Poenget med nav-pilot er at du skal slippe å sette sammen delene selv. Vi har til og med forenklet det underveis: før måtte du velge mellom fem samlinger som viste seg å være nesten like, nå er det én pakke. Vil du lære mer om modeller, agenter og skills, er det bare å spørre, men du må ikke.
- Målet er ikke mest mulig KI-bruk. Målet, slik også ledelsen har formulert det, er å bruke KI der det gir verdi. Noen ganger hjelper det. Andre ganger er det enklere å gjøre jobben uten, og den vurderingen blir bedre jo mer du har prøvd.
- Er du skeptisk? Si det. Vi trenger å høre om det som ikke fungerer, og du trenger ikke ha et forslag til løsning for å ta opp en bekymring. Innspill i #github-copilot går til arbeidsgruppen.
- KI erstatter ikke faglige vurderinger eller samarbeid. Vi må fortsatt forstå og stå inne for det vi leverer.

## Start med én oppgave

Hvis du ikke har prøvd ennå, velg én liten og avgrenset oppgave. Det er lettere å vurdere om verktøyet hjalp når du vet hva du ville ha gjort uten det. Noen eksempler:

- Be om en forklaring av et stykke kode du ikke har skrevet selv, før du begynner å endre i det.
- Få skrevet et utkast til tester for en funksjon du allerede har, og se hva som mangler.
- Lim inn en feilmelding og be om tre mulige årsaker.
- Be om et utkast til en PR-beskrivelse ut fra diffen, og rett opp det som er feil.

Gjør det gjerne sammen med en kollega. Ble jobben bedre eller enklere? Hva måtte dere rette opp? Dere trenger ikke legge om hele arbeidsformen for å finne ut om noe er nyttig.

Det fungerer også som team: sett av en time der teknologer og fagansvarlige tar én problemstilling fra produktet og utforsker den sammen med KI. Da får dere både den faglige og den tekniske vurderingen i samme rom.

## Spør oss

Still spørsmål i #github-copilot på Slack, eller kom på Åpent KI-kontor hver onsdag kl. 09.00 på Teams, med påmelding i Delta. Grunnleggende spørsmål, innvendinger og erfaringer med det som ikke fungerer, er like velkomne som gode eksempler.

Fortsett å være nysgjerrig, og bruk KI der du har funnet ut at det gir deg, teamet ditt og produktet ditt verdi.
