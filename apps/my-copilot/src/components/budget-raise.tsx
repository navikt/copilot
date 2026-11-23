"use client";

import {
  BodyShort,
  Box,
  Button,
  ErrorSummary,
  Detail,
  HGrid,
  Heading,
  List,
  LocalAlert,
  ProgressBar,
  Select,
  Skeleton,
  Table,
  Textarea,
  VStack,
} from "@navikt/ds-react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  type Budget,
  type BudgetOptions,
  type BudgetProfile,
  type RaiseResult,
  errorMessage,
  hasOwnBudget,
  raisesLeft,
  label,
  statusLabel,
  usd,
} from "@/lib/budget-raise";

// "AI Credits" stays as GitHub's name for the billing unit (ORDBOK.md).

type Loaded<T> = { data: T } | { error: string };

async function load<T>(url: string): Promise<Loaded<T>> {
  try {
    const res = await fetch(url, { cache: "no-store" });
    const body = await res.json().catch(() => ({}));
    return res.ok ? { data: body as T } : { error: (body as { error?: string }).error ?? "failed" };
  } catch {
    return { error: "failed" };
  }
}

const timeFmt = new Intl.DateTimeFormat("nb-NO", {
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
});

type FieldErrors = Partial<Record<string, string>>;

function Errors({ errors, refEl }: { errors: FieldErrors; refEl: React.RefObject<HTMLDivElement | null> }) {
  const entries = Object.entries(errors).filter(([, v]) => v);
  if (entries.length === 0) return null;
  return (
    <ErrorSummary ref={refEl} heading="Rett dette før du sender inn:">
      {entries.map(([id, msg]) => (
        <ErrorSummary.Item key={id} href={`#${id}`}>
          {msg}
        </ErrorSummary.Item>
      ))}
    </ErrorSummary>
  );
}

function Message({
  status,
  title,
  children,
}: {
  status: "success" | "warning" | "error" | "announcement";
  title: string;
  children?: React.ReactNode;
}) {
  return (
    <LocalAlert status={status}>
      <LocalAlert.Header>
        <LocalAlert.Title as="h3">{title}</LocalAlert.Title>
      </LocalAlert.Header>
      {children && <LocalAlert.Content>{children}</LocalAlert.Content>}
    </LocalAlert>
  );
}

function Panel({ children }: { children: React.ReactNode }) {
  return (
    <Box
      as="section"
      background="default"
      borderColor="neutral-subtle"
      borderWidth="1"
      borderRadius="12"
      padding={{ xs: "space-16", md: "space-24" }}
    >
      {children}
    </Box>
  );
}

function Stat({ title, value, note }: { title: string; value: string; note?: string }) {
  return (
    <Box background="neutral-soft" borderRadius="8" padding="space-16" className="min-w-0">
      <VStack gap="space-4">
        <Detail textColor="subtle">{title}</Detail>
        <BodyShort size="large" weight="semibold">
          {value}
        </BodyShort>
        {note && <Detail textColor="subtle">{note}</Detail>}
      </VStack>
    </Box>
  );
}

export function BudgetRaisePage() {
  const [budget, setBudget] = useState<Loaded<Budget>>();
  const [options, setOptions] = useState<Loaded<BudgetOptions>>();
  const [profile, setProfile] = useState<Loaded<BudgetProfile>>();
  // Set from a refused raise, so «Kort fortalt» matches what the form just said.
  const [refused, setRefused] = useState<string>();

  const reloadBudget = useCallback(() => load<Budget>("/api/budget").then(setBudget), []);
  useEffect(() => {
    reloadBudget();
    load<BudgetOptions>("/api/budget/options").then(setOptions);
    load<BudgetProfile>("/api/budget/profile").then(setProfile);
  }, [reloadBudget]);

  if (!budget || !options || !profile) {
    return (
      <VStack gap="space-8" aria-busy>
        <Skeleton variant="text" width="12rem" />
        <Skeleton variant="rectangle" height="8rem" />
      </VStack>
    );
  }

  const b = "data" in budget ? budget.data : undefined;
  const o = "data" in options ? options.data : undefined;
  const p = "data" in profile ? profile.data : undefined;

  return (
    <VStack gap="space-32">
      <Summary budget={b} options={o} budgetError={"error" in budget ? budget.error : undefined} refused={refused} />
      {b && <Consumption budget={b} capUsd={o?.capUsd} />}
      {o ? (
        <>
          <Panel>
            <RaiseSection budget={b} options={o} profile={p} onRaised={reloadBudget} onRefused={setRefused} />
          </Panel>
          <Panel>
            <ProfileSection
              options={o}
              profile={p}
              profileError={"error" in profile ? profile.error : undefined}
              onSaved={(np) => setProfile({ data: np })}
            />
          </Panel>
        </>
      ) : (
        <Message status="warning" title="Påfyll er ikke tilgjengelig akkurat nå">
          {"error" in options && options.error !== "budget_writes_unavailable"
            ? errorMessage(options.error)
            : "Du kan ikke fylle på eller endre profilen nå. Prøv igjen om litt."}
        </Message>
      )}
    </VStack>
  );
}

