import { render, screen } from "@testing-library/react";
import type { OverlayComponent } from "@/lib/public-videos";
import { VideoOverlayRenderer } from "./video-overlay-renderer";

const episode1: OverlayComponent[] = [
  { kind: "episode-number", anchor: "top-left", labels: ["01"] },
  { kind: "chip", anchor: "center-left", monospace: true, labels: ["Mål", "Fil", "Begrensning", "Output"] },
  { kind: "chip", anchor: "center-right", monospace: true, labels: ["cost-optimization.tsx"] },
  { kind: "counter", anchor: "bottom-left", labels: ["3 → 1"] },
  { kind: "badge", anchor: "bottom-right", labels: ["patch + 2 linjer"] },
  { kind: "badge", anchor: "top-right", labels: ["✓"] },
];

const episode2: OverlayComponent[] = [
  { kind: "episode-number", anchor: "top-left", labels: ["02"] },
  { kind: "chip", anchor: "center-left", monospace: true, labels: ["/resume", "/compact", "/clear"] },
  { kind: "chip", anchor: "top-right", labels: ["chronicle search"] },
  { kind: "chip", anchor: "center-right", monospace: true, labels: [".../adoption/summary"] },
  { kind: "rule-pill", anchor: "bottom-full", labels: ["nytt mål = ny tråd"] },
];

const episode3: OverlayComponent[] = [
  { kind: "episode-number", anchor: "top-left", labels: ["03"] },
  {
    kind: "ladder",
    anchor: "center-left",
    labels: ["ask/execute", "plan (Shift+Tab)", "/autopilot"],
    highlightIndex: 1,
  },
  {
    kind: "chip",
    anchor: "center-right",
    monospace: true,
    labels: ["@research-agent", "@nav-pilot", "@nav-pilot-opus"],
  },
  { kind: "chip", anchor: "bottom-left", monospace: true, labels: ["DATE/string-mismatch"] },
];

describe("VideoOverlayRenderer", () => {
  it("renders nothing without overlays", () => {
    const { container } = render(<VideoOverlayRenderer />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders every label in a multi-label chip (not just the first)", () => {
    render(<VideoOverlayRenderer overlays={episode1} />);
    for (const label of ["Mål", "Fil", "Begrensning", "Output"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("keeps full filename available via tooltip when chip text is truncated", () => {
    render(<VideoOverlayRenderer overlays={episode1} />);
    expect(screen.getByTitle("cost-optimization.tsx")).toBeInTheDocument();
    expect(screen.getByText(/cost-optimi/)).toBeInTheDocument();
  });

  it("splits a counter into before and after values", () => {
    render(<VideoOverlayRenderer overlays={episode1} />);
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("renders the episode number once as a pill", () => {
    render(<VideoOverlayRenderer overlays={episode1} />);
    expect(screen.getByText("01")).toBeInTheDocument();
  });

  it("renders all command chips for episode 2", () => {
    render(<VideoOverlayRenderer overlays={episode2} />);
    for (const label of ["/resume", "/compact", "/clear", "chronicle search", ".../adoption/summary"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("renders the rule-pill takeaway", () => {
    render(<VideoOverlayRenderer overlays={episode2} />);
    expect(screen.getByText("nytt mål = ny tråd")).toBeInTheDocument();
  });

  it("renders ladder steps including the highlighted one", () => {
    render(<VideoOverlayRenderer overlays={episode3} />);
    for (const label of ["ask/execute", "plan (Shift+Tab)", "/autopilot"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("renders all agent chips for episode 3", () => {
    render(<VideoOverlayRenderer overlays={episode3} />);
    for (const label of ["@research-agent", "@nav-pilot", "@nav-pilot-opus"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("does not crash on an unknown overlay kind", () => {
    const overlays: OverlayComponent[] = [{ kind: "mystery", anchor: "center", labels: ["surprise"] }];
    expect(() => render(<VideoOverlayRenderer overlays={overlays} />)).not.toThrow();
    expect(screen.getByText("surprise")).toBeInTheDocument();
  });
});
