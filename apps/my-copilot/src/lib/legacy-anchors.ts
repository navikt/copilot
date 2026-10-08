// Anchors that have moved: old "path#anchor" to new "path#anchor".
//
// A published anchor must keep working. When one moves or goes away, add an
// entry here. The key's path is the page the browser lands on after any
// redirect in next.config.ts. The test does not follow a redirect() inside a
// page. Write ø and å as-is, not percent-encoded. HashAnchorScroll sends the
// reader on to the target. src/link-inventory.test.ts checks that every
// target exists and that no key is a real id on its page. A value must be a
// real id on its page, never another key. The link guard rejects chains.
export const LEGACY_ANCHORS: Record<string, string> = {
  // /nav-pilot/agentpakker was split into a how-to and a reference page.
  "/nav-pilot/agentpakker#artefakttyper": "/nav-pilot/agentpakker/referanse#artefakttyper",
  "/nav-pilot/agentpakker#kjorbar-kode": "/nav-pilot/agentpakker/referanse#kjorbar-kode",
  "/nav-pilot/agentpakker#skript-i-en-skill": "/nav-pilot/agentpakker/referanse#skript-i-en-skill",
  "/nav-pilot/agentpakker#klientoppforinga": "/nav-pilot/agentpakker/referanse#klientoppforinga",
  "/nav-pilot/agentpakker#hvilken-tier": "/nav-pilot/agentpakker/referanse#hvilken-tier",
  "/nav-pilot/agentpakker#uten-agent": "/nav-pilot/agentpakker/referanse#uten-agent",
  "/nav-pilot/agentpakker#mcp-servere": "/nav-pilot/agentpakker/referanse#mcp-servere",
  "/nav-pilot/agentpakker#sandkasse": "/nav-pilot/agentpakker/referanse#sandkasse",
  "/nav-pilot/agentpakker#kollisjoner": "/nav-pilot/agentpakker/referanse#kollisjoner",
  "/nav-pilot/agentpakker#hva-som-komponerer": "/nav-pilot/agentpakker/referanse#hva-som-komponerer",
  "/nav-pilot/agentpakker#hold-basen-oppdatert": "/nav-pilot/agentpakker/referanse#hold-basen-oppdatert",
  "/nav-pilot/agentpakker#nar-endringen-nar-fram": "/nav-pilot/agentpakker/referanse#nar-endringen-nar-fram",
  "/nav-pilot/agentpakker#stabile-releases": "/nav-pilot/agentpakker/referanse#stabile-releases",
  "/nav-pilot/agentpakker#pensjonering": "/nav-pilot/agentpakker/referanse#pensjonering",
  "/statistikk#team": "/innsikt/team#teamkostnad",
  // /nav-pilot/docs was split into guides, reference and explanation pages
  // (docs/nav-pilot-dokumentasjon-forslag.md §1.4). next.config.ts sends the
  // page to /nav-pilot/referanse, so the old anchors are keyed there.
  // #filstruktur and #lenker still exist there. The client sections moved on to
  // /nav-pilot/klienter (§4, PR 7).
  "/nav-pilot/referanse#agentpakke": "/nav-pilot/agentpakker#pakkene-som-finnes",
  "/nav-pilot/referanse#arkitektur": "/nav-pilot/forklaring/arkitektur#arkitektur",
  "/nav-pilot/referanse#automatisk-sync": "/nav-pilot/guider/synkronisere#automatisk-sync",
  "/nav-pilot/referanse#brukerundersokelser": "/nav-pilot/forklaring/personvern#brukerundersokelser",
  "/nav-pilot/referanse#brukerundersøkelser": "/nav-pilot/forklaring/personvern#brukerundersokelser",
  "/nav-pilot/referanse#bytte-modell": "/nav-pilot/guider/lokal#bytte-lokal-modell",
  "/nav-pilot/referanse#cli-referanse": "/nav-pilot/referanse#kommandoer",
  "/nav-pilot/referanse#collections": "/nav-pilot/agentpakker#pakkene-som-finnes",
  "/nav-pilot/referanse#cplt-sikkerhetsniva": "/nav-pilot/forklaring/sandkassen#sikkerhetsniva",
  "/nav-pilot/referanse#de-fire-fasene": "/nav-pilot/forklaring/planlegging#fire-faser",
  "/nav-pilot/referanse#demo-i-praksis": "/nav-pilot/forklaring/planlegging#demo-i-praksis",
  "/nav-pilot/referanse#designprinsipper": "/nav-pilot/forklaring/arkitektur#designprinsipper",
  "/nav-pilot/referanse#faq": "/nav-pilot/guider/synkronisere#faq",
  "/nav-pilot/referanse#fire-faser": "/nav-pilot/forklaring/planlegging#fire-faser",
  "/nav-pilot/referanse#gronn-rod-sone": "/nav-pilot/forklaring/planlegging#gronn-rod-sone",
  "/nav-pilot/referanse#grønn-og-rød-sone": "/nav-pilot/forklaring/planlegging#gronn-rod-sone",
  "/nav-pilot/referanse#hva-den-klarer": "/nav-pilot/forklaring/lokal-modell#malte-grenser",
  "/nav-pilot/referanse#hva-er-nav-pilot": "/kom-i-gang#hva-er-nav-pilot",
  "/nav-pilot/referanse#hva-nav-pilot-vet": "/nav-pilot/forklaring/arkitektur#hva-nav-pilot-vet",
  "/nav-pilot/referanse#hva-nav-pilot-vet-som-copilot-ikke-vet": "/nav-pilot/forklaring/arkitektur#hva-nav-pilot-vet",
  "/nav-pilot/referanse#hvor-installere": "/nav-pilot/guider/installere-og-oppgradere#velg-installasjonssted",
  "/nav-pilot/referanse#hvor-mye-som-sendes": "/nav-pilot/guider/lokal#utsending",
  "/nav-pilot/referanse#hvor-skal-artefaktene-installeres":
    "/nav-pilot/guider/installere-og-oppgradere#velg-installasjonssted",
  "/nav-pilot/referanse#hvorfor-nav-pilot": "/nav-pilot/forklaring/arkitektur#hvorfor",
  "/nav-pilot/referanse#ignorere-enkeltkomponenter": "/nav-pilot/guider/tilpasse#ignorere-enkeltkomponenter",
  "/nav-pilot/referanse#installasjon": "/kom-i-gang#installer",
  "/nav-pilot/referanse#installasjon-5-min": "/kom-i-gang#installer",
  "/nav-pilot/referanse#installer-cli": "/kom-i-gang#installer",
  "/nav-pilot/referanse#introduksjon": "/kom-i-gang#hva-er-nav-pilot",
  "/nav-pilot/referanse#isolasjon-er-pakrevd": "/nav-pilot/forklaring/sandkassen#isolasjon-er-pakrevd",
  "/nav-pilot/referanse#isolasjon-er-påkrevd-på-nav-utstyr": "/nav-pilot/forklaring/sandkassen#isolasjon-er-pakrevd",
  "/nav-pilot/referanse#klienter": "/nav-pilot/klienter#stotte-klienter",
  "/nav-pilot/referanse#klienter-og-konfig": "/nav-pilot/klienter#stotte-klienter",
  "/nav-pilot/referanse#klienter-og-konfigurasjon": "/nav-pilot/klienter#stotte-klienter",
  "/nav-pilot/referanse#kom-i-gang": "/kom-i-gang#installer",
  "/nav-pilot/referanse#kommandooversikt": "/nav-pilot/referanse#kommandoer",
  "/nav-pilot/referanse#kompetansebevaring": "/nav-pilot/forklaring/planlegging#kompetansebevaring",
  "/nav-pilot/referanse#konfig-nokler": "/nav-pilot/referanse#konfignokler",
  "/nav-pilot/referanse#konfigurasjon": "/nav-pilot/guider/tilpasse#endre-innstillinger",
  "/nav-pilot/referanse#konfigurasjonsnøkler": "/nav-pilot/referanse#konfignokler",
  "/nav-pilot/referanse#logging-av-blokkeringer": "/nav-pilot/guider/feilsoking#blokkeringer",
  "/nav-pilot/referanse#lokal-decide": "/nav-pilot/lokal/decide#start-serveren",
  "/nav-pilot/referanse#lokal-decide-oppskrifter": "/nav-pilot/guider/lokal#decide-oppskrifter",
  "/nav-pilot/referanse#lokal-egen-server": "/nav-pilot/lokal/egen-server#start-serveren",
  "/nav-pilot/referanse#lokal-feilsoking": "/nav-pilot/guider/feilsoking#lokal",
  "/nav-pilot/referanse#lokal-hva-den-klarer": "/nav-pilot/forklaring/lokal-modell#malte-grenser",
  "/nav-pilot/referanse#lokal-kom-i-gang": "/nav-pilot/lokal#installer",
  "/nav-pilot/referanse#lokal-modell": "/nav-pilot/lokal#hva-du-far",
  "/nav-pilot/referanse#lokal-modeller": "/nav-pilot/referanse#lokale-modeller",
  "/nav-pilot/referanse#lokal-sync": "/nav-pilot/guider/synkronisere#lokal-sync",
  "/nav-pilot/referanse#lokal-utsending": "/nav-pilot/guider/lokal#utsending",
  "/nav-pilot/referanse#modeller-i-alfa": "/nav-pilot/referanse#lokale-modeller",
  "/nav-pilot/referanse#nar-strict-ikke-anbefales": "/nav-pilot/forklaring/sandkassen#nar-strict-ikke-anbefales",
  "/nav-pilot/referanse#når-noe-henger": "/nav-pilot/guider/feilsoking#lokal",
  "/nav-pilot/referanse#når-strict-ikke-anbefales": "/nav-pilot/forklaring/sandkassen#nar-strict-ikke-anbefales",
  "/nav-pilot/referanse#oppgrader-cli": "/nav-pilot/guider/installere-og-oppgradere#oppgradere",
  "/nav-pilot/referanse#overstyre-installerte-filer": "/nav-pilot/guider/tilpasse#overstyre-installerte-filer",
  "/nav-pilot/referanse#personvern": "/nav-pilot/forklaring/personvern#hva-som-males",
  "/nav-pilot/referanse#personvern-og-telemetri": "/nav-pilot/forklaring/personvern#hva-som-males",
  "/nav-pilot/referanse#planleggingspipelinen": "/nav-pilot/forklaring/planlegging#planleggingspipelinen",
  "/nav-pilot/referanse#planning-skills": "/nav-pilot/forklaring/planlegging#planning-skills",
  "/nav-pilot/referanse#prosjektkontekst-med-nav-pilot-init":
    "/nav-pilot/guider/tilpasse#prosjektkontekst-med-nav-pilot-init",
  "/nav-pilot/referanse#ressurser": "/nav-pilot/referanse#lenker",
  "/nav-pilot/referanse#sikkerhetsnivå-i-cplt": "/nav-pilot/forklaring/sandkassen#sikkerhetsniva",
  "/nav-pilot/referanse#skills-i-detalj": "/nav-pilot/forklaring/planlegging#skills-i-detalj",
  "/nav-pilot/referanse#slik-fungerer-det": "/nav-pilot/referanse#filstruktur",
  "/nav-pilot/referanse#opencode": "/nav-pilot/klienter#opencode",
  "/nav-pilot/referanse#stotte-klienter": "/nav-pilot/klienter#stotte-klienter",
  "/nav-pilot/referanse#støttede-klienter": "/nav-pilot/klienter#stotte-klienter",
  "/nav-pilot/referanse#sync-faq": "/nav-pilot/guider/synkronisere#faq",
  "/nav-pilot/referanse#sync-og-oppdatering": "/nav-pilot/guider/synkronisere#automatisk-sync",
  "/nav-pilot/referanse#team-egne-instruksjoner": "/nav-pilot/guider/tilpasse#team-egne-instruksjoner",
  "/nav-pilot/referanse#tilpasning": "/nav-pilot/guider/tilpasse#team-egne-instruksjoner",
  "/nav-pilot/referanse#tilpasse-sync": "/nav-pilot/guider/synkronisere#tilpasse-sync",
  "/nav-pilot/referanse#vanlige-oppgaver": "/nav-pilot/guider/installere-og-oppgradere#vanlige-oppgaver",

  // /nav-pilot/lokal became the introduction for Mac (§3). #kom-i-gang and
  // #hva-du-far still exist there as wrappers, and #lenker is at the bottom.
  "/nav-pilot/lokal#egen-server": "/nav-pilot/lokal/egen-server#start-serveren",
  "/nav-pilot/lokal#hva-kommer": "/nav-pilot/forklaring/lokal-modell#hva-kommer",
  "/nav-pilot/lokal#malt": "/nav-pilot/forklaring/lokal-modell#malte-grenser",
  "/nav-pilot/lokal#malt-decide": "/innsikt/lokale-modeller#malt-decide",
  "/nav-pilot/lokal#malt-utsending": "/innsikt/lokale-modeller#malt-delegering",
  "/nav-pilot/lokal#utsending": "/nav-pilot/forklaring/lokal-modell#utsending",

  // The measured numbers moved to their own page.
  "/nav-pilot/forklaring/lokal-modell#malt-decide": "/innsikt/lokale-modeller#malt-decide",
  "/nav-pilot/forklaring/lokal-modell#malt-utsending": "/innsikt/lokale-modeller#malt-delegering",

  // The measurements page moved to /innsikt/lokale-modeller (next.config.ts),
  // and "utsending" became "delegering".
  "/innsikt/lokale-modeller#utsendingsnivaer": "/innsikt/lokale-modeller#delegeringsnivaer",
  "/innsikt/lokale-modeller#malt-utsending": "/innsikt/lokale-modeller#malt-delegering",

  // Own server got its own introduction.
  "/nav-pilot/guider/lokal#egen-server": "/nav-pilot/lokal/egen-server#start-serveren",

  // The worktree guide assumes an up-to-date cplt: the version check and its
  // "unknown config key" entry went. The generic entry covers the latter.
  "/nav-pilot/guider/worktrees#forutsetninger": "/nav-pilot/guider/worktrees#hvorfor",
  "/nav-pilot/guider/worktrees#unknown-config-key": "/nav-pilot/guider/cplt-feilmeldinger#unknown-config-key",

  // /cplt was Norwegian for a while (#1144) and is English again: it is the
  // landing page for the cplt open-source project.
  "/cplt#apen-kildekode": "/cplt#open-source",
  "/cplt#felles-konfig": "/cplt#team-config",
  "/cplt#innstillinger": "/cplt#configuration",
  "/cplt#installer": "/cplt#install",
  "/cplt#krav-i-nav": "/cplt#nav-policy",
  "/cplt#nettverk": "/cplt#network-proxy",
  "/cplt#sikkerhetsgrense": "/cplt#security-boundary",
  "/cplt#slik-virker-det": "/cplt#how-it-works",
  "/cplt#vakter": "/cplt#guards",
};
