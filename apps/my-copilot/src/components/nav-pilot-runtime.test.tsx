vi.mock("react-chartjs-2", () => ({ Bar: () => null }));
vi.mock("@/lib/auth", () => ({ getUser: vi.fn(), getUserToken: async () => "t" }));
vi.mock("@/lib/cached-bigquery", () => ({ getNavPilotTelemetry: vi.fn() }));

import { render, screen } from "@testing-library/react";
import { getNavPilotTelemetry } from "@/lib/cached-bigquery";
import type { NavPilotTelemetry } from "@/lib/types";
import { RuntimeTelemetry } from "./nav-pilot-runtime";

// Go marshals an empty slice as null.
const nullSlices = {
  devices_7d: 10,
  top_skills_7d: null,
  top_agents_7d: null,
  top_tools_7d: null,
  command_reach_7d: null,
} as unknown as NavPilotTelemetry;

it("renders when every list is null, and skips the empty sections", async () => {
  vi.mocked(getNavPilotTelemetry).mockResolvedValue({ telemetry: nullSlices, error: null });
  render(await RuntimeTelemetry({ token: "t" }));
  expect(screen.getByText("Rekkevidde etter vindu")).toBeInTheDocument();
  expect(screen.queryByText("Skills og subagenter")).not.toBeInTheDocument();
});

it("says «blant de viste» without a share, and the fold lists every row", async () => {
  const skills = Array.from({ length: 12 }, (_, i) => ({ skill: `s${i}`, source: "navikt", loads: 100 - i }));
  vi.mocked(getNavPilotTelemetry).mockResolvedValue({
    telemetry: { ...nullSlices, top_skills_7d: skills, agent_skill_loads_7d: 1, user_skill_loads_7d: 1 },
    error: null,
  });
  render(await RuntimeTelemetry({ token: "t" }));
  expect(screen.getByText(/s0 har flest innlastinger blant de viste radene: 100\./)).toBeInTheDocument();
  expect(screen.getByText("Vis alle 12 rader, diagrammet viser de 10 øverste")).toBeInTheDocument();
});
