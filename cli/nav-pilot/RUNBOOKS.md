# Runbooks — nav-pilot Telemetry Incident Response

Operative retningslinjer for håndtering av nav-pilot-telemetrialarmer. Hver runbook: Problem → Rask diagnose → Tiltak → Suksesskriterier.

---

## ⚠️ Metrikk-tilgjengelighet (les først)

nav-pilot CLI emitterer disse metrikkene i dag (se `cli/nav-pilot/telemetry.go`):

| Metrikk | Type | Datapunkt-dimensjoner |
|---------|------|-----------------------|
| `nav_pilot_command_total` | Counter | `command`, `mode`, `scope`, `result`, `version`, `execution_context` |
| `nav_pilot_command_duration_ms` | Histogram | `command`, `mode`, `scope`, `result`, `version`, `execution_context` |
| `nav_pilot_command_error_total` | Counter | `command`, `mode`, `scope`, `version`, `execution_context` |
| `nav_pilot_rtk_setup_total` | Counter | `client`, `choice`, `result`, `version`, `execution_context` |
| `nav_pilot_install_items_total` | Counter | `command`, `mode`, `scope`, `version`, `execution_context` |
| `nav_pilot_sync_updates_total` | Counter | `command`, `mode`, `scope`, `version`, `execution_context` |
| `nav_pilot_sync_conflicts_total` | Counter | `command`, `mode`, `scope`, `version`, `execution_context` |
| `nav_pilot_info` | Gauge | `version`, `device_id`, `execution_context`, `os`, `arch` |
| `nav_pilot_install_present` | Gauge | `scope`, `collection`, `version`, `execution_context` |
| `nav_pilot_installed_items` | Gauge | `scope`, `type`, `status`, `version`, `execution_context` |
| `nav_pilot_staleness_check_total` | Counter | `component`, `scope`, `result`, `version`, `execution_context` |
| `nav_pilot_up_to_date` | Gauge | `component`, `scope`, `version`, `execution_context` |
| `nav_pilot_version_skew_days` | Histogram | `component`, `scope`, `version`, `execution_context` |

Pluss resource-attributtene `service.name`, `service.version`, `os`, `arch`, `device_id`.

**Konsekvens for runbookene under:**
- **Avledede** alarmer (kan bygges i dag fra metrikkene over): Runbook 1 (install-suksessrate)
  og Runbook 5 (sync-konflikter).
- **Ikke implementert ennå** (krever ny instrumentering før alarmene kan eksistere):
  Runbook 2 (`nav_pilot_dryrun_conversion_rate`), Runbook 3
  (`nav_pilot_error_category_total{category=...}` — det finnes ingen feilkategori-dimensjon),
  og Runbook 4 (`nav_pilot_confirmation_abort_rate`). Disse er bevart som
  *design-/fremtidige* runbooks og må ikke konfigureres som live-alarmer mot dagens metrikker.

---

## Runbook 1: Install Success Rate < 85%

**Alert**: `nav_pilot_install_success_rate < 0.85` (sustained 1h)

> **Avledet metrikk** — `install_success_rate` emitteres ikke direkte. Bygg den fra dagens metrikker, f.eks.:
> ```promql
> 1 - (
>   sum(increase(nav_pilot_command_error_total{command="install"}[1h]))
>   /
>   sum(increase(nav_pilot_command_total{command="install"}[1h]))
> )
> ```

**Dette betyr**:
- Installation success rate has dropped below 85%
- Likely indicates a regression in the `install` command
- Could be new bug, configuration issue, or external dependency

### Rask diagnose (< 5 min)

1. **Is this sustained or transient?**
   ```bash
   # Check Grafana dashboard: last 1h vs. last 24h trend
   # Transient spikes (< 15 min): monitor, low urgency
   # Sustained (> 1h): investigate immediately
   ```

2. **Was there a recent code push?**
   ```bash
   # Check git log for installs.go changes in last 2h
   rtk git log --oneline -n 20 -- cli/nav-pilot/install.go
   ```

3. **Which error types are causing failures?**
   - Check Grafana "Command Health" dashboard
   - Look for: conflict (high), permission (high), network (transient)

### Årsakstre

```
Install success < 85%
├─ Recent code change? → Check diff, potential rollback
├─ Conflict errors spiking? → Merge logic issues
├─ Permission errors high? → Scope confusion, auth issue
├─ Network errors? → External service, latency spike
└─ All error types up? → Broader platform issue
```

