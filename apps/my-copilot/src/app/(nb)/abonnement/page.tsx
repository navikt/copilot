import { Suspense } from "react";
import SubscriptionDetails from "@/components/subscription";
import UsageDistributionChart from "@/components/charts/UsageDistributionChart";
import { getUser, getUserToken } from "@/lib/auth";
import { getUsageDistribution, getUserDailyCredits } from "@/lib/cached-bigquery";
import { backendRequest } from "@/lib/backend-api";
import { PageHero } from "@/components/page-hero";
import { Box, Skeleton, VStack } from "@navikt/ds-react";
import { LinkableHeading } from "@/components/linkable-heading";

export default async function AbonnementPage() {
  const user = await getUser();

  return (
    <main>
      <PageHero title="Abonnement" description="Administrer ditt GitHub Copilot-abonnement." />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap="space-24">
            <SubscriptionDetails user={user!} />
            <Suspense
              fallback={
                <Box padding="space-16">
                  <VStack gap="space-8">
                    <Skeleton variant="text" width="12rem" />
                    <Skeleton variant="rectangle" height="10rem" />
                  </VStack>
                </Box>
              }
            >
              <UserDistribution email={user?.email} />
            </Suspense>
          </VStack>
        </Box>
      </div>
    </main>
  );
}

/** Server component: fetches org distribution + user's credits to show placement */
async function UserDistribution({ email }: { email?: string | null }) {
  const token = await getUserToken();
  if (!token || !email) return null;

  // Resolve GitHub username via SAML
  let ghUsername: string | null = null;
  if (process.env.NODE_ENV === "development" && process.env.DEV_GITHUB_LOGIN) {
    ghUsername = process.env.DEV_GITHUB_LOGIN;
  } else {
    try {
      const saml = await backendRequest<{ identity: string; username: string | null }>(
        `/api/v1/copilot/saml/${encodeURIComponent(email)}`,
        token
      );
      ghUsername = saml.username;
    } catch {
      return null;
    }
  }
  if (!ghUsername) return null;

  const [{ distribution }, { credits }] = await Promise.all([
    getUsageDistribution(token),
    getUserDailyCredits(ghUsername, token),
  ]);

  if (!distribution) return null;

  // Sum user's credits for the current month to determine their bucket
  const currentUserCredits = credits.reduce((sum, c) => sum + c.credits, 0);

  return (
    <VStack gap="space-8">
      <LinkableHeading size="small" level="3">
        Din plassering i Nav
      </LinkableHeading>
      <UsageDistributionChart distribution={distribution} currentUserCredits={currentUserCredits} />
    </VStack>
  );
}
