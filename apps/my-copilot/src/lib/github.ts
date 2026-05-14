// TODO: This file is kept temporarily for getPremiumRequestUsage which is not yet implemented in the backend API.
// Once the backend implements the premium request usage endpoint, this file can be removed entirely.

import { Octokit } from "@octokit/rest";
import { createAppAuth } from "@octokit/auth-app";
import type { PremiumRequestUsage } from "@/lib/types";

const requiredEnvVars = ["GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY", "GITHUB_APP_INSTALLATION_ID"];

for (const varName of requiredEnvVars) {
  if (!process.env[varName]) {
    throw new Error(`Environment variable ${varName} is required but not set.`);
  }
}

const octokit = new Octokit({
  authStrategy: createAppAuth,
  auth: {
    appId: process.env.GITHUB_APP_ID,
    privateKey: process.env.GITHUB_APP_PRIVATE_KEY,
    installationId: process.env.GITHUB_APP_INSTALLATION_ID,
  },
});

export async function getPremiumRequestUsage(
  org: string,
  year?: number,
  month?: number
): Promise<{ usage: PremiumRequestUsage | null; error: string | null }> {
  try {
    const params: { org: string; year?: number; month?: number } = { org };
    if (year) params.year = year;
    if (month) params.month = month;

    const { data } = await octokit.request("GET /organizations/{org}/settings/billing/premium_request/usage", {
      org,
      ...(year && { year }),
      ...(month && { month }),
      headers: {
        "X-GitHub-Api-Version": "2022-11-28",
      },
    });

    // GitHub API returns snake_case, map to our camelCase types
    const raw = data as Record<string, unknown>;
    const rawItems = (raw.usage_items ?? raw.usageItems ?? []) as Record<string, unknown>[];
    const usage: PremiumRequestUsage = {
      timePeriod: (raw.time_period ?? raw.timePeriod) as PremiumRequestUsage["timePeriod"],
      organization: (raw.organization ?? org) as string,
      usageItems: rawItems.map((item) => ({
        product: item.product as string,
        sku: item.sku as string,
        model: item.model as string,
        unitType: (item.unit_type ?? item.unitType) as string,
        pricePerUnit: (item.price_per_unit ?? item.pricePerUnit) as number,
        grossQuantity: (item.gross_quantity ?? item.grossQuantity) as number,
        grossAmount: (item.gross_amount ?? item.grossAmount) as number,
        discountQuantity: (item.discount_quantity ?? item.discountQuantity) as number,
        discountAmount: (item.discount_amount ?? item.discountAmount) as number,
        netQuantity: (item.net_quantity ?? item.netQuantity) as number,
        netAmount: (item.net_amount ?? item.netAmount) as number,
      })),
    };

    return { usage, error: null };
  } catch (error) {
    return { usage: null, error: error instanceof Error ? error.message : String(error) };
  }
}