### Tiltak

**If recent code change (within 2h):**
1. Review diff in `install.go`, `manifest.go`, `resolver.go`
2. If risky change: **revert immediately** (coordinate with team)
3. If not obvious: enable DEBUG logging and deploy to staging

**If conflict errors spiking:**
1. Check `sync.go` conflict resolution logic
2. Review error messages in dashboard
3. Possible fix: improve conflict detection or auto-resolve safe cases
4. Create ticket: "Improve conflict resolution UX"

**If permission errors high:**
1. Check if scope documentation is clear
2. Review error message (is it actionable?)
3. Possible fix: improve error guidance, clarify scope defaults
4. Create ticket: "Improve scope documentation"

**If network errors:**
1. Check upstream service availability (manifest resolver, registry)
2. If external service down: post in #nav-platform and wait
3. If intermittent: likely transient, monitor and document

**General escalation:**
- No obvious cause? → Page on-call engineer
- Taking > 30 min to diagnose? → Escalate to team lead

### Suksesskriterier

- Success rate back to ≥ 85% within **2 hours**
- Root cause documented
- If rollback: deployment procedure logged

### Eskalering

```
Success rate < 85% sustained 1h
  ↓
Follow actions above (max 30 min)
  ↓
No improvement → Page on-call engineer
  ↓
On-call determines: fix in place, or revert to stable version
```

---

## Runbook 2: Dry-Run Conversion < 40%

**Alert**: `nav_pilot_dryrun_conversion_rate < 0.40` (sustained 2h)

> **⚠️ Ikke implementert.** CLI-en instrumenterer ikke dry-run → faktisk kjøring. Denne runbooken
> er en design-skisse; alarmen kan ikke aktiveres før metrikk for dry-run-konvertering legges til.

**Dette betyr**:
- Brukere tester med `--dry-run`, men går ikke videre til reell kjøring
- Indicates low confidence or unmet expectations
- Could be unclear output, safety concerns, or UX friction

### Rask diagnose (< 10 min)

1. **Is this a new regression?**
   - Check last 7 days trend
   - If been low for days: likely structural issue (docs, UX)
   - If sudden drop: likely recent change

2. **Recent error message changes?**
   ```bash
   rtk git log --oneline -n 10 -- cli/nav-pilot/output.go cli/nav-pilot/interactive.go
   ```

3. **Check error context:**
   - Do dry-run outputs contain alarming warnings?
   - Are conflicts shown clearly?
   - Is success message convincing?

### Årsakstre

```
Dry-run conversion < 40%
├─ New feature flag or warning? → Review messaging
├─ Recent UX change? → Check diff, user feedback
├─ High conflict rate? → Users hesitant on conflicts
├─ Network/latency issues? → Slow dry-runs discourage follow-up
└─ Documentation unclear? → Education needed
```

### Tiltak

**If recent change to error messaging:**
1. Review diff in `output.go` or `interactive.go`
2. Assess: is the warning too alarming?
3. Possible fix: soften wording, add reassurance
4. Deploy and monitor conversion rate next day

**If conflict resolution unclear:**
1. Check if dry-run shows conflicts clearly
2. Add reassurance: "Safe conflicts can be auto-resolved"
3. Create ticket: "Auto-resolve safe conflicts in sync"
4. Update docs: "Why conflicts happen and how to resolve"

**If UX friction detected:**
1. Run quick user interview (5 min chat)
2. Ask: "Why didn't you proceed from dry-run?"
3. Document answer
4. Create ticket based on feedback

**If documentation unclear:**
1. Review TELEMETRY.md dry-run explanation
2. Create video demo: "Dry-run → Real Install workflow"
3. Post in #nav-pilot Slack: "Tip: Use --dry-run to test safely"
4. Measure: conversion rate next week

### Suksesskriterier

- Conversion rate back to ≥ 50% within **3 days**
- Root cause identified (UX, docs, or code)
- Action item created if structural fix needed

### Eskalering

```
Dry-run conversion < 40% sustained 2h
  ↓
Quick diagnosis (< 10 min) + document findings
  ↓
If messaging issue → Fix + deploy same day
If UX issue → Create ticket, prioritize next sprint
If docs issue → Update docs + announce
  ↓
Check rate next day
```

---

## Runbook 3: Permission Errors Spike (> 100/day)

**Alert**: `nav_pilot_error_category_total{category="permission"} > 100` in 24h

