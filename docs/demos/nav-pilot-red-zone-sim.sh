#!/bin/bash
# Simulated nav-pilot session showing rød/grønn sone in action.
# Used by the VHS tape file for generating demo GIFs.

set -e

BOLD='\033[1m'
DIM='\033[2m'
GREEN='\033[32m'
RED='\033[31m'
YELLOW='\033[33m'
CYAN='\033[36m'
RESET='\033[0m'

slow_print() {
    local text="$1"
    local delay="${2:-0.02}"
    # Print character by character. Use printf %s to handle multibyte UTF-8.
    local i=0
    local rendered
    rendered=$(echo -e "$text")
    local len=${#rendered}
    while [ $i -lt $len ]; do
        printf "%s" "${rendered:$i:1}"
        sleep "$delay"
        i=$((i + 1))
    done
    echo
}

section_pause() { sleep "${1:-2}"; }

# --- Header ---
echo -e "${DIM}╭──────────────────────────────────────────────────────────────╮${RESET}"
echo -e "${DIM}│${RESET}  ${BOLD}GitHub Copilot CLI${RESET}  •  ${DIM}@nav-pilot${RESET}                          ${DIM}│${RESET}"
echo -e "${DIM}╰──────────────────────────────────────────────────────────────╯${RESET}"
echo
section_pause 1

# --- User prompt ---
echo -e "${CYAN}❯${RESET} ${BOLD}@nav-pilot${RESET} Vi skal legge til støtte for ny beregningsregel:"
echo "  arbeidsgiverperioden skal reduseres fra 16 til 10 dager"
echo "  for kronisk syke med godkjent §8-20-vedtak."
echo
section_pause 2

# --- Phase 1 ---
slow_print "${BOLD}🔍 Fase 1: Intervju — kartlegger behov og blindsoner${RESET}" 0.03
echo
section_pause 1

echo -e "${DIM}● Analyserer kodebasen...${RESET}"
sleep 1
echo -e "${DIM}  Fant: Arbeidsgiverperiodeteller.kt — NormalArbeidstaker = Teller(16)${RESET}"
echo -e "${DIM}  Fant: Yrkesaktivitet.kt:843 — alltid NormalArbeidstaker i dag${RESET}"
echo -e "${DIM}  Fant: Ingen eksisterende §8-20-logikk${RESET}"
echo
section_pause 2

echo -e "${BOLD}🧐 Spørsmål:${RESET}"
echo "  1. Informasjonsflyt — Hvordan mottar spleis §8-20-vedtaket?"
echo "  2. Tidsavgrensning — Gjelder vedtaket for en bestemt periode?"
echo "  3. Teamgrenser — Eier dere informasjonen, eller annet team?"
echo
section_pause 2

# --- User answers ---
echo -e "${CYAN}❯${RESET} Eget signal fra Arena på Kafka-topic. Helt nytt domene for oss —"
echo "  ingen på teamet har jobbet med §8-20 før."
echo
section_pause 2

# --- Phase 2 ---
slow_print "${BOLD}📐 Fase 2: Plan — bygger arkitektur og beslutninger${RESET}" 0.03
echo
section_pause 1

echo "  ┌─────────────────────────────────────────────────┐"
echo "  │  Kafka topic → River → Person → Yrkesaktivitet  │"
echo "  │  → velger teller: KroniskSyk(10) vs Normal(16)  │"
echo "  └─────────────────────────────────────────────────┘"
echo
section_pause 2

echo -e "  ${GREEN}🟢 Grønn sone (genererer full kode):${RESET}"
echo "     • KroniskSykdomRiver.kt — Kafka-konsumering"
echo "     • KroniskSykdomVedtakMessage.kt — meldingsparsing"
echo "     • KroniskSykdomVedtak.kt — hendelsesklasse"
echo "     • Nais deploy-manifest — topic-tilgang"
echo
section_pause 2

echo -e "  ${RED}🔴 Rød sone (leverer kun stubs med TODO):${RESET}"
echo -e "     ${RED}• Tellervalg — logikken som avgjør om §8-20 gjelder${RESET}"
echo -e "     ${RED}• Subsumsjonslogging — riktig juridisk §8-20-kobling${RESET}"
echo -e "     ${RED}• Edge cases — vedtak midt i arbeidsgiverperiode${RESET}"
echo
section_pause 3

echo -e "  ${YELLOW}⚠️  Siden ingen på teamet har jobbet med §8-20 før,${RESET}"
echo -e "  ${YELLOW}   bør kjernelogikken skrives manuelt for å bygge forståelse.${RESET}"
echo
section_pause 3

# --- Phase 3 (abbreviated) ---
slow_print "${BOLD}🔎 Fase 3: Review — verifiserer fra fire perspektiver${RESET}" 0.03
echo
echo "  ✅ Sikkerhet    ✅ Plattform    ✅ Arkitektur    ✅ Endringssikkerhet"
echo
section_pause 2

# --- Phase 4 ---
slow_print "${BOLD}🚀 Fase 4: Lever — genererer kode og dokumentasjon${RESET}" 0.03
echo
section_pause 1

echo -e "  ${GREEN}Genererer full kode:${RESET}"
echo -e "  ${DIM}● Create KroniskSykdomRiver.kt                    +31 linjer${RESET}"
sleep 0.5
echo -e "  ${DIM}● Create KroniskSykdomVedtakMessage.kt             +30 linjer${RESET}"
sleep 0.5
echo -e "  ${DIM}● Create KroniskSykdomVedtak.kt                    +26 linjer${RESET}"
sleep 0.5
echo -e "  ${DIM}● Edit   Arbeidsgiverperiodeteller.kt              +1 linje${RESET}"
sleep 0.5
echo -e "  ${DIM}● Edit   Paragraf.kt                               +1 linje${RESET}"
echo
section_pause 2

echo -e "  ${RED}🔴 Rød-sone stubs (du skriver kjernelogikken):${RESET}"
echo
sleep 0.5
echo -e "  ${DIM}● Edit   Yrkesaktivitet.kt${RESET}"
echo -e "    ${RED}private fun velgArbeidsgiverperiodeteller(dato: LocalDate) ="
echo -e "        TODO(\"🔴 Rød sone: Implementer §8-20-sjekk.\")"
echo -e "        // Sjekk om gyldig vedtak overlapper dato."
echo -e "        // Skriv dette manuelt — regelverksforståelse er kritisk.${RESET}"
echo
sleep 1
echo -e "  ${DIM}● Edit   Subsumsjonslogg.kt${RESET}"
echo -e "    ${RED}fun \`§ 8-20 ledd 1\`(...) ="
echo -e "        TODO(\"🔴 Rød sone: Definer subsumsjon for §8-20.\")"
echo -e "        // Hvilke input/output skal logges?"
echo -e "        // Forstå juridisk kontekst før implementering.${RESET}"
echo
section_pause 3

echo -e "  ${DIM}● Create KroniskSykdomE2ETest.kt (skjelett)${RESET}"
echo -e "    ${RED}@Test fun \`10 dagers agp med gyldig §8-20-vedtak\`() {"
echo -e "        TODO(\"Skriv testen manuelt — edge cases krever forståelse\")"
echo -e "    }${RESET}"
echo
section_pause 3

# --- Summary ---
echo "  ─────────────────────────────────────────────────────────────"
echo -e "  ${BOLD}Oppsummering:${RESET}"
echo -e "  ${GREEN}✅ 5 filer generert komplett${RESET} (plumbing, River, manifest)"
echo -e "  ${RED}🔴 3 stubs med TODO${RESET} (kjernelogikk du bør forstå dypt)"
echo -e "  ${BOLD}→ Skriv tellervalg + subsumsjon selv for å bygge kompetanse${RESET}"
echo "  ─────────────────────────────────────────────────────────────"
echo
section_pause 5
