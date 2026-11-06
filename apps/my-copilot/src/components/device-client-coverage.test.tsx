vi.mock("react-chartjs-2", () => ({ Bar: () => null }));

import { render, screen, within } from "@testing-library/react";
import DeviceClientCoverage from "./device-client-coverage";
import type { NavPilotTelemetry } from "@/lib/types";

describe("DeviceClientCoverage", () => {
  it("keeps missing cohorts unknown and preserves reported zero", () => {
    render(
      <DeviceClientCoverage
        telemetry={
          {
            client_available_7d: { copilot: 210, opencode: null, pi: 0 },
            install_present_7d: [{ scope: "user", collection: "nav-pilot", devices: 0 }],
          } as NavPilotTelemetry
        }
      />
    );
    expect(within(screen.getByText("OpenCode").closest("tr")!).getByText("Ukjent")).toBeInTheDocument();
    expect(within(screen.getByText("Pi").closest("tr")!).getByText("0")).toBeInTheDocument();
    expect(within(screen.getByText("user / nav-pilot").closest("tr")!).getByText("0")).toBeInTheDocument();
    expect(within(screen.getByText("Pi").closest("tr")!).queryByText("Ukjent")).toBeNull();
    expect(screen.getByText(/ikke bruk/)).toBeInTheDocument();
    expect(screen.getByText(/kan ikke summeres/)).toBeInTheDocument();
  });

  it("renders a missing install snapshot as unknown", () => {
    render(<DeviceClientCoverage telemetry={{} as NavPilotTelemetry} />);
    expect(screen.getAllByText("Ukjent")).toHaveLength(3);
    expect(screen.queryByText("Klient med oppsett")).not.toBeInTheDocument();
    expect(screen.getByText(/Manglende rader betyr ikke null installasjoner/)).toBeInTheDocument();
  });
});