function Summary({
  budget,
  options,
  budgetError,
  refused,
}: {
  budget?: Budget;
  options?: BudgetOptions;
  budgetError?: string;
  refused?: string;
}) {
  const step = options?.stepUsd ?? 150;
  const cap = options?.capUsd ?? 750;
  const state =
    refused === "own_budget" || (budget && hasOwnBudget(budget))
      ? "own"
      : refused === "cap_reached" || (options && raisesLeft(budget, options) === 0)
        ? "cap"
        : refused === "not_open"
          ? "closed"
          : !options
            ? "down"
            : "open";
  const raiseLine = {
    own: "Du har et eget, fast budsjett, så påfyll gjelder ikke for deg.",
    cap: "Du har nådd taket. 1. i neste måned er du tilbake på standardbudsjettet.",
    closed: `Påfyll er ikke åpnet ennå. Når det åpner, kan du fylle på ${usd(step)} selv.`,
    down: "Påfyll er ikke tilgjengelig akkurat nå. Prøv igjen om litt.",
    open: `Går du tom, kan du fylle på ${usd(step)} selv. Påfyllet gjelder ut måneden.`,
  }[state];
  return (
    <VStack gap="space-12">
      <Heading size="medium" level="2" id="kort-fortalt">
        Kort fortalt
      </Heading>
      <List>
        {budget ? (
          <List.Item>
            Budsjettet ditt denne måneden er <strong>{usd(budget.budgetAmount)}</strong>
            {typeof budget.consumedAmount === "number" && <>, og du har brukt {usd(budget.consumedAmount)}</>}.
          </List.Item>
        ) : (
          <List.Item>Vi får ikke hentet budsjettet ditt akkurat nå. {errorMessage(budgetError)}</List.Item>
        )}
        <List.Item>{raiseLine}</List.Item>
        {state !== "own" && state !== "cap" && (
          <List.Item>Taket er {usd(cap)} i måneden. 1. i neste måned er du tilbake på standardbudsjettet.</List.Item>
        )}
        {state !== "own" && (
          <List.Item>Skriv aldri personopplysninger eller saksopplysninger i fritekstfeltet.</List.Item>
        )}
      </List>
    </VStack>
  );
}