> **⚠️ Ikke implementert.** Det finnes ingen `nav_pilot_error_category_total`-metrikk og ingen
> feilkategori-dimensjon. `nav_pilot_command_error_total` har kun `command`, `mode`, `scope`,
> `version`, `execution_context`. Tillatelses-/feilkategorisering må legges til før denne alarmen kan brukes.

**Dette betyr**:
- Users encountering permission/scope issues
- Could indicate: scope confusion, documentation gap, or new bug

### Rask diagnose (< 10 min)

1. **Error spike recent or ongoing pattern?**
   - Check 7d trend
   - One-day spike: likely specific incident
   - Gradual rise: documentation gap

2. **Any recent changes to scope handling?**
   ```bash
   rtk git log --oneline -n 5 -- cli/nav-pilot/scope.go
   ```

3. **What does the error message say?**
   - Check dashboard for error text snippets
   - Is it actionable? Clear?

### Årsakstre

```
Permission errors > 100/day
├─ New users not understanding scope? → Docs/education
├─ Scope default change? → Check git history
├─ Git config issue on some machines? → Environment issue
├─ API permissions changed? → Upstream change
└─ Bug in permission checking? → Code issue
```

### Tiltak

**If documentation gap:**
1. Review TELEMETRY.md scope section
2. Add: "Scope defaults to `repo`. Use `--user` for global installs."
3. Create FAQ entry
4. Post in #nav-pilot with clear example

**If specific user cohort affected:**
1. Identifiser hvilke team (`device_id` er en enveis-hash og kan **ikke** mappes til team uten en ekstern, frivillig opt-in-mapping — bruk heller `scope`/`version`/`os`-fordeling for å se mønstre)
2. Reach out: "We noticed permission issues; here's the fix"
3. Offer 1:1 help

**If error message unclear:**
1. Review error text in `scope.go` or permission checking
2. Improve message: include actionable next step
3. Example: "Permission denied. Are you in the right repo? Try: `nav-pilot install --scope user @foo`"
4. Deploy update

**If git config issue:**
1. Document workaround in TELEMETRY.md
2. Post in #nav-pilot: "If you see permission errors, check: `git config --list`"
3. Create support guide

### Suksesskriterier

- Permission error count back to < 50/day within **5 days**
- Error message improved if unclear
- Documentation updated if gap

### Eskalering

```
Permission errors > 100/day
  ↓
Diagnose: is it docs, UX, or code? (< 10 min)
  ↓
If docs/education → Update + announce same day
If error message unclear → Improve + deploy within 24h
If code bug → Create ticket, investigate next sprint
  ↓
Følg med neste uke
```

---

## Runbook 4: Confirmation Abort Rate > 25%

**Alert**: `nav_pilot_confirmation_abort_rate > 0.25` (sustained 2h)

> **⚠️ Ikke implementert.** CLI-en instrumenterer ikke bekreftelses-prompt/abort. Denne runbooken
> er en design-skisse; alarmen kan ikke aktiveres før metrikk for bekreftelser legges til.

**Dette betyr**:
- Users are declining confirmation prompts at high rate (> 25%)
- Indicates: prompt fatigue, unclear risk, or unnecessary barriers

### Rask diagnose (< 5 min)

1. **When did this spike start?**
   - Recent change to prompts?
   ```bash
   rtk git log --oneline -n 5 -- cli/nav-pilot/interactive.go
   ```

2. **Which action is being declined?**
   - `install` confirmations?
   - `sync --force` confirmations?
   - Check metrics by action

3. **Is the prompt wording clear?**
   - Review in `interactive.go`
   - Is risk/consequence obvious?

### Årsakstre

```
Confirmation abort > 25%
├─ New confirmation added? → Evaluate necessity
├─ Wording unclear/alarming? → Improve phrasing
├─ Too many confirmations? → Reduce/batch
├─ Safe actions prompted unnecessarily? → Remove prompt
└─ Users just learning → New user cohort (expected)
```

### Tiltak

**If new confirmation added:**
1. Evaluate: is this really necessary?
2. Alternatives: `--yes` flag to skip, smart defaults
3. Consider: batch confirmations or single approval

**If wording unclear:**
1. Improve prompt to be clearer: "This will update 5 agents. Continue? [y/N]"
2. Add reassurance: "(You can undo with 'nav-pilot sync --revert')"
3. Deploy update

