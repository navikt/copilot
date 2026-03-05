import SubscriptionDetails from "@/components/subscription";
import { getUser } from "@/lib/auth";
import { PageHero } from "@/components/page-hero";
import { Box } from "@navikt/ds-react";

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
          <SubscriptionDetails user={user!} />
        </Box>
      </div>
    </main>
  );
}
