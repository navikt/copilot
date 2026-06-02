"use client";

import React, { useState, useEffect } from "react";
import { Button, Alert, Box, VStack, HGrid, Heading, BodyShort, Link, Tag, Skeleton } from "@navikt/ds-react";
import { User } from "@/lib/auth";
import { formatNumber } from "@/lib/format";

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
}> = ({ subscription, onClick }) => {
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

  if (subscription?.pending_cancellation_date) {
    buttonColor = "danger";
    buttonText = "Kanseller Copilot...";
  } else if (subscription?.updated_at) {
    buttonColor = "danger";
    buttonText = "Deaktiver Copilot";
  } else {
    buttonColor = "primary";
    buttonText = "Aktiver Copilot";
  }

  return (
    <Button variant={buttonColor} onClick={onClick}>
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

  const fetchSubscription = async () => {
    setLoading(true);
    try {
      const response = await fetch("/api/copilot");
      const data = await response.json();

      if (data.error) {
        setSubscriptionError(data.error);
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
    if (eligibility) {
      if (subscription && subscription.updated_at && !subscription.pending_cancellation_date) {
        try {
          const response = await updateCopilotSubscription("deactivate");
          const data = await response.json();
          if (data.error) {
            console.error("Error deactivating subscription:", data.error);
          }
        } catch (error) {
          console.error("Error:", error);
        } finally {
          fetchSubscription();
        }
      } else {
        try {
          const response = await updateCopilotSubscription("activate");
          const data = await response.json();
          if (data.error) {
            console.error("Error activating subscription:", data.error);
          }
        } catch (error) {
          console.error("Error:", error);
        } finally {
          fetchSubscription();
        }
      }
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
              Koble GitHub-konto via SSO →
            </Link>
          </Alert>
        </Box>
      )}

      <HGrid columns={{ xs: 1, md: 2, lg: 3 }} gap="space-8">
        {" "}
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
                    : "Individuell Plan"
                  : "Ikke tilgjengelig"}
              </BodyShort>
              <BodyShort>
                <strong>Status:</strong>{" "}
                {subscription.pending_cancellation_date
                  ? "Kansellering Pågår"
                  : subscription.updated_at
                    ? "Aktiv"
                    : "Inaktiv"}
              </BodyShort>
              <BodyShort>
                <strong>Sist Oppdatert:</strong>{" "}
                {subscription.updated_at ? new Date(subscription.updated_at).toLocaleDateString() : "Ikke tilgjengelig"}
              </BodyShort>
              <BodyShort>
                <strong>Siste Aktivitet:</strong>{" "}
                {subscription.last_activity_at
                  ? new Date(subscription.last_activity_at).toLocaleDateString()
                  : "Ikke tilgjengelig"}
              </BodyShort>
              <BodyShort>
                <strong>Siste Editor:</strong> {subscription.last_activity_editor || "Ikke tilgjengelig"}
              </BodyShort>
              <SubscriptionActionButton subscription={subscription} onClick={handleClick} />
            </VStack>
          ) : (
            <VStack gap="space-4">
              <Heading size="small" level="3">
                Du har ikke Copilot ennå
              </Heading>
              <BodyShort>
                Du er kvalifisert for GitHub Copilot. Aktiver for å komme i gang – det tar bare et øyeblikk.
              </BodyShort>
              <SubscriptionActionButton subscription={subscription} onClick={handleClick} />
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
              AI-kredittbudsjett
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
                          {budget.isOverride ? "Utvidet budsjett" : "Budsjett"} ·{" "}
                          {Math.round((budget.consumedAmount / budget.budgetAmount) * 100)}% brukt
                        </BodyShort>
                        <BodyShort size="small" className="text-gray-600">
                          {formatNumber(budget.consumedAmount)} / {formatNumber(budget.budgetAmount)} USD
                        </BodyShort>
                      </div>
                      <div
                        style={{
                          height: "10px",
                          width: "100%",
                          borderRadius: "var(--a-border-radius-full)",
                          backgroundColor: "var(--a-surface-neutral)",
                        }}
                        role="progressbar"
                        aria-label={`${Math.round((budget.consumedAmount / budget.budgetAmount) * 100)}% av budsjettet brukt`}
                        aria-valuenow={budget.consumedAmount}
                        aria-valuemin={0}
                        aria-valuemax={budget.budgetAmount}
                      >
                        <div
                          style={{
                            height: "100%",
                            borderRadius: "var(--a-border-radius-full)",
                            width: `${Math.min(100, Math.round((budget.consumedAmount / budget.budgetAmount) * 100))}%`,
                            backgroundColor:
                              budget.consumedAmount / budget.budgetAmount > 0.9
                                ? "var(--a-icon-danger)"
                                : budget.consumedAmount / budget.budgetAmount > 0.7
                                  ? "var(--a-icon-warning)"
                                  : "var(--a-icon-success)",
                          }}
                        />
                      </div>
                    </div>
                    <BodyShort>
                      <strong>Gjenstående:</strong>{" "}
                      {formatNumber(Math.max(0, budget.budgetAmount - budget.consumedAmount))} USD
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
                      <strong>Månedlig budsjett:</strong> {formatNumber(budget.budgetAmount)} USD
                    </BodyShort>
                  </>
                )}
                {!budget.isOverride && (
                  <BodyShort size="small">Dette er standardbudsjettet for alle Nav-utviklere.</BodyShort>
                )}
              </>
            ) : (
              <BodyShort>Budsjettinformasjon er ikke tilgjengelig.</BodyShort>
            )}
          </VStack>
        </Box>
      </HGrid>
    </>
  );
};

export default SubscriptionDetails;