function Consumption({ budget, capUsd }: { budget: Budget; capUsd?: number }) {
  const consumed = budget.consumedAmount;
  const pct =
    budget.budgetAmount > 0 && typeof consumed === "number"
      ? Math.min(100, Math.round((consumed / budget.budgetAmount) * 100))
      : null;
  const done = budget.raises?.length;
  return (
    <VStack gap="space-16">
      <Heading size="small" level="2">
        Forbruk denne måneden
      </Heading>
      <HGrid columns={{ xs: 2, md: 4 }} gap="space-12">
        <Stat title="Budsjett" value={usd(budget.budgetAmount)} note={capUsd ? `Tak ${usd(capUsd)}` : undefined} />
        <Stat title="Brukt" value={typeof consumed === "number" ? usd(consumed) : "–"} />
        <Stat
          title="Igjen"
          value={typeof consumed === "number" ? usd(Math.max(0, budget.budgetAmount - consumed)) : "–"}
        />
        <Stat title="Påfyll denne måneden" value={done === undefined ? "–" : String(done)} />
      </HGrid>
      {pct !== null && (
        <VStack gap="space-8">
          <ProgressBar
            value={pct}
            valueMax={100}
            // Calm until close to the limit: warning from 80 %, danger only from 95 %.
            data-color={pct >= 95 ? "danger" : pct >= 80 ? "warning" : "accent"}
            aria-labelledby="forbruk-tekst"
          />
          <BodyShort id="forbruk-tekst">
            Du har brukt {usd(consumed!)} av {usd(budget.budgetAmount)} i AI Credits ({pct} %).
          </BodyShort>
        </VStack>
      )}
      {pct === null && <BodyShort>Forbruket er ikke tilgjengelig ennå.</BodyShort>}
      {!budget.raises ? (
        <BodyShort>Vi får ikke vist påfyllene dine akkurat nå.</BodyShort>
      ) : budget.raises.length === 0 ? (
        <BodyShort>Du har ikke fylt på denne måneden.</BodyShort>
      ) : (
        <Table size="small">
          <caption className="sr-only">Påfyll denne måneden</caption>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell scope="col">Tidspunkt</Table.HeaderCell>
              <Table.HeaderCell scope="col">Beløp</Table.HeaderCell>
              <Table.HeaderCell scope="col">Kategori</Table.HeaderCell>
              <Table.HeaderCell scope="col">Forventet verdi</Table.HeaderCell>
              <Table.HeaderCell scope="col">Status</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {budget.raises.map((r) => (
              <Table.Row key={r.id}>
                <Table.DataCell>{timeFmt.format(new Date(r.createdAt))}</Table.DataCell>
                <Table.DataCell>{usd(r.amountUsd)}</Table.DataCell>
                <Table.DataCell>{label(r.category)}</Table.DataCell>
                <Table.DataCell>{label(r.expectedValue)}</Table.DataCell>
                <Table.DataCell>{statusLabel(r.status)}</Table.DataCell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table>
      )}
    </VStack>
  );
}

function RaiseSection({
  budget,
  options,
  profile,
  onRaised,
  onRefused,
}: {
  budget?: Budget;
  options: BudgetOptions;
  profile?: BudgetProfile;
  onRaised: () => void;
  onRefused: (code: string) => void;
}) {
  const [category, setCategory] = useState("");
  // Only a team the user picked here; otherwise the backend uses the profile team, also after a profile change.
  const [teamOverride, setTeamOverride] = useState("");
  const teamId = teamOverride || profile?.teamId || "";
  const [expectedValue, setExpectedValue] = useState("");
  const [freeText, setFreeText] = useState("");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ ok: RaiseResult } | { error: string }>();
  // One key per raise, kept across retries of the same raise so a double click or retry gives one raise (F9).
  const key = useRef<string | null>(null);
  const summaryRef = useRef<HTMLDivElement>(null);
  const resultRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (Object.keys(errors).length) summaryRef.current?.focus();
  }, [errors]);
  useEffect(() => {
    if (result) resultRef.current?.focus();
  }, [result]);

  const ownBudget = budget ? hasOwnBudget(budget) : null;
  const left = raisesLeft(budget, options);

  const heading = (
    <Heading size="small" level="2" id="pafyll">
      Be om påfyll
    </Heading>
  );

  if (ownBudget) {
    return (
      <VStack gap="space-12">
        {heading}
        <Message status="announcement" title="Påfyll gjelder ikke for deg">
          {errorMessage("own_budget")}
        </Message>
      </VStack>
    );
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const next: FieldErrors = {};
    if (!category) next["pafyll-kategori"] = "Velg hva du skal bruke påfyllet til.";
    if (!expectedValue) next["pafyll-verdi"] = "Velg hva du forventer å få ut av det.";
    if (freeText.length > options.freeTextMax)
      next["pafyll-fritekst"] = `Fritekst kan være høyst ${options.freeTextMax} tegn.`;
    setErrors(next);
    if (Object.keys(next).length) return;

    key.current ??= crypto.randomUUID();
    setBusy(true);
    try {
      const res = await fetch("/api/budget/raises", {
        method: "POST",
        headers: { "Content-Type": "application/json", "Idempotency-Key": key.current },
        body: JSON.stringify({ category, teamId: teamOverride || undefined, expectedValue, freeText }),
      });
      const body = await res.json().catch(() => ({}));
      if (res.ok) {
        const ok = body as RaiseResult;
        setResult({ ok });
        // Pending: GitHub's answer is unknown, so a retry must replay this raise, not start a new one.
        if (ok.raise?.status !== "pending") key.current = null;
        setCategory("");
        setExpectedValue("");
        setFreeText("");
        onRaised();
      } else {
        const code = (body as { error?: string }).error ?? "failed";
        setResult({ error: code });
        onRefused(code);
        // A definite answer ends this raise; a network or server failure keeps the key for the retry.
        if (res.status < 500) key.current = null;
      }
    } catch {
      setResult({ error: "failed" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <VStack gap="space-16">
      {heading}
      <BodyShort>
        Hvert påfyll gir {usd(options.stepUsd)} ekstra ut måneden. Taket er {usd(options.capUsd)}.{" "}
        {left !== null &&
          (left > 0
            ? `Du kan fylle på ${left} ${left === 1 ? "gang" : "ganger"} til denne måneden.`
            : "Du har nådd taket. 1. i neste måned er du tilbake på standardbudsjettet.")}
      </BodyShort>

      <div ref={resultRef} tabIndex={-1} aria-live="polite">
        {result && "ok" in result && <RaiseOutcome result={result.ok} />}
        {result && "error" in result && (
          <Message
            status={result.error === "cap_reached" || result.error === "not_open" ? "warning" : "error"}
            title="Du fikk ikke påfyll"
          >
            {errorMessage(result.error)}
          </Message>
        )}
      </div>

      {left !== 0 && (
        <form onSubmit={submit} noValidate>
          <VStack gap="space-20" className="max-w-xl">
            <Errors errors={errors} refEl={summaryRef} />
            <Select
              id="pafyll-kategori"
              label="Hva skal du bruke påfyllet til?"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              error={errors["pafyll-kategori"]}
            >
              <option value="">Velg kategori</option>
              {options.categories.map((c) => (
                <option key={c} value={c}>
                  {label(c)}
                </option>
              ))}
            </Select>
            <Select
              id="pafyll-team"
              label="Team"
              description="Påfyllet registreres på dette teamet. Standard er teamet i profilen din."
              value={teamId}
              onChange={(e) => setTeamOverride(e.target.value)}
            >
              {!teamId && <option value="">Bruk teamet i profilen</option>}
              {options.teams.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </Select>
            <Select
              id="pafyll-verdi"
              label="Hva forventer du å få ut av det?"
              value={expectedValue}
              onChange={(e) => setExpectedValue(e.target.value)}
              error={errors["pafyll-verdi"]}
            >
              <option value="">Velg forventet verdi</option>
              {options.expectedValues.map((v) => (
                <option key={v} value={v}>
                  {label(v)}
                </option>
              ))}
            </Select>
            <VStack gap="space-8">
              <Message status="warning" title="Ikke skriv personopplysninger eller saksopplysninger">
                Teksten lagres sammen med påfyllet. Skriv om arbeidet, ikke om brukere, saker eller kolleger.
              </Message>
              <Textarea
                id="pafyll-fritekst"
                label="Fortell mer (valgfritt)"
                description="For eksempel hva du jobber med eller hvorfor du trenger mer nå."
                value={freeText}
                onChange={(e) => setFreeText(e.target.value)}
                maxLength={options.freeTextMax}
                error={errors["pafyll-fritekst"]}
              />
            </VStack>
            <div>
              <Button type="submit" loading={busy}>
                Fyll på {usd(options.stepUsd)}
              </Button>
            </div>
          </VStack>
        </form>
      )}
    </VStack>
  );
}

function ProfileSection({
  options,
  profile,
  profileError,
  onSaved,
}: {
  options: BudgetOptions;
  profile?: BudgetProfile;
  profileError?: string;
  onSaved: (p: BudgetProfile) => void;
}) {
  const [teamId, setTeamId] = useState(profile?.teamId || profile?.suggestedTeamId || "");
  const [role, setRole] = useState(profile?.role ?? "");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<"saved" | { error: string }>();
  const summaryRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (Object.keys(errors).length) summaryRef.current?.focus();
  }, [errors]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    const next: FieldErrors = {};
    if (!teamId) next["profil-team"] = "Velg team.";
    if (!role) next["profil-rolle"] = "Velg rolle.";
    setErrors(next);
    if (Object.keys(next).length) return;
    setBusy(true);
    try {
      const res = await fetch("/api/budget/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ teamId, role }),
      });
      const body = await res.json().catch(() => ({}));
      if (res.ok) {
        setResult("saved");
        onSaved(body as BudgetProfile);
      } else setResult({ error: (body as { error?: string }).error ?? "failed" });
    } catch {
      setResult({ error: "failed" });
    } finally {
      setBusy(false);
    }
  }

  return (
    <VStack gap="space-16">
      <Heading size="small" level="2" id="profil">
        Profil
      </Heading>
      <BodyShort>
        Vi bruker team og rolle til å se hvor behovet er størst, ikke til å vurdere deg. Du må lagre profilen før første
        påfyll.
      </BodyShort>
      {profile?.leftTeamId && (
        <Message status="announcement" title="Du er ikke lenger med i teamet du lagret">
          Vi fant deg ikke i teamet «{profile.leftTeamId}» lenger, så påfyll registreres på «Annet/ukjent». Velg et av
          teamene dine og lagre profilen på nytt.
        </Message>
      )}
      {profileError && (
        <Message status="warning" title="Vi får ikke hentet profilen din">
          {errorMessage(profileError)}
        </Message>
      )}
      <div aria-live="polite">
        {result === "saved" && <Message status="success" title="Profilen er lagret" />}
        {result && result !== "saved" && (
          <Message status="error" title="Profilen ble ikke lagret">
            {errorMessage(result.error)}
          </Message>
        )}
      </div>
      <form onSubmit={save} noValidate>
        <Box className="max-w-xl">
          <VStack gap="space-20">
            <Errors errors={errors} refEl={summaryRef} />
            <Select
              id="profil-team"
              label="Team"
              description="Bare teamene du er med i på GitHub vises. Finner du ikke teamet ditt, velg «Annet/ukjent»."
              value={teamId}
              onChange={(e) => setTeamId(e.target.value)}
              error={errors["profil-team"]}
            >
              <option value="">Velg team</option>
              {options.teams.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </Select>
            <Select
              id="profil-rolle"
              label="Rolle"
              value={role}
              onChange={(e) => setRole(e.target.value)}
              error={errors["profil-rolle"]}
            >
              <option value="">Velg rolle</option>
              {options.roles.map((r) => (
                <option key={r} value={r}>
                  {label(r)}
                </option>
              ))}
            </Select>
            <div>
              <Button type="submit" variant="secondary" loading={busy}>
                Lagre profil
              </Button>
            </div>
          </VStack>
        </Box>
      </form>
    </VStack>
  );
}

/** What a 200/201 from POST raises means. Success only when GitHub has the new budget (status applied). */
function RaiseOutcome({ result }: { result: RaiseResult }) {
  const status = result.dryRun ? "dry_run" : (result.raise?.status ?? "applied");
  if (status === "dry_run") {
    return (
      <Message status="announcement" title="Testkjøring: ingenting er endret i GitHub">
        Påfyllet er lagret som en test. Budsjettet ditt i GitHub er det samme som før.
      </Message>
    );
  }
  if (status === "pending") {
    return (
      <Message status="announcement" title="Påfyllet venter på GitHub">
        Vi har registrert påfyllet, men vet ikke ennå om GitHub har tatt det i bruk. Se statusen i tabellen over om noen
        minutter. Sender du skjemaet igjen, gir det ikke et nytt påfyll.
      </Message>
    );
  }
  if (status === "failed") {
    return (
      <Message status="error" title="Påfyllet feilet">
        {statusLabel("failed")}
      </Message>
    );
  }
  if (status !== "applied") {
    return (
      <Message status="warning" title="Påfyllet er ikke lagt inn">
        Status: {statusLabel(status)}.
      </Message>
    );
  }
  if (result.replayed || result.budgetUsd === undefined) {
    return (
      <Message status="success" title="Påfyllet er allerede registrert">
        Du har sendt det samme påfyllet før. Du får ikke et nytt påfyll av å sende det igjen.
      </Message>
    );
  }
  const left = result.raisesLeft ?? 0;
  return (
    <Message status="success" title="Du har fått påfyll">
      Budsjettet ditt er nå {usd(result.budgetUsd)} ut måneden. Du kan fylle på {left} {left === 1 ? "gang" : "ganger"}{" "}
      til. Det kan ta noen minutter før GitHub bruker det nye budsjettet.
    </Message>
  );
}