**If too many confirmations:**
1. Count confirmations per session
2. Consolidate: batch multiple actions
3. Add `--yes` flag for automated contexts
4. Document: "Use `--yes` in CI pipelines"

**If safe operations prompted:**
1. Re-evaluate: does this action really need confirmation?
2. Example: `list` operations shouldn't prompt
3. Remove unnecessary confirmation
4. Deploy

### Suksesskriterier

- Abort rate back to < 15% within **1 day**
- Prompt wording evaluated
- `--yes` flag added if needed

### Eskalering

```
Confirmation abort > 25% sustained 2h
  ↓
Diagnose: is this prompt necessary? (< 5 min)
  ↓
Unnecessary → Remove + deploy same day
Poor wording → Improve + deploy same day
Too many → Batch or add --yes flag
  ↓
Følg med neste vakt
```

---

## Runbook 5: Sync Conflicts > 50/hour

**Alert**: `nav_pilot_sync_conflicts_total > 50` in 1h window

**Dette betyr**:
- High rate of merge conflicts being detected
- Could indicate: complex manifest changes, or merge logic issues
- Users may be aborting due to complexity

### Rask diagnose (< 10 min)

1. **Is this a spike or new pattern?**
   - Check 24h trend
   - Spike: likely specific change or event
   - Gradual: merge logic issue

2. **Recent changes to merge algorithm?**
   ```bash
   rtk git log --oneline -n 5 -- cli/nav-pilot/sync.go cli/nav-pilot/manifest.go
   ```

3. **Are users aborting on conflicts?**
   - Check error recovery metrics
   - High conflict abort rate?

### Årsakstre

```
Sync conflicts > 50/hour
├─ Merge algorithm too strict? → Relax rules
├─ User edits conflicting? → Expected, communicate
├─ Automatic merge failing? → Improve logic
├─ New conflict detection added? → Evaluate thresholds
└─ Manifest schema ambiguity? → Clarify specs
```

### Tiltak

**If merge algorithm too strict:**
1. Review `sync.go` conflict detection
2. Identify: what's being flagged as conflict?
3. Can we auto-resolve safe cases?
4. Create ticket: "Auto-resolve non-destructive conflicts"

**If conflicts are legitimate:**
1. This is expected in collaborative environments
2. Communicate: "Conflicts are normal; resolve with: `nav-pilot sync --interactive`"
3. Improve UX: better conflict resolution interface
4. Add docs: "Conflict resolution guide"

**If conflict abort rate high:**
1. Simplify merge UX
2. Add suggestions: "Auto-resolve this safely? [y/N]"
3. Create dashboard: "How to resolve conflicts"

**If metric threshold too low:**
1. Evaluate: is 50/hour too aggressive?
2. Baseline: what's normal for our user base?
3. Adjust threshold if necessary (discuss with team)

### Suksesskriterier

- Conflict detection rate < 30/hour OR abort rate < 40%
- Merge UX improved if necessary
- Documentation updated

### Eskalering

```
Sync conflicts > 50/hour
  ↓
Is this expected growth, or regression? (< 10 min diagnosis)
  ↓
Expected → Improve UX, create guide
Regression → Investigate recent changes, rollback if necessary
  ↓
Følg med neste uke for trender
```

---

## Generell eskaleringsmatrise

| Severity | Duration | Action |
|----------|----------|--------|
| CRITICAL | > 1h | Page on-call immediately; consider rollback |
| HIGH | > 2h | Escalate to team lead; fix same day |
| MEDIUM | > 4h | Create ticket; prioritize next sprint |
| LOW | Ongoing | Document; low priority fix |

---

## FAQ

**Q: I see an alert but don't understand it. What do I do?**  
A: 1. Les relevant runbook over. 2. Følg seksjonen "Rask diagnose" (< 10 min). 3. Hvis det fortsatt er uklart, varsle on-call-lead.

**Q: When should I wake someone up?**  
A: Page on-call if: Success rate < 80% OR can't diagnose within 15 minutes.

**Q: Can I just ignore a low-priority alert?**  
A: Only if it's been ongoing for days with no user impact. Otherwise, create a ticket to investigate.

**Q: How do I know if this is my problem or an upstream issue?**  
A: Check: Are other commands working? Is it specific to one team or global? If global + not in our code, check #nav-platform Slack.

---

**Sist oppdatert**: 2026-06-15  
**Runbook-ansvarlig**: @nav-pilot-team  
**Questions?**: Post in #nav-pilot or create issue in navikt/copilot
