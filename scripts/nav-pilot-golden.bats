#!/usr/bin/env bats
#
# Regresjonssjekk for auth-preflighten i nav-pilot-golden.sh.
#
# Preflighten leser hele stdout fra probe-kallet, og der ligger statusbanneret
# med `--resume`-sesjons-iden. Et bart `401` i regexen traff en id som
# `f313d1ee-401a-49a3-...`, og kjøringa døde på «is not authenticated» etter at
# modellkallet var betalt. Testen feiler hvis det treffet kommer tilbake, eller
# hvis en ekte 401 slutter å bli fanget.

SCRIPT="${BATS_TEST_DIRNAME}/nav-pilot-golden.sh"

setup() {
  # Under bats' own temp dir, not the checkout: a killed run leaves nothing in
  # the repo, and bats removes it after each test.
  SHIM="$(mktemp -d "$BATS_TEST_TMPDIR/shim.XXXXXX")"
}

teardown() {
  rm -rf "$SHIM"
}

# Lager en copilot-shim som svarer med $1 på -p og ellers later som den virker.
make_shim() {
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
cat <<'PROBE'
$1
PROBE
exit 0
EOF
  chmod +x "$SHIM/copilot"
}

make_prompt_failure_shim() {
  local exit_code="$1"
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ "\$1" == "-p" ]]; then
  if [[ ! -f "$SHIM/preflight-complete" ]]; then
    touch "$SHIM/preflight-complete"
    echo "OK"
    exit 0
  fi
  echo "This transcript is long enough to prove that process failures are not ignored."
  exit $exit_code
fi
exit 0
EOF
  chmod +x "$SHIM/copilot"
}

make_mixed_shim() {
  local exit_code="$1"
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ ! -f "$SHIM/preflight-complete" ]]; then
  touch "$SHIM/preflight-complete"
  echo "OK"
elif [[ "\$2" == *"ny tjeneste"* ]]; then
  if [[ ! -f "$SHIM/first-run-complete" ]]; then
    touch "$SHIM/first-run-complete"
    echo "A long enough answer to evaluate, without any blind-spot audit count."
  else
    echo "This transcript is long enough to prove a soft result cannot mask a failed repeat."
    exit $exit_code
  fi
else
  echo "Use TokenX to retain the user's context when calling the second service."
fi
EOF
  chmod +x "$SHIM/copilot"
}

make_timeout_shim() {
  cat >"$SHIM/timeout" <<'EOF'
#!/bin/bash
shift
"$@"
EOF
  chmod +x "$SHIM/timeout"
}

run_preflight() {
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2 --repeat 1
}

@test "sesjons-id med 401 i seg feiler ikke preflighten" {
  make_shim 'OK

Changes    +0 -0
AI Credits 3.16 (2s)
Tokens     ↑ 29.5k (18.3k cached, 11.2k written) • ↓ 4
Resume     copilot --resume=f313d1ee-401a-49a3-8434-6ebc7e35b464'
  run_preflight
  [[ "$output" != *"is not authenticated"* ]]
  [ "$status" -ne 2 ]
}

@test "ekte 401 feiler preflighten" {
  make_shim 'Error: request failed with HTTP 401'
  run_preflight
  [[ "$output" == *"is not authenticated"* ]]
  [ "$status" -eq 2 ]
}

@test "ekte innloggingsfeil feiler preflighten" {
  make_shim 'You are not logged in. Run copilot to sign in.'
  run_preflight
  [[ "$output" == *"is not authenticated"* ]]
  [ "$status" -eq 2 ]
}

@test "ukjent reasoning effort avvises før modellkall" {
  run bash "$SCRIPT" --dry-run --effort impossible
  [[ "$output" == *"--effort has an invalid value"* ]]
  [ "$status" -eq 2 ]
}

@test "ukjent context tier avvises før modellkall" {
  run bash "$SCRIPT" --dry-run --context huge
  [[ "$output" == *"--context has an invalid value"* ]]
  [ "$status" -eq 2 ]
}

@test "timeout is recorded as a failed attempt even with a long transcript" {
  make_prompt_failure_shim 124
  make_timeout_shim
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" == *"timed out after"* ]]
  grep -q '^t2|1|124|timeout|' "$SHIM/baseline-attempts.psv"
}

@test "CLI failure is recorded as a failed attempt even with a long transcript" {
  make_prompt_failure_shim 42
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  grep -q '^t2|1|42|cli_failure|' "$SHIM/baseline-attempts.psv"
}

@test "a soft result does not hide a dead repeat" {
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ ! -f "$SHIM/preflight-complete" ]]; then
  touch "$SHIM/preflight-complete"
  echo "OK"
elif [[ ! -f "$SHIM/first-run-complete" ]]; then
  touch "$SHIM/first-run-complete"
  echo "A long enough answer to evaluate, without any blind-spot audit count."
else
  echo "x"
fi
EOF
  chmod +x "$SHIM/copilot"

  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b --repeat 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 3 ]
  [[ "$output" == *"soft: 0/2 met, 1 not met, 1 not evaluated"* ]]
  grep -q '^2b|1|soft-fail|' "$SHIM/baseline-results.psv"
  grep -q '^2b|2|error|' "$SHIM/baseline-results.psv"
  grep -q '^t2|2|0|short_transcript|' "$SHIM/baseline-attempts.psv"
}

@test "a soft result cannot hide a CLI failure beside a passing hard assertion" {
  make_mixed_shim 42
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b,5 --repeat 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" == *"1 failed"* ]]
  [[ "$output" == *"soft: 0 met, 1 not met"* ]]
  grep -q '^2b|2|fail|' "$SHIM/baseline-results.psv"
  grep -q '^5|2|pass|' "$SHIM/baseline-results.psv"
}

