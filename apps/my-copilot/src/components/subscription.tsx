"use client";

import React, { useState, useEffect } from "react";
import {
  Button,
  Alert,
  Box,
  VStack,
  HGrid,
  Heading,
  BodyShort,
  Detail,
  Link,
  Tag,
  Skeleton,
  ProgressBar,
} from "@navikt/ds-react";
import { User } from "@/lib/auth";
import { formatNumber } from "@/lib/format";
import type { UserMetricsSummary, DailyCredits } from "@/lib/types";
import dynamic from "next/dynamic";

const DailyCreditsChart = dynamic(() => import("@/components/charts/DailyCreditsChart"), { ssr: false });

interface BudgetData {
  budgetAmount: number;
  consumedAmount: number | null;
  isOverride: boolean;
  defaultBudget: number;
}

interface SubscriptionDetailsProps {
  icanhazcopilot: boolean;
  subscription: {
    plan_type: string;
    pending_cancellation_date: string | null;
    updated_at: string | null;
    last_activity_at: string | null;
    last_activity_editor: string;
  };
  githubUsername: string | null;
}

async function updateCopilotSubscription(action: "activate" | "deactivate") {
  return await fetch("/api/copilot", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ action }),
  });
}

const SubscriptionActionButton: React.FC<{
  subscription: SubscriptionDetailsProps["subscription"] | null;
  onClick: () => void;
  isLoading?: boolean;
}> = ({ subscription, onClick, isLoading }) => {
  let buttonColor:
    | "secondary"
    | "primary"
    | "primary-neutral"
    | "secondary-neutral"
    | "tertiary"
    | "tertiary-neutral"
    | "danger"
    | undefined = "secondary";
  let buttonText: string;
  let disabled = false;

  if (subscription?.pending_cancellation_date) {
    // IMPORTANT: Button MUST be disabled when cancellation is pending.
    // Without this, clicking the "danger" button would POST /activate (because
    // the deactivate branch requires !pending_cancellation_date), silently
    // re-activating the seat and cancelling the cancellation with zero feedback.
    buttonColor = "secondary-neutral";
    buttonText = "Avslutter Copilot…";
    disabled = true;
  } else if (subscription?.updated_at) {
    buttonColor = "danger";
    buttonText = "Deaktiver Copilot";
  } else {
    buttonColor = "primary";
    buttonText = "Aktiver Copilot";
  }

  return (
    <Button variant={buttonColor} onClick={onClick} disabled={disabled || isLoading} loading={isLoading}>
      {buttonText}
    </Button>
  );
};

