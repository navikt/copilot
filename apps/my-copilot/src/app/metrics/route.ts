import { getCopilotBilling } from "@/lib/github";
import { getEngagementMetrics } from "@/lib/metrics";

export async function GET() {
  const { billing, error } = await getCopilotBilling("navikt");

  if (error) {
    return new Response(`Error fetching billing data: ${error}`, { status: 500 });
  }

  const billingMetrics = `# HELP copilot_seats_total Total number of Copilot seats
# TYPE copilot_seats_total gauge
copilot_seats_total ${billing.seat_breakdown.total || 0}

# HELP copilot_seats_added_this_cycle Number of Copilot seats added this cycle
# TYPE copilot_seats_added_this_cycle gauge
copilot_seats_added_this_cycle ${billing.seat_breakdown.added_this_cycle || 0}

# HELP copilot_seats_pending_invitation Number of Copilot seats pending invitation
# TYPE copilot_seats_pending_invitation gauge
copilot_seats_pending_invitation ${billing.seat_breakdown.pending_invitation || 0}

# HELP copilot_seats_pending_cancellation Number of Copilot seats pending cancellation
# TYPE copilot_seats_pending_cancellation gauge
copilot_seats_pending_cancellation ${billing.seat_breakdown.pending_cancellation || 0}

# HELP copilot_seats_active_this_cycle Number of Copilot seats active this cycle
# TYPE copilot_seats_active_this_cycle gauge
copilot_seats_active_this_cycle ${billing.seat_breakdown.active_this_cycle || 0}

# HELP copilot_seats_inactive_this_cycle Number of Copilot seats inactive this cycle
# TYPE copilot_seats_inactive_this_cycle gauge
copilot_seats_inactive_this_cycle ${billing.seat_breakdown.inactive_this_cycle || 0}`;

  const engagement = getEngagementMetrics();
  const allMetrics = [billingMetrics, engagement].filter(Boolean).join("\n\n");

  return new Response(allMetrics + "\n", {
    headers: { "Content-Type": "text/plain" },
  });
}
