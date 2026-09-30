import type { Metadata } from "next";
import { getAllCustomizations } from "@/lib/customizations";
import { loadGoldenSummary } from "@/lib/golden-summary-file";
import { NAV_PILOT_DEFAULT_MODEL, NAV_PILOT_MODEL_CHOICES } from "@/lib/model-policy";
import { ModellerContent } from "./modeller-content";

export const metadata: Metadata = {
  title: "Modellvalg — Hvilke modeller agentene bruker og hvorfor",
  description:
    "Hvilken modell hver agent og prompt i Nav bruker, hvorfor, hva målingene viser og hva modellene koster.",
};

/** Who uses each primary model: the frontmatter pins from the manifest, plus @nav-pilot's pack default. */
function usersOf(model: string): string[] {
  const pinned = getAllCustomizations()
    .filter((item) => (item.type === "agent" || item.type === "prompt") && item.model?.[0] === model)
    .map((item) => (item.type === "agent" ? `@${item.name.replace(/-agent$/, "")}` : item.name));
  return model === NAV_PILOT_DEFAULT_MODEL ? ["@nav-pilot", ...pinned] : pinned;
}

// Read per request: the build only compiles, and the image copies the summary in (see Dockerfile).
export default function ModellerPage() {
  const users = Object.fromEntries(NAV_PILOT_MODEL_CHOICES.map((c) => [c.primary, usersOf(c.primary)]));
  return <ModellerContent summary={loadGoldenSummary()} users={users} />;
}