const SubscriptionDetails: React.FC<{ user: User; showGroups?: boolean }> = ({ user, showGroups = false }) => {
  const [loading, setLoading] = useState<boolean>(true);
  const [eligibility, setEligible] = useState<boolean>(false);
  const [subscription, setCopilotSubscription] = useState<SubscriptionDetailsProps["subscription"] | null>(null);
  const [githubUsername, setGitHubUsername] = useState<string | null>(null);
  const [subscriptionError, setSubscriptionError] = useState<string | null>(null);
  const [errorTraceId, setErrorTraceId] = useState<string | null>(null);
  const [needsGitHubLink, setNeedsGitHubLink] = useState<boolean>(false);
  const [budget, setBudget] = useState<BudgetData | null>(null);
  const [usageMetrics, setUsageMetrics] = useState<UserMetricsSummary | null>(null);
  const [dailyCredits, setDailyCredits] = useState<DailyCredits[] | null>(null);
  const [mutating, setMutating] = useState<boolean>(false);
  const [mutationError, setMutationError] = useState<string | null>(null);

  const fetchSubscription = async () => {
    setLoading(true);
    try {
      const response = await fetch("/api/copilot");
      const data = await response.json();

      if (!response.ok || data.error) {
        setSubscriptionError(data.error ?? "Feil ved henting av abonnement");
        setErrorTraceId(data.traceId ?? null);
        return;
      }

      if (data.githubAccountLinked === false) {
        setNeedsGitHubLink(true);
        setEligible(data.icanhazcopilot);
        return;
      }

      setSubscriptionError(null);
      setErrorTraceId(null);
      setNeedsGitHubLink(false);
      setEligible(data.icanhazcopilot);
      setCopilotSubscription(data.subscription);
      setGitHubUsername(data.githubUsername);
    } catch (error) {
      console.error("Error fetching subscription details:", error);
      setErrorTraceId(null);
      if (error instanceof Error) {
        setSubscriptionError(error.message);
      } else {
        setSubscriptionError("Ukjent feil ved henting av abonnement");
      }
    } finally {
      setLoading(false);
    }
  };

  const handleClick = async () => {
    if (!eligibility || mutating) return;
    // Never allow mutation when cancellation is pending — the button should be
    // disabled, but guard here as defense-in-depth.
    if (subscription?.pending_cancellation_date) return;

    const action = subscription?.updated_at ? "deactivate" : "activate";
    setMutating(true);
    setMutationError(null);

    try {
      const response = await updateCopilotSubscription(action);
      const data = await response.json();
      if (!response.ok || data.error) {
        setMutationError(data.error ?? `Feil ved ${action === "activate" ? "aktivering" : "deaktivering"} av Copilot`);
      }
    } catch (error) {
      console.error("Error:", error);
      setMutationError("Nettverksfeil — prøv igjen");
    } finally {
      await fetchSubscription();
      setMutating(false);
    }
  };

  useEffect(() => {
    let cancelled = false;

    async function loadSubscription() {
      const [subscriptionResult, budgetResult] = await Promise.allSettled([
        fetch("/api/copilot"),
        fetch("/api/budget"),
      ]);

      if (cancelled) return;

      let resolvedUsername: string | null = null;

      // Handle subscription independently
      if (subscriptionResult.status === "fulfilled") {
        try {
          const data = await subscriptionResult.value.json();
          if (data.error) {
            setSubscriptionError(data.error);
            setErrorTraceId(data.traceId ?? null);
          } else if (data.githubAccountLinked === false) {
            setNeedsGitHubLink(true);
            setEligible(data.icanhazcopilot);
          } else {
            setSubscriptionError(null);
            setErrorTraceId(null);
            setNeedsGitHubLink(false);
            setEligible(data.icanhazcopilot);
            setCopilotSubscription(data.subscription);
            setGitHubUsername(data.githubUsername);
            resolvedUsername = data.githubUsername;
          }
        } catch {
          setSubscriptionError("Ukjent feil ved henting av abonnement");
        }
      } else {
        setSubscriptionError("Ukjent feil ved henting av abonnement");
      }

      // Handle budget independently — always attempted regardless of subscription outcome
      if (budgetResult.status === "fulfilled" && budgetResult.value.ok) {
        try {
          const budgetData = await budgetResult.value.json();
          if (!cancelled) setBudget(budgetData);
        } catch {
          // Budget parse failure — leave budget as null, card shows fallback text
        }
      }

      // Fetch usage metrics and daily credits if we have a GitHub username
      if (resolvedUsername) {
        const [usageRes, creditsRes] = await Promise.allSettled([
          fetch(`/api/usage?username=${encodeURIComponent(resolvedUsername)}`),
          fetch(`/api/credits?username=${encodeURIComponent(resolvedUsername)}`),
        ]);
        if (usageRes.status === "fulfilled" && usageRes.value.ok && !cancelled) {
          try {
            setUsageMetrics(await usageRes.value.json());
          } catch {
            /* ignore */
          }
        }
        if (creditsRes.status === "fulfilled" && creditsRes.value.ok && !cancelled) {
          try {
            setDailyCredits(await creditsRes.value.json());
          } catch {
            /* ignore */
          }
        }
      }

      if (!cancelled) setLoading(false);
    }

    loadSubscription();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      {subscriptionError && (
        <Box paddingBlock="space-8">
          <Alert variant="error">
            {subscriptionError}
            {errorTraceId && (
              <BodyShort size="small" spacing>
                Sporings-ID: <code>{errorTraceId}</code>
              </BodyShort>
            )}
          </Alert>
        </Box>
      )}

      {needsGitHubLink && (
        <Box paddingBlock="space-8">
          <Alert variant="warning">
            <Heading size="small" level="3" spacing>
              GitHub-kontoen din er ikke koblet til Nav
            </Heading>
            <BodyShort spacing>
              For å bruke Copilot må GitHub-kontoen din være koblet til <strong>navikt</strong>-organisasjonen. Logg inn
              via GitHub SSO for å koble kontoen din.
            </BodyShort>
            <Link href="https://github.com/orgs/navikt/sso" target="_blank">
              Koble til GitHub-kontoen via SSO →
            </Link>
          </Alert>
        </Box>
      )}

      <VStack gap="space-16">
        <HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="space-8">
          <Box padding="space-8" borderRadius="8" className="border">
            {" "}
            {loading ? (
              <VStack gap="space-4" role="status" className="max-w-sm animate-pulse">
                <div className="h-6 bg-gray-200 rounded-full dark:bg-gray-700 w-48"></div>
                <div className="h-5 bg-gray-200 rounded-full dark:bg-gray-700 max-w-90"></div>
                <div className="h-5 bg-gray-200 rounded-full dark:bg-gray-700"></div>
                <div className="h-5 bg-gray-200 rounded-full dark:bg-gray-700 max-w-82.5"></div>
                <div className="h-5 bg-gray-200 rounded-full dark:bg-gray-700 max-w-75"></div>
                <div className="h-5 bg-gray-200 rounded-full dark:bg-gray-700 max-w-90"></div>
                <span className="sr-only">Loading...</span>
              </VStack>
            ) : needsGitHubLink ? (
              <BodyShort>Koble GitHub-kontoen din til navikt-organisasjonen for å aktivere Copilot.</BodyShort>
            ) : subscriptionError ? (
              <BodyShort>Noe gikk galt ved henting av abonnementsinformasjon. Prøv å laste siden på nytt.</BodyShort>
            ) : !eligibility ? (
              <BodyShort>
                Du har ikke tilgang til å få GitHub Copilot nå. GitHub Copilot er bare tilgjengelig for ansatte og
                konsulenter i Utvikling og Data.
              </BodyShort>
            ) : subscription ? (
              <VStack gap="space-4">
                <BodyShort>
                  <strong>Plan:</strong>{" "}
                  {subscription.plan_type
                    ? subscription.plan_type === "business"
                      ? "Bedriftsplan"
                      : "Individuell plan"
                    : "Ikke tilgjengelig"}
                </BodyShort>
                <BodyShort>
                  <strong>Status:</strong>{" "}
                  {subscription.pending_cancellation_date
                    ? "Kansellering pågår"
                    : subscription.updated_at
                      ? "Aktiv"
                      : "Inaktiv"}
                </BodyShort>
                <BodyShort>
                  <strong>Sist oppdatert:</strong>{" "}
                  {subscription.updated_at
                    ? new Date(subscription.updated_at).toLocaleDateString()
                    : "Ikke tilgjengelig"}
                </BodyShort>
                <BodyShort>
                  <strong>Siste aktivitet:</strong>{" "}
                  {subscription.last_activity_at
                    ? new Date(subscription.last_activity_at).toLocaleDateString()
                    : "Ikke tilgjengelig"}
                </BodyShort>
                <BodyShort>
                  <strong>Siste editor:</strong> {subscription.last_activity_editor || "Ikke tilgjengelig"}
                </BodyShort>
                <SubscriptionActionButton subscription={subscription} onClick={handleClick} isLoading={mutating} />
                {mutationError && (
                  <Alert variant="error" size="small">
                    {mutationError}
                  </Alert>
                )}
              </VStack>
            ) : (
              <VStack gap="space-4">
                <Heading size="small" level="3">
                  Du har ikke Copilot ennå
                </Heading>
                <BodyShort>
                  Du er kvalifisert for GitHub Copilot. Aktiver for å komme i gang – det er raskt gjort.
                </BodyShort>
                <SubscriptionActionButton subscription={subscription} onClick={handleClick} isLoading={mutating} />
                {mutationError && (
                  <Alert variant="error" size="small">
                    {mutationError}
                  </Alert>
                )}
              </VStack>
            )}
          </Box>
          <Box padding="space-8" borderRadius="8" className="border">
            <VStack gap="space-4">
              <Heading size="medium" level="3">
                Brukerinformasjon
              </Heading>
              <BodyShort>
                <strong>Navn:</strong> {user.firstName} {user.lastName}
              </BodyShort>
              <BodyShort>
                <strong>E-post:</strong> {user.email}
              </BodyShort>
              <div>
                <strong>GitHub:</strong>
                {githubUsername ? (
                  <span>
                    {" "}
                    <a href={`https://github.com/${githubUsername}`}>{githubUsername}</a>
                  </span>
                ) : loading ? (
                  <div role="status" className="inline-block animate-pulse" style={{ marginLeft: "8px" }}>
                    <div className="h-5 bg-gray-200 rounded-full dark:bg-gray-700 w-32"></div>
                  </div>
                ) : (
                  <span> Ikke koblet</span>
                )}
              </div>
              {showGroups && (
                <div>
                  <strong>Grupper:</strong>
                  <ul className="list-disc list-inside" style={{ marginLeft: "16px" }}>
                    {user.groups.map((group, index) => (
                      <li key={index}>{group}</li>
                    ))}
                  </ul>
                </div>
              )}
            </VStack>
          </Box>
          <Box padding="space-8" borderRadius="8" className="border">
            <VStack gap="space-4">
              <Heading size="medium" level="3">
                AI-forbruksgrense
              </Heading>
              {loading ? (
                <VStack gap="space-4" role="status">
                  <Skeleton variant="text" width="10rem" />
                  <Skeleton variant="text" width="14rem" />
                  <span className="sr-only">Laster budsjett...</span>
                </VStack>
              ) : budget ? (
                <>
                  {budget.consumedAmount !== null ? (
                    <>
                      <div style={{ width: "100%" }}>
                        <div
                          style={{
                            display: "flex",
                            justifyContent: "space-between",
                            marginBottom: "var(--a-spacing-1)",
                          }}
                        >
                          <BodyShort size="small" className="text-gray-600">
                            {budget.isOverride ? "Utvidet grense" : "Grense"} ·{" "}
                            {Math.round((budget.consumedAmount / budget.budgetAmount) * 100)}% brukt
                          </BodyShort>
                          <BodyShort size="small" className="text-gray-600">
                            {formatNumber(budget.consumedAmount)} / {formatNumber(budget.budgetAmount)} USD
                          </BodyShort>
                        </div>
                        <ProgressBar
                          value={budget.consumedAmount}
                          valueMax={budget.budgetAmount}
                          size="small"
                          aria-label={`${Math.round((budget.consumedAmount / budget.budgetAmount) * 100)}% av AI-kredittgrensen brukt`}
                        />
                      </div>
                      <BodyShort size="small">
                        Grensen er satt for å unngå uventet høyt forbruk – ikke et mål du skal nå. Ubrukt kapasitet
                        overføres ikke, og Nav betaler bare for faktisk forbruk.
                      </BodyShort>
                    </>
                  ) : (
                    <>
                      {budget.isOverride && (
                        <Tag variant="info" size="small">
                          Utvidet budsjett
                        </Tag>
                      )}
                      <BodyShort>
                        <strong>Månedlig grense:</strong> {formatNumber(budget.budgetAmount)} USD
                      </BodyShort>
                      <BodyShort size="small" className="text-gray-600">
                        {budget.isOverride
                          ? "Ingen forbruksdata for denne grensen."
                          : "GitHub rapporterer ikke individuelt forbruk for standardgrensen."}
                      </BodyShort>
                    </>
                  )}
                  {!budget.isOverride && (
                    <BodyShort size="small">
                      Standardgrense for alle Nav-utviklere. Bruk Copilot normalt — Nav betaler bare for faktisk
                      forbruk, ikke for ubrukt kapasitet.
                    </BodyShort>
                  )}
                </>
              ) : (
                <BodyShort>Ingen budsjettinformasjon tilgjengelig.</BodyShort>
              )}
            </VStack>
          </Box>
        </HGrid>

        {/* Usage row — only shown when we have data or are loading with a GitHub account */}
        {(loading || usageMetrics || githubUsername) && (
          <HGrid columns={{ xs: 1, md: 3 }} gap="space-8">
            {/* Card: Kodeforslag */}
            <Box padding="space-8" borderRadius="8" className="border">
              <VStack gap="space-4">
                <VStack gap="space-1">
                  <Heading size="medium" level="3">
                    Kodeforslag (30 dager)
                  </Heading>
                  <Detail className="text-gray-600">
                    Inline kodeforslag i IDE — Copilot foreslår kode mens du skriver
                  </Detail>
                </VStack>
                {loading ? (
                  <VStack gap="space-4" role="status">
                    <Skeleton variant="text" width="12rem" />
                    <Skeleton variant="rectangle" height="0.5rem" />
                    <Skeleton variant="text" width="10rem" />
                    <Skeleton variant="text" width="8rem" />
                    <span className="sr-only">Laster kodedata...</span>
                  </VStack>
                ) : usageMetrics && usageMetrics.total_generations > 0 ? (
                  <>
                    <div style={{ width: "100%" }}>
                      <div
                        style={{
                          display: "flex",
                          justifyContent: "space-between",
                          marginBottom: "var(--a-spacing-1)",
                        }}
                      >
                        <BodyShort size="small" className="text-gray-600">
                          Forslag akseptert
                        </BodyShort>
                        <BodyShort size="small" className="text-gray-600">
                          {usageMetrics.total_acceptances} / {usageMetrics.total_generations} (
                          {Math.round((usageMetrics.total_acceptances / usageMetrics.total_generations) * 100)}%)
                        </BodyShort>
                      </div>
                      <ProgressBar
                        value={usageMetrics.total_acceptances}
                        valueMax={usageMetrics.total_generations}
                        size="small"
                        aria-label={`${Math.round((usageMetrics.total_acceptances / usageMetrics.total_generations) * 100)}% av kodeforslag akseptert`}
                      />
                    </div>
                    <BodyShort>
                      <strong>Linjer akseptert:</strong> {formatNumber(usageMetrics.total_lines_accepted)}
                    </BodyShort>
                    {usageMetrics.days_used_code_review > 0 && (
                      <BodyShort>
                        <strong>Kode-gjennomgang:</strong> {usageMetrics.days_used_code_review} dager{" "}
                        <Detail as="span" className="text-gray-600">
                          (Copilot code review i PR)
                        </Detail>
                      </BodyShort>
                    )}
                  </>
                ) : (
                  <BodyShort>Ingen data for kodeforslag.</BodyShort>
                )}
              </VStack>
            </Box>

            {/* Card: Nav Pilot CLI */}
            <Box padding="space-8" borderRadius="8" className="border">
              <VStack gap="space-4">
                <VStack gap="space-1">
                  <Heading size="medium" level="3">
                    Copilot CLI (30 dager)
                  </Heading>
                  <Detail className="text-gray-600">
                    GitHub Copilot i terminal — chat, agenter og verktøykall via nav-pilot eller gh copilot
                  </Detail>
                </VStack>
                {loading ? (
                  <VStack gap="space-4" role="status">
                    <Skeleton variant="text" width="10rem" />
                    <Skeleton variant="text" width="12rem" />
                    <Skeleton variant="text" width="9rem" />
                    <span className="sr-only">Laster CLI-data...</span>
                  </VStack>
                ) : usageMetrics && usageMetrics.days_used_cli > 0 ? (
                  <>
                    <BodyShort>
                      <strong>Aktive dager:</strong> {usageMetrics.days_used_cli} av {usageMetrics.days_in_period}
                    </BodyShort>
                    <BodyShort>
                      <strong>Sesjoner:</strong> {formatNumber(usageMetrics.cli_sessions)}
                    </BodyShort>
                    <BodyShort>
                      <strong>Prompts:</strong> {formatNumber(usageMetrics.cli_prompts)}
                    </BodyShort>
                    <BodyShort>
                      <strong>Verktøykall:</strong> {formatNumber(usageMetrics.cli_total_requests)}{" "}
                      <Detail as="span" className="text-gray-600">
                        (MCP-kall, filoperasjoner, m.m.)
                      </Detail>
                    </BodyShort>
                    {usageMetrics.cli_prompt_tokens > 0 && (
                      <BodyShort size="small" className="text-gray-600">
                        {formatNumber(Math.round(usageMetrics.cli_prompt_tokens / 1_000_000))}M prompt-tokens ·{" "}
                        {formatNumber(Math.round(usageMetrics.cli_output_tokens / 1_000))}K output-tokens
                      </BodyShort>
                    )}
                  </>
                ) : (
                  <BodyShort>Ingen CLI-data tilgjengelig.</BodyShort>
                )}
              </VStack>
            </Box>

            {/* Card: Top models */}
            <Box padding="space-8" borderRadius="8" className="border">
              <VStack gap="space-4">
                <VStack gap="space-1">
                  <Heading size="medium" level="3">
                    Modeller brukt (30 dager)
                  </Heading>
                  <Detail className="text-gray-600">
                    AI-modeller rangert etter antall interaksjoner (chat + kodeforslag)
                  </Detail>
                </VStack>
                {loading ? (
                  <VStack gap="space-4" role="status">
                    <Skeleton variant="text" width="14rem" />
                    <Skeleton variant="rectangle" height="0.5rem" />
                    <Skeleton variant="text" width="12rem" />
                    <Skeleton variant="rectangle" height="0.5rem" />
                    <span className="sr-only">Laster modelldata...</span>
                  </VStack>
                ) : usageMetrics?.top_models?.length ? (
                  (() => {
                    const maxInteractions = usageMetrics.top_models[0].interactions;
                    return (
                      <VStack gap="space-4">
                        {usageMetrics.top_models.map((m) => (
                          <div key={m.model}>
                            <div
                              style={{
                                display: "flex",
                                justifyContent: "space-between",
                                marginBottom: "var(--a-spacing-1)",
                              }}
                            >
                              <BodyShort size="small">{m.model}</BodyShort>
                              <BodyShort size="small" className="text-gray-600">
                                {formatNumber(m.interactions)}
                              </BodyShort>
                            </div>
                            <ProgressBar
                              value={m.interactions}
                              valueMax={maxInteractions}
                              size="small"
                              aria-label={`${m.model}: ${m.interactions} interaksjoner`}
                            />
                          </div>
                        ))}
                      </VStack>
                    );
                  })()
                ) : (
                  <BodyShort>Ingen modelldata tilgjengelig.</BodyShort>
                )}
              </VStack>
            </Box>
          </HGrid>
        )}

        {/* Row 3: Daily credit usage chart */}
        {(loading || dailyCredits) && githubUsername && (
          <Box padding="space-8" borderRadius="8" className="border">
            <VStack gap="space-4">
              <Heading size="medium" level="3">
                AI-kredittforbruk per dag (30 dager)
              </Heading>
              {loading ? (
                <VStack gap="space-4" role="status">
                  <Skeleton variant="text" width="16rem" />
                  <Skeleton variant="rectangle" height="8rem" />
                  <span className="sr-only">Laster kredittdata...</span>
                </VStack>
              ) : dailyCredits ? (
                <DailyCreditsChart data={dailyCredits} />
              ) : null}
            </VStack>
          </Box>
        )}
      </VStack>
    </>
  );
};

export default SubscriptionDetails;
