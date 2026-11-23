"use client";

import {
  BodyShort,
  Box,
  Button,
  ErrorSummary,
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

const dateFmt = new Intl.DateTimeFormat("nb-NO", { day: "numeric", month: "long" });
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
    <ErrorSummary ref={refEl} heading="Rett dette før du sender:">
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

export function BudgetRaisePage() {
  const [budget, setBudget] = useState<Loaded<Budget>>();
  const [options, setOptions] = useState<Loaded<BudgetOptions>>();
  const [profile, setProfile] = useState<Loaded<BudgetProfile>>();

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
      <Summary budget={b} options={o} budgetError={"error" in budget ? budget.error : undefined} />
      {b && <Consumption budget={b} />}
      {o ? (
        <>
          <RaiseSection budget={b} options={o} profile={p} onRaised={reloadBudget} />
          <ProfileSection
            options={o}
            profile={p}
            profileError={"error" in profile ? profile.error : undefined}
            onSaved={(np) => setProfile({ data: np })}
          />
        </>
      ) : (
        <Message status="warning" title="Påfyll er ikke tilgjengelig akkurat nå">
          {errorMessage("error" in options ? options.error : undefined)}
        </Message>
      )}
    </VStack>
  );
}

function Summary({ budget, options, budgetError }: { budget?: Budget; options?: BudgetOptions; budgetError?: string }) {
  const step = options?.stepUsd ?? 150;
  const cap = options?.capUsd ?? 750;
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
        <List.Item>Går du tom, kan du fylle på {usd(step)} selv. Påfyllet gjelder ut måneden.</List.Item>
        <List.Item>Taket er {usd(cap)} i måneden. 1. i neste måned er du tilbake på standardbudsjettet.</List.Item>
        <List.Item>Skriv aldri personopplysninger eller saksopplysninger i fritekstfeltet.</List.Item>
      </List>
    </VStack>
  );
}

function Consumption({ budget }: { budget: Budget }) {
  const consumed = budget.consumedAmount;
  const pct =
    budget.budgetAmount > 0 && typeof consumed === "number"
      ? Math.min(100, Math.round((consumed / budget.budgetAmount) * 100))
      : null;
  return (
    <VStack gap="space-12">
      <Heading size="small" level="2">
        Forbruk denne måneden
      </Heading>
      {pct !== null && (
        <VStack gap="space-8">
          <ProgressBar
            value={pct}
            valueMax={100}
            data-color={pct >= 90 ? "danger" : pct >= 80 ? "warning" : "accent"}
            aria-labelledby="forbruk-tekst"
          />
          <BodyShort id="forbruk-tekst">
            Du har brukt {usd(consumed!)} av {usd(budget.budgetAmount)} i AI Credits ({pct} %).
          </BodyShort>
        </VStack>
      )}
      {pct === null && <BodyShort>Forbruket er ikke tilgjengelig ennå.</BodyShort>}
      {budget.raises === undefined ? (
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
}: {
  budget?: Budget;
  options: BudgetOptions;
  profile?: BudgetProfile;
  onRaised: () => void;
}) {
  const [category, setCategory] = useState("");
  // Empty means the profile team: the backend fills it in.
  const [teamId, setTeamId] = useState(profile?.teamId ?? "");
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
  const current = budget?.budgetAmount ?? 0;
  const left = budget ? Math.max(0, Math.floor((options.capUsd - current) / options.stepUsd)) : null;

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
        body: JSON.stringify({ category, teamId, expectedValue, freeText }),
      });
      const body = await res.json().catch(() => ({}));
      if (res.ok) {
        setResult({ ok: body as RaiseResult });
        key.current = null;
        setCategory("");
        setExpectedValue("");
        setFreeText("");
        onRaised();
      } else {
        setResult({ error: (body as { error?: string }).error ?? "failed" });
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
            : "Du har nådd taket.")}
      </BodyShort>

      <div ref={resultRef} tabIndex={-1} aria-live="polite">
        {result && "ok" in result && (
          <Message
            status="success"
            title={
              result.ok.dryRun
                ? "Testkjøring: påfyllet er registrert, men ikke sendt til GitHub"
                : "Påfyllet er registrert"
            }
          >
            Nytt budsjett er {usd(result.ok.budgetUsd)} til {dateFmt.format(new Date(result.ok.expiresAt))}. Du kan
            fylle på {result.ok.raisesLeft} {result.ok.raisesLeft === 1 ? "gang" : "ganger"} til denne måneden. Det kan
            ta noen minutter før GitHub bruker det nye budsjettet.
          </Message>
        )}
        {result && "error" in result && (
          <Message
            status={result.error === "cap_reached" || result.error === "not_open" ? "warning" : "error"}
            title="Påfyllet ble ikke gjennomført"
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
              description="Teamet påfyllet føres på. Standard er teamet i profilen din."
              value={teamId}
              onChange={(e) => setTeamId(e.target.value)}
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
  const [teamId, setTeamId] = useState(profile?.teamId ?? "");
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
        Team og rolle brukes til å se hvem som trenger mer, ikke til å vurdere deg. Du må lagre profilen før første
        påfyll.
      </BodyShort>
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