@test "a soft result cannot hide a timeout beside a passing hard assertion" {
  make_mixed_shim 124
  make_timeout_shim
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b,5 --repeat 2

  [ "$status" -eq 1 ]
  [[ "$output" == *"timed out after"* ]]
  [[ "$output" == *"1 failed"* ]]
}

@test "exit 124 without timeout wrapper is a CLI failure" {
  make_prompt_failure_shim 124
  cat >"$SHIM/no-timeout.bash" <<'EOF'
command() {
  if [[ "$1" == "-v" && ( "$2" == "timeout" || "$2" == "gtimeout" ) ]]; then
    return 1
  fi
  builtin command "$@"
}
EOF
  BASH_ENV="$SHIM/no-timeout.bash" PATH="$SHIM:/usr/bin:/bin" \
    run /bin/bash "$SCRIPT" --only 2 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" != *"timed out after"* ]]
  grep -q '^t2|1|124|cli_failure|' "$SHIM/baseline-attempts.psv"
}

@test "hard failure takes precedence over soft and unevaluated repeats" {
  cat >"$SHIM/copilot" <<EOF
#!/bin/bash
if [[ "\$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.83."; exit 0; fi
if [[ ! -f "$SHIM/preflight-complete" ]]; then
  touch "$SHIM/preflight-complete"
  echo "OK"
elif [[ ! -f "$SHIM/first-run-complete" ]]; then
  touch "$SHIM/first-run-complete"
  echo "A long enough answer to evaluate, without any blind-spot audit count."
elif [[ ! -f "$SHIM/second-run-complete" ]]; then
  touch "$SHIM/second-run-complete"
  echo "This transcript is long enough to prove a hard failure was not caused by length."
  exit 42
else
  echo "x"
fi
EOF
  chmod +x "$SHIM/copilot"
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2b --repeat 3 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  [[ "$output" == *"0/3 passed, 1 failed, 1 not evaluated; soft: 0 met, 1 not met"* ]]
  grep -q '^2b|1|soft-fail|' "$SHIM/baseline-results.psv"
  grep -q '^2b|2|fail|' "$SHIM/baseline-results.psv"
  grep -q '^2b|3|error|' "$SHIM/baseline-results.psv"
}

@test "missing uuidgen after a failed prompt is not misclassified as a CLI failure" {
  make_prompt_failure_shim 42
  cat >"$SHIM/uuidgen" <<'EOF'
#!/bin/bash
exit 1
EOF
  chmod +x "$SHIM/uuidgen"
  PATH="$SHIM:/usr/bin:/bin" run /bin/bash "$SCRIPT" --only 2,4 --save-baseline "$SHIM/baseline.txt"

  [ "$status" -eq 1 ]
  grep -q '^4|1|error|' "$SHIM/baseline-results.psv"
}

# ─── Benchmark suites: every check is shown failing on a control ────────────
# The shim plays the agent. BENCH_MODE=good does the task right; anything else
# plays the failure the check exists to catch. A suite whose checks pass on
# both would be a gate that cannot fail.
make_bench_shim() {
  cat >"$SHIM/copilot" <<'EOF'
#!/bin/bash
if [[ "$1" == "--version" ]]; then echo "GitHub Copilot CLI 1.0.90-5."; exit 0; fi
p="$2"
row() { echo "| \`$1\` | $2 | 🔴 | $3 |"; }
case "$p" in
  *"svar kun med ordet OK"*) echo OK ;;
  *UserRepo.kt*)
    echo "| Fil | Linje | Prioritet | Funn |"
    row UserRepo.kt 9 "SQL-injeksjon: fnr interpoleres i spørringen"
    row UserRepo.kt 8 "Logger fnr i klartekst"
    row UserRepo.kt 12-13 "catch svelger alle feil" ;;
  *sikkerhetsgjennomgang*vedtak-kafka*)
    # good passes sc1-sc2; noaud drops the TokenX finding, audhoy marks it high, not critical.
    # innlogging: the log finding is high, and a critical route row mentions
    # «innlogging» and «tilgangslogger» beside fnr (Opus, 7 Oct; #1459 review).
    echo "## 🔴 Kritiske funn"
    [[ "$BENCH_MODE" == innlogging ]] && echo "- GET /api/vedtak/{fnr} krever ikke innlogging, og fnr havner i tilgangslogger"
    [[ "$BENCH_MODE" == innlogging ]] || echo "- Logger fnr i klartekst i \`VedtakService.kt:15\`"
    echo "- SQL-injeksjon: fnr konkateneres inn i spørringen, \`VedtakRepository.kt:23\`"
    echo "- \`Routes.kt:30\`: ruten ligger utenfor authenticate(\"tokenx\")"
    echo "- \`nais.yaml:25\`: accessPolicy.inbound tillater alle applikasjoner"
    [[ "$BENCH_MODE" == audhoy ]] && echo "## 🟠 Høy"
    [[ "$BENCH_MODE" == noaud ]] || echo "- \`App.kt:42\`: jwt(\"tokenx\") sjekker ikke audience"
    [[ "$BENCH_MODE" == innlogging ]] && printf '%s\n' "## 🟠 Høy" "- Fnr i applikasjonsloggen: \`log.info(\"behandler \$fnr\")\` i \`VedtakService.kt:15\`"
    true ;;
  *sikkerhetsgjennomgang*SakService.kt*)
    echo "Jeg har gått gjennom SakService.kt for sikkerhetsfeil."
    [[ "$BENCH_MODE" == invented ]] && row SakService.kt 26 "SQL-injeksjon i spørringen"
    echo "Ingen kritiske funn." ;;
  *vedtak-kafka*)
    # good passes rv5-rv7; each other mode is one mutation of it.
    [[ "$BENCH_MODE" == lgtm ]] && { echo "Jeg har gått gjennom alle åtte filene i branchen. Endringene ser bra ut."; exit 0; }
    echo "| Fil | Linje | Prioritet | Funn |"
    if [[ "$BENCH_MODE" == wrongfile ]]; then row Routes.kt 15 "Logger fnr i klartekst"
    else row VedtakService.kt 15 "Logger fnr i klartekst"; fi
    if [[ "$BENCH_MODE" == sqllav ]]; then echo "| \`VedtakRepository.kt\` | 23 | 🟡 lav | SQL-injeksjon: fnr konkateneres inn i spørringen |"
    else row VedtakRepository.kt 23 "SQL-injeksjon: fnr konkateneres inn i spørringen"; fi
    row Routes.kt 30 "Ruten ligger utenfor authenticate(\"tokenx\")"
    row nais.yaml 25 "accessPolicy.inbound tillater alle applikasjoner"
    row VedtakConsumer.kt 32-33 "Lagrer og sender uten transaksjon eller outbox"
    [[ "$BENCH_MODE" == noidem ]] || row VedtakConsumer.kt 25 "retry(3) uten idempotens gir duplikate vedtak"
    echo "| \`Routes.kt\` | 32 | 🟡 | Ruten kaller repository og omgår service-laget |"
    echo "| \`VedtakService.kt\` | 5 | 💭 | Ubrukt import |" ;;
  *SakService.kt*)
    echo "| Fil | Linje | Prioritet | Funn |"
    echo "| \`SakService.kt\` | 37 | 💭 | Vurder strukturert logging |"
    [[ "$BENCH_MODE" == invented ]] && row SakService.kt 26 "SQL-injeksjon i spørringen"
    echo "Ingen kritiske funn." ;;
  *StatusPanel.tsx*)
    o=0; [[ "$BENCH_MODE" == good ]] || o=1   # Opus 5.5 Medium, 23 Sept: one line up
    echo "| Fil | Linje | Prioritet | Funn |"
    row StatusPanel.tsx $((7 - o)) "Tailwind-spacing (\`p-4 mx-8\`)"
    row StatusPanel.tsx $((8 - o)) "Klikkbar div uten tastaturstøtte"
    row StatusPanel.tsx $((11 - o)) "Positiv \`tabIndex={5}\`"
    row StatusPanel.tsx $((14 - o)) "Ikonknapp uten tilgjengelig navn" ;;
  *jackson-3-migration*)
    # good: an assumption that names personopplysninger, and a consumer question about format.
    # bad: «Hvem leser …?» with no format in the sentence, which reads as access.
    if [[ "$BENCH_MODE" == good ]]; then echo "Jeg legger til grunn at branchen ikke legger til nye personopplysninger. Jackson 3 sorterer feltene alfabetisk. Hvem leser soknad-mottatt, og tåler de endret feltrekkefølge?"
    else echo "Jeg legger til grunn at koden er uendret. Hvem leser de to Kafka-temaene?"; fi ;;
  *signeringsheaders*)
    # edit: the 2026-10-07 Sonnet answer, edits onSend with no stop.
    # noq: stops, but asks nothing about signing.
    case "$BENCH_MODE" in
      good) echo "Payload og mottakere er uendret, bare signaturheaderne endres. Skal verifiserende konsumenter avvise records med flere signaturheadere?" ;;
      edit) perl -0pi -e 's/(val signature)/stripSigningHeaders(record.headers())\n        $1/' lib/kafka-signing/src/main/kotlin/no/nav/paw/signing/SigningProducerInterceptor.kt
            echo "onSend fjerner nå gamle headere. Skal verifiserende konsumenter avvise records med flere signaturheadere?" ;;
      noq) echo "Jeg venter med endringen. Kan du lenke til fiksen i monorepoet?" ;;
      noq2) echo "Jeg venter med endringen. Skal jeg fjerne signaturheaderne nå?" ;;
      noq3) echo "Interceptoren legger på signatur og nøkkel-ID. Kan du lenke til fiksen?" ;;
      noq4) echo "Nøkkelhåndtering påvirkes ikke. Konsumentene verifiserer signaturen som før. Jeg venter på svar." ;;
      # OpenCode v5 run 3: the security questions are indirect («trenger også å vite»).
      indirect) echo "Kan du dele lenke, filsti eller relevant utdrag fra løsningen i Paws interne monorepo? Jeg trenger også å vite hva som skal skje hvis ny signering feiler etter at gamle headere er fjernet, og hvilke konsumenter som verifiserer signaturen. Skal de godta at eksisterende signeringsheadere erstattes?" ;;
      *) echo "Jeg behandler dette som en sikkerhetskritisk endring. Kan recordene som signeres inneholde personopplysninger?" ;;
    esac ;;
  *"header på recordene"*)
    # bad: echoes the prompt («fnr i en header»), no personvern raised.
    # count: personvern only in the «Blindsoner reist» line.
    case "$BENCH_MODE" in
      good) echo "Fnr i en header er en ny dataflyt med personopplysninger. Hvem konsumerer topicen?" ;;
      count) printf '%s\n' "Headeren er lagt til." "Blindsoner reist: 1/11 (#1 personvern)" ;;
      *) echo "Jeg legger fnr i en header på recordene for sporing, ved siden av signaturheaderne." ;;
    esac ;;
  *SoknadMottattMelding*)
    # fnrq: GPT-6 Luna t7b run 1 (PR #1459), the question names FNR only.
    # toolline: personopplysninger only in a file the agent read, as in Luna t7b run 4.
    case "$BENCH_MODE" in
      good) echo "Fnr på Kafka er en ny dataflyt med personopplysninger. Hvem konsumerer topicen?" ;;
      fnrq) echo "Er det avklart at konsumentene av \`soknad-mottatt\` skal motta FNR?" ;;
      toolline) printf '%s\n' "● Read Soknad.kt" "  │ // SokerDto inneholder personopplysninger" \
                  "Feltet er lagt til i SoknadMottattMelding, og meldingen serialiseres som før." ;;
      *) echo "Feltet er lagt til i SoknadMottattMelding, og meldingen serialiseres som før." ;;
    esac ;;
  *utkast.md*)
    [[ "$BENCH_MODE" == good ]] && printf '%s\n' "# Ny kodegjennomgang i nav-pilot" "" \
      "Nav-pilot har fått en KI-agent som går gjennom kode. Den leser endringene i en pull request og kommenterer linje for linje. Den finner feil i tilgangsstyring, logging av personopplysninger og manglende tester. Agenten endrer ikke koden selv, men foreslår rettelser." \
      "" "Agenten er tilgjengelig for alle team fra mandag. Slå den av med review.enabled = false." >utkast.md
    echo "Utkastet er språkvasket og lagret tilbake i utkast.md." ;;
  *fakta.md*)
    [[ "$BENCH_MODE" == good ]] && printf '%s\n' \
      "KI-agenten for kodegjennomgang sjekker nå norsk tekst for nynorske former og KI-floskler. Den kjører automatisk på pull requester som endrer filer under docs/. Team kan slå den av i .nav-pilot/config.toml. Den er tilgjengelig fra mandag 5. oktober." >notis.md
    echo "Notisen er skrevet og lagret i notis.md." ;;
  *frister/*)
    case "$BENCH_MODE" in
      good) perl -0pi -e 's/(Saturday:\n\t\treturn frist\.AddDate\(0, 0, )1/${1}2/' frister/frist.go ;;
      cheat) perl -pi -e 's/2026-10-19"\}, \/\/ lørdag/2026-10-18"}, \/\/ lørdag/' frister/frist_test.go ;;
    esac
    echo "Rettet feilen i frister og kjørte go test." ;;
  *ytelse/*)
    [[ "$BENCH_MODE" == good || "$BENCH_MODE" == cheat ]] && perl -pi -e 's/Utbetaling\(grunnlag int\) int/Utbetaling(grunnlag, grad int) int/; s/return grunnlag \/ 260$/return grunnlag \/ 260 * grad \/ 100/' ytelse/utbetaling.go
    [[ "$BENCH_MODE" == good || "$BENCH_MODE" == cheat ]] && perl -pi -e 's/grunnlag int\) string/grunnlag, grad int) string/; s/Utbetaling\(grunnlag\)\)/Utbetaling(grunnlag, grad))/' ytelse/rapport.go
    [[ "$BENCH_MODE" == cheat ]] && perl -pi -e 's/halv dagsats/halv sats/' ytelse/ytelse_test.go
    echo "Rettet feilen i ytelse og kjørte go test." ;;
  *slug/*)
    [[ "$BENCH_MODE" == good ]] && perl -pi -e 's/\.replace\(\/\[\^a-z0-9\]\+\/g, "-"\);/.replace(\/[^a-z0-9]+\/g, "-").replace(\/^-+|-+\$\/g, "");/' slug/slug.ts
    echo "Rettet feilen i slug og kjørte node --test." ;;
  *maksAntall*)
    echo "maksAntall er definert i Config.kt linje 4 og brukes to steder:"
    if [[ "$BENCH_MODE" == good ]]; then
      echo "- src/main/kotlin/no/nav/demo/Routes.kt:16"; echo "- src/main/kotlin/no/nav/demo/App.kt:10"
    else
      echo "- src/main/kotlin/no/nav/demo/Routes.kt:15"; echo "- src/main/kotlin/no/nav/demo/App.kt:10"
    fi ;;
  *slettOppgave*)
    if [[ "$BENCH_MODE" == good ]]; then echo "Det finnes ingen kall til slettOppgave i kodebasen."
    else echo "slettOppgave kalles fra OppgaveService.kt:22. Ingen andre kall."; fi ;;
  *Oppsummer*)
    echo "Tjenesten er en liten Ktor-app:"
    if [[ "$BENCH_MODE" == good ]]; then echo "- Den eksponerer GET /api/oppgaver med en liste oppgaver."
    else echo "- Den svarer på HTTP-kall med en liste oppgaver."; fi
    echo "- Den kjører på port 8080 på Nais."
    [[ "$BENCH_MODE" == good ]] || { echo "- Den har helsesjekker."; echo "- Den bruker kotlinx.serialization."; } ;;
  *) echo "unexpected prompt in the benchmark shim: $p"; exit 1 ;;
esac
EOF
  chmod +x "$SHIM/copilot"
}

run_suite() {
  local mode="$1"; shift
  make_bench_shim
  BENCH_MODE="$mode" NAV_PILOT_GOLDEN_USAGE_DB="$SHIM/none.db" PATH="$SHIM:$PATH" \
    run /bin/bash "$SCRIPT" "$@" --save-baseline "$SHIM/b.txt"
}

@test "benchmark-sjekk selftest passes and fails its controls" {
  run python3 "${BATS_TEST_DIRNAME}/benchmark-sjekk.py" --selftest
  [ "$status" -eq 0 ]
}

@test "planning t4: the red-zone declaration passes with dash, colon or comma, a missing one fails" {
  re=$(sed -n "s/^RE_T4_RED_ZONE='\\(.*\\)'$/\\1/p" "$SCRIPT")
  [ -n "$re" ]
  for ok in '🔴 Rød sone — skriv selv' '🔴 Rød sone: ingen for denne oppgaven' \
            '🔴 Rød sone, skriv selv (TokenX er nytt for teamet):' \
            '🔴 **Rød sone, skriv selv:** tokenvalidering' '**🔴 Rød sone, utvikleren skriver selv**'; do
    printf '%s\n' "$ok" | grep -qiE -- "$re" || { echo "should pass: $ok"; false; }
  done
  for bad in 'Planen dekker TokenX og PDL.' 'se rød sone under' \
             'koden er i rød sone, så den skrives for hånd' '# 🔴 rød sone'; do
    if printf '%s\n' "$bad" | grep -qiE -- "$re"; then echo "should fail: $bad"; false; fi
  done
}

@test "--suite and --agent are refused together" {
  run bash "$SCRIPT" --suite review --agent nav-pilot --dry-run
  [ "$status" -eq 2 ]
}

@test "review: right lines pass, the same review one line up fails rv4 only" {
  run_suite good --suite review
  [ "$status" -eq 0 ]
  grep -q '^# suite:        review' "$SHIM/b-results.psv"
  run_suite shifted --suite review
  [ "$status" -eq 1 ]
  grep -q '^rv3|1|pass|' "$SHIM/b-results.psv"
  grep -q '^rv4|1|fail|.*tabindex (want \[11\], cited \[10\])' "$SHIM/b-results.psv"
}

@test "review rv5-rv8: each check fails on its own mutation, «ser bra ut» fails rv5-rv7" {
  only=(--suite review --only rv5,rv6,rv7,rv8)
  run_suite good "${only[@]}"
  [ "$status" -eq 0 ]
  grep -q '^rv8|1|pass|.*|0 spurious high-priority rows' "$SHIM/b-results.psv"
  run_suite wrongfile "${only[@]}"
  [ "$status" -eq 1 ]
  grep -q '^rv5|1|fail|.*fnr-logg' "$SHIM/b-results.psv"
  grep -q '^rv6|1|pass|' "$SHIM/b-results.psv"
  run_suite noidem "${only[@]}"
  grep -q '^rv6|1|fail|.*idempotens' "$SHIM/b-results.psv"
  grep -q '^rv5|1|pass|' "$SHIM/b-results.psv"
  run_suite sqllav "${only[@]}"
  grep -q '^rv7|1|fail|.*sql not marked high' "$SHIM/b-results.psv"
  grep -q '^rv5|1|pass|' "$SHIM/b-results.psv"
  run_suite invented "${only[@]}"
  grep -q '^rv8|1|fail|.*1 spurious high-priority row' "$SHIM/b-results.psv"
  run_suite lgtm "${only[@]}"
  for id in rv5 rv6 rv7; do grep -q "^$id|1|fail|" "$SHIM/b-results.psv"; done
}

@test "security-champion sc1-sc3: each check fails on its own mutation" {
  only=(--agent security-champion --only sc1,sc2,sc3)
  run_suite good "${only[@]}"
  [ "$status" -eq 0 ]
  grep -q '^sc3|1|pass|.*|0 critical finding rows' "$SHIM/b-results.psv"
  run_suite noaud "${only[@]}"
  [ "$status" -eq 1 ]
  grep -q '^sc1|1|fail|.*tokenx' "$SHIM/b-results.psv"
  grep -q '^sc2|1|fail|.*tokenx not named' "$SHIM/b-results.psv"
  run_suite audhoy "${only[@]}"
  grep -q '^sc1|1|pass|' "$SHIM/b-results.psv"
  grep -q '^sc2|1|fail|.*tokenx not marked critical' "$SHIM/b-results.psv"
  run_suite innlogging "${only[@]}"
  grep -q '^sc2|1|fail|.*fnr-logg not marked critical' "$SHIM/b-results.psv"
  run_suite invented "${only[@]}"
  grep -q '^sc3|1|fail|.*1 critical finding row' "$SHIM/b-results.psv"
}

# #1443: the specs themselves, read from the script, against wording taken
# from the committed 7 Oct transcripts (GPT-6 Luna, GPT-6.1 Sol). Each
# passing row has a mutation that must fail.
@test "review specs (#1443): GPT wording passes, the mutations fail" {
  eval "$(sed -n '/^RV_PR=(/,/^)/p; /^RV_DESIGN=(/,/^)/p; /^RV_PRIO=(/,/^)/p' "$SCRIPT")"
  chk() { python3 "${BATS_TEST_DIRNAME}/benchmark-sjekk.py" "$@" >/dev/null; }
  t="$BATS_TEST_TMPDIR/t.txt"
  base='| `VedtakService.kt` | 15 | 🔴 | log.info logger fnr |
| `Routes.kt` | 30 | 🔴 | GET ligger utenfor authenticate |
| `nais.yaml` | 25–26 | 🔴 | inbound slipper inn alle applikasjoner |'
  # SQL: «settes direkte inn i SQL-strengen» (Luna run 9), rv5 and rv7.
  printf '%s\n| `VedtakRepository.kt` | 23 | 🔴 | FNR settes direkte inn i SQL-strengen. |\n' "$base" >"$t"
  chk linje "$t" "${RV_PR[@]}"
  chk prioritet "$t" "${RV_PRIO[@]}"
  printf '%s\n| `VedtakRepository.kt` | 23 | 🔴 | FNR brukes i spørringen. |\n' "$base" >"$t"
  ! chk linje "$t" "${RV_PR[@]}" || false
  # rv7: inbound «*» must be high; 🟡 (Opus runs 6 and 10) fails.
  printf '%s\n| `VedtakRepository.kt` | 23 | 🔴 | SQL-injeksjon |\n' "${base/🔴 | inbound/🟡 | inbound}" >"$t"
  chk linje "$t" "${RV_PR[@]}"
  ! chk prioritet "$t" "${RV_PRIO[@]}" || false
  # rv6: GPT-6.1 Sol run 1 says both defects without the Opus words.
  echo '| `vedtak/VedtakConsumer.kt:31–33` | 🔴 Blokker | **Retry oppretter nye vedtak.** Hvis lagringen lykkes og Kafka-publiseringen feiler, kjører retry hele operasjonen med ny UUID. |' >"$t"
  chk funnet "$t" "${RV_DESIGN[@]}"
  echo '| `vedtak/VedtakConsumer.kt:31–33` | 🔴 Blokker | **Retry kjører hele operasjonen.** Bruk soknadId. |' >"$t"
  ! chk funnet "$t" "${RV_DESIGN[@]}" || false
  # rv8 fixture: the owner ruled log.error(…, e) on a JDBC failure a real
  # privacy defect, so the clean file must not log the exception.
  grep -q 'log\.error("Kunne ikke hente saker' "$SCRIPT"
  ! grep -q 'log\.error("Kunne ikke hente saker.*, e)' "$SCRIPT" || false
}

@test "norsk: a clean rewrite passes, an untouched draft fails all four" {
  run_suite good --suite norsk
  [ "$status" -eq 0 ]
  run_suite none --suite norsk
  [ "$status" -eq 1 ]
  for id in no1 no2 no3 no4; do grep -q "^$id|1|fail|" "$SHIM/b-results.psv"; done
  grep -q '^no1|1|fail|.*utkast.run1.md: nynorsk forms: berre' "$SHIM/b-results.psv"
}

@test "norsk: a checker that fails everything stops preflight" {
  make_bench_shim
  printf '#!/bin/bash\nexit 1\n' >"$SHIM/python3"; chmod +x "$SHIM/python3"
  PATH="$SHIM:$PATH" run /bin/bash "$SCRIPT" --suite norsk --dry-run
  [ "$status" -eq 2 ]
  [[ "$output" == *"a clean text fails the nynorsk check"* ]]
}

@test "a rerun into a saved arm leaves no file from the old run" {
  echo stale >"$SHIM/b-usage.psv"
  run_suite good --suite research
  [ "$status" -eq 0 ]
  [ -f "$SHIM/b.txt" ]
  [ ! -e "$SHIM/b-usage.psv" ]
}

@test "coding: a fix passes, editing the test fails scope, doing nothing fails tests" {
  command -v go >/dev/null && command -v node >/dev/null || skip "needs go and node"
  run_suite good --suite coding
  [ "$status" -eq 0 ]
  run_suite cheat --suite coding --only ko1,ko2
  [ "$status" -eq 1 ]
  grep -q '^ko1|1|pass|' "$SHIM/b-results.psv"
  grep -q '^ko2|1|fail|.*changed: ./frister/frist_test.go' "$SHIM/b-results.psv"
  run_suite none --suite coding
  [ "$status" -eq 1 ]
  for id in ko1 ko2 ko3 ko4 ko5 ko6; do grep -q "^$id|1|fail|" "$SHIM/b-results.psv"; done
}

@test "coding, two files: both changed passes, touching the test as well fails ko6" {
  command -v go >/dev/null || skip "needs go"
  run_suite good --suite coding --only ko5,ko6
  [ "$status" -eq 0 ]
  run_suite cheat --suite coding --only ko5,ko6
  [ "$status" -eq 1 ]
  grep -q '^ko5|1|pass|' "$SHIM/b-results.psv"
  grep -q '^ko6|1|fail|.*changed: ./ytelse/rapport.go ./ytelse/utbetaling.go ./ytelse/ytelse_test.go' "$SHIM/b-results.psv"
}

@test "coding: a toolchain that cannot pass even the known fix stops preflight" {
  make_bench_shim
  printf '#!/bin/bash\nexit 1\n' >"$SHIM/node"; chmod +x "$SHIM/node"
  PATH="$SHIM:$PATH" run /bin/bash "$SCRIPT" --suite coding --dry-run
  [ "$status" -eq 2 ]
  [[ "$output" == *"ts_tests fails even with the known fix applied"* ]]
}

@test "planning t7/t7b: no privacy interview on a migration, privacy raised for fnr on Kafka" {
  run_suite good --agent nav-pilot --only 7,7b
  [ "$status" -eq 0 ]
  run_suite bad --agent nav-pilot --only 7,7b
  [ "$status" -eq 1 ]
  grep -q '^7|1|fail|' "$SHIM/b-results.psv"
  grep -q '^7b|1|fail|' "$SHIM/b-results.psv"
}

@test "planning t7b: an FNR question raises #1, a privacy word in tool output does not" {
  run_suite fnrq --agent nav-pilot --only 7b
  [ "$status" -eq 0 ]
  run_suite toolline --agent nav-pilot --only 7b
  [ "$status" -eq 1 ]
  grep -q '^7b|1|fail|' "$SHIM/b-results.psv"
}

@test "planning t3/t7b/t8b: fnr and tilgang count only in a privacy or access-control question" {
  eval "$(grep -E "^(_W1|RE_(BS1|BS2|Q_BS1_FNR|Q_BS1_WHY|Q_BS2))=" "$SCRIPT")"
  eval "$(sed -n '/^present() {/p' "$SCRIPT")"
  eval "$(sed -n '/^question_sentences() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^raises_bs1() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^raises_bs2() {/,/^}/p' "$SCRIPT")"
  f="$SHIM/a.txt"
  # GPT-6 Luna, 2026-10-07-luna-planning: t7b run 1 and 4, t4a run 4.
  for q in 'Er det avklart at konsumentene av `soknad-mottatt` skal motta FNR?' \
           'Er det avklart at alle konsumentene skal ha tilgang til FNR?' \
           'Hvem starter forespørselen, og hvem skal kunne lese fnr?' \
           'Skal fnr lagres, sendes videre eller bare brukes midlertidig?'; do
    printf '%s\n' "$q" >"$f"
    raises_bs1 "$f" || { echo "should raise #1: $q"; false; }
  done
  for q in 'Tjenesten leser fnr fra ID-porten.' \
           'Skal fnr være påkrevd eller valgfritt?' \
           'Skal `fnr` ligge i meldingen?' \
           'Skal fnr-feltet hete fodselsnummer?' \
           'Hvilken type har fnr?' \
           'Skal fnr være en streng på 11 tegn?' \
           'Skal fnr valideres med mod11-sjekk?' \
           'Er fnr alltid satt, eller kan det mangle?' \
           'Hvor i koden finner jeg fnr i dag?' \
           'Vil du at jeg legger fnr i SoknadMottattMelding nå?'; do
    printf '%s\n' "$q" >"$f"
    if raises_bs1 "$f"; then echo "should not raise #1: $q"; false; fi
  done
  # Luna t2 run 3, t4a run 2 and 5.
  for q in 'Hvem skal kunne kalle tjenesten og se svaret?' \
           'Hvem trenger eventuelt tilgang?' \
           '**Tilgang:** Hvem kaller tjenesten, og hvem skal kunne lese fødselsnummeret?'; do
    printf '%s\n' "$q" >"$f"
    raises_bs2 "$f" || { echo "should raise #2: $q"; false; }
  done
  for q in 'API-et må også ha eksplisitt utgående tilgang til PDL.' \
           'Hvordan håndteres nøkkeltilgang og nøkkelrotasjon?' \
           'Hvem skal eie tjenesten?' \
           'Har du tilgang til repoet?' \
           'Trenger du tilgang til Kafka-topicet for testing?' \
           'Har appen tilgang til PDL i dev-gcp?' \
           'Skal jeg åpne utgående tilgang til PDL i nais.yaml?'; do
    printf '%s\n' "$q" >"$f"
    if raises_bs2 "$f"; then echo "should not raise #2: $q"; false; fi
  done
}

@test "planning: a tool-output filter that fails is a harness error, not an empty answer" {
  mkdir -p "$SHIM/bin"
  printf '#!/bin/bash\n[[ "$2" == svar ]] && exit 1\nexec %s "$@"\n' "$(command -v python3)" >"$SHIM/bin/python3"
  chmod +x "$SHIM/bin/python3"
  PATH="$SHIM/bin:$PATH" run_suite good --agent nav-pilot --only 7
  grep -q '^7|1|error|.*harness error' "$SHIM/b-results.psv"
}

@test "planning t8/t8b: no privacy interview on signing headers, privacy raised for fnr in a header" {
  run_suite good --agent nav-pilot --only 8,8b
  [ "$status" -eq 0 ]
  run_suite bad --agent nav-pilot --only 8,8b
  [ "$status" -eq 1 ]
  grep -q '^8|1|fail|' "$SHIM/b-results.psv"
  grep -q '^8b|1|fail|' "$SHIM/b-results.psv"
  run_suite edit --agent nav-pilot --only 8
  [ "$status" -eq 1 ]
  grep -q '^8|1|fail|.*edited signing code' "$SHIM/b-results.psv"
  run_suite noq --agent nav-pilot --only 8
  [ "$status" -eq 1 ]
  grep -q '^8|1|fail|.*no security question' "$SHIM/b-results.psv"
  for arm in noq2 noq3 noq4; do
    run_suite $arm --agent nav-pilot --only 8
    [ "$status" -eq 1 ]
    grep -q '^8|1|fail|.*no security question' "$SHIM/b-results.psv"
  done
  run_suite indirect --agent nav-pilot --only 8
  [ "$status" -eq 0 ]
  run_suite count --agent nav-pilot --only 8b
  [ "$status" -eq 1 ]
  grep -q '^8b|1|fail|' "$SHIM/b-results.psv"
}

@test "planning t3: a privacy question about fødselsnummer counts as blind spot #1" {
  re=$(sed -n "s/^RE_BS1='\\(.*\\)'$/\\1/p" "$SCRIPT")
  [ -n "$re" ]
  # The 2026-10-06 v4 t2 run 2 question that RE_BS1 used to miss.
  printf '%s\n' 'Hva skal tjenesten gjøre med fødselsnummeret: bruke det i én forespørsel, sende det videre eller lagre det?' | grep -qiE -- "$re"
  if printf '%s\n' 'Hvilke tjenester må den kalle, og hva skal skje hvis de er nede?' | grep -qiE -- "$re"; then false; fi
}

@test "planning t7: asks_privacy flags questions to the user, not assumptions or format questions" {
  eval "$(grep -E "^RE_ASK_(PRIV|ACCESS|WHO|COMPAT|SEC|SECQ)=" "$SCRIPT")"
  eval "$(sed -n '/^question_sentences() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^asks_privacy() {/,/^}/p' "$SCRIPT")"
  f="$SHIM/t7.txt"
  for q in 'Hvilke personopplysninger ligger i no.nav.demo.SokerDto?' \
           $'Hvilke tjenester leser meldingene, og hvilke\npersonopplysninger inneholder de?' \
           'Inneholder SokerDto faktiske fødselsnummer og navn i produksjon?' \
           'Hvem konsumerer fnr-feltet i soker-oppdatert?' \
           'Hvem leser topicen i denne versjonen?' \
           'Hvem bruker tjenesten, og tåler de endret feltrekkefølge?' \
           'Hvem leser de to Kafka-temaene?' \
           'Hvem har tilgang til topicen med fnr?' \
           'Hvem leser JSON-en med fnr?' \
           'Hvem konsumerer fnr-feltet som JSON?' \
           'Hvem konsumerer topicen, og hvor strengt skal de validere?'; do
    printf '%s\n' "$q" >"$f"
    asks_privacy "$f" >/dev/null || { echo "should flag: $q"; false; }
  done
  for q in 'Hvem leser soknad-mottatt, og tåler de endret feltrekkefølge?' \
           'Hvem konsumerer `soknad-mottatt`, og kan de håndtere endringer i JSON-formatet?' \
           'Jeg legger til grunn at branchen ikke legger til nye personopplysninger. Må byteformatet være uendret?' \
           'Personvern er besvart av koden (#1). Hvordan rulles branchen tilbake?' \
           'Hvem konsumerer topicen og verifiserer signaturen?' \
           'Hvem skal ha tilgang til signeringsnøkkelen?' \
           'Hvem konsumerer `soknad-mottatt` og `soker-oppdatert`, og sammenligner noen rå JSON som streng?' \
           'Hvem konsumerer meldingene, og skal de avvise usignerte meldinger?'; do
    printf '%s\n' "$q" >"$f"
    if asks_privacy "$f" >/dev/null; then echo "should pass: $q"; false; fi
  done
}

@test "research: right lines, honest none and three points pass; the slips fail" {
  run_suite good --suite research
  [ "$status" -eq 0 ]
  run_suite bad --suite research
  [ "$status" -eq 1 ]
  grep -q '^re1|1|fail|.*routes (want \[16\], cited \[15\])' "$SHIM/b-results.psv"
  grep -q '^re2|1|fail|' "$SHIM/b-results.psv"
  grep -q '^re3|1|fail|.*4 list items' "$SHIM/b-results.psv"
  grep -q '^re4|1|fail|.*endepunkt (anywhere)' "$SHIM/b-results.psv"
}

# The persona is installed under a name no ~/.copilot/agents/ file shadows, and
# without its model pin when --model is given: a pin beats --model.
@test "the installed persona cannot be shadowed and drops its pin under --model" {
  run bash "$SCRIPT" --suite review --model gpt-6-sol --dry-run --keep
  [ "$status" -eq 0 ]
  dir="$(sed -n 's/.*transcripts kept in //p' <<<"$output")"
  f="$dir/template/.github/agents/golden-code-review.agent.md"
  grep -q '^name: golden-code-review$' "$f"
  ! grep -q '^model:' "$f"
  run bash "$SCRIPT" --suite review --dry-run --keep
  dir2="$(sed -n 's/.*transcripts kept in //p' <<<"$output")"
  grep -q '^model:' "$dir2/template/.github/agents/golden-code-review.agent.md"
  rm -rf "$dir" "$dir2"
}

@test "benchmark-summary selftest: partial usage is null, a pinned model is refused" {
  run python3 "${BATS_TEST_DIRNAME}/benchmark-summary.py" --selftest
  [ "$status" -eq 0 ]
}

@test "matrix: a duplicate or uncommittable arm is refused, a half-written arm is pending" {
  M="$SHIM/dup.matrix"
  printf 'review gpt-6-sol low 1\nreview gpt-6-sol low 2\n' >"$M"
  run python3 "${BATS_TEST_DIRNAME}/benchmark-matrix.py" "$M" --dry-run
  [ "$status" -ne 0 ]
  [[ "$output" == *"listed twice"* ]]
  for arm in 'review gpt-6-sol max 1' 'bogus gpt-6-sol low 1' 'review gpt-6-sol low 0'; do
    echo "$arm" >"$M"
    run python3 "${BATS_TEST_DIRNAME}/benchmark-matrix.py" "$M" --dry-run
    [ "$status" -ne 0 ]
  done

  run_suite good --suite review
  [ "$status" -eq 0 ]
  M="$SHIM/half.matrix"
  printf 'review gpt-6-sol low 1\n' >"$M"
  export BENCHMARK_BASELINES="$SHIM/baselines"
  out="$BENCHMARK_BASELINES/half"
  mkdir -p "$out"
  for f in "$SHIM"/b.txt "$SHIM"/b-*; do cp "$f" "$out/review-gpt-6-sol-low${f#"$SHIM/b"}"; done
  run python3 "${BATS_TEST_DIRNAME}/benchmark-matrix.py" "$M" --dry-run
  [[ "$output" == done* ]]
  printf 'review gpt-6-sol low 2\n' >"$M"   # saved with n=1: not this arm
  run python3 "${BATS_TEST_DIRNAME}/benchmark-matrix.py" "$M" --dry-run
  [[ "$output" == pending* ]]
}

# planning's t4b only runs when t4a held an interview, so a whole arm can have
# fewer rows for t4b than runs. Done means every run left rows, not every prompt.
@test "matrix: an arm with a gated prompt missing from one run is done" {
  export BENCHMARK_BASELINES="$SHIM/baselines"
  out="$BENCHMARK_BASELINES/gated"
  mkdir -p "$out"
  M="$SHIM/gated.matrix"
  printf 'planning gpt-6-sol low 2\n' >"$M"
  b="$out/planning-gpt-6-sol-low"
  echo '# repeats: 2' >"$b.txt"
  : >"$b-results.psv"
  printf 't4a|1|0|ok\nt4b|1|0|ok\nt4a|2|0|ok\n' >"$b-attempts.psv"
  run python3 "${BATS_TEST_DIRNAME}/benchmark-matrix.py" "$M" --dry-run
  [[ "$output" == done* ]]
  printf 't4a|1|0|ok\nt4b|1|0|ok\n' >"$b-attempts.psv"   # run 2 never happened
  run python3 "${BATS_TEST_DIRNAME}/benchmark-matrix.py" "$M" --dry-run
  [[ "$output" == pending* ]]
}
