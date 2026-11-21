import fs from "node:fs";
import path from "node:path";
import { render, screen } from "@testing-library/react";
import allowlist from "../../../../../../../mcp-registry/allowlist.json";
import FleetGatewayPage from "./page";
import RetningslinjerPage from "./retningslinjer/page";
import { PILOT_SERVERS } from "./servers";

vi.mock("next/navigation", () => ({ usePathname: () => "/verktoy/fleet-gateway" }));

const prodRoutes = fs.readFileSync(
  path.resolve(__dirname, "../../../../../../../fleet-gateway/pilot-routes.prod.yaml"),
  "utf8"
);

describe("Servere i piloten", () => {
  it.each(PILOT_SERVERS.map((s) => [s.id, s]))("%s is active in the registry and routed in prod", (id, server) => {
    const entry = allowlist.servers.find((e) => e.name === id);
    expect(entry?.status).toBe("active");
    expect(entry?.remotes?.[0]?.url).toContain(server.route);
    expect(prodRoutes).toContain(`route: ${server.route}\n`);
  });

  it("renders a card with command and collapsed setup per server", () => {
    const { container } = render(<FleetGatewayPage />);
    for (const s of PILOT_SERVERS) {
      expect(screen.getByRole("heading", { level: 3, name: s.title })).toBeInTheDocument();
      expect(container.textContent).toContain(`nav-pilot mcp enable ${s.id.split("/")[1]}`);
      expect(container.textContent).toContain(s.codeLabel);
    }
    expect(screen.getAllByRole("button", { name: "Se hvordan den er satt opp" })).toHaveLength(PILOT_SERVERS.length);
  });
});

describe("Retningslinjer", () => {
  const ids = [
    "token",
    "lesing-forst",
    "verktoylister",
    "grenser",
    "hoder",
    "logging",
    "beskrivelser",
    "metrikker",
    "ros",
    "eksempler",
    "kilder",
  ];

  it("keeps every section anchor, and every Kort fortalt link hits one", () => {
    const { container } = render(<RetningslinjerPage />);
    for (const id of ids) expect(container.querySelector(`#${id}`)).not.toBeNull();
    const summary = container.querySelector("#kort-fortalt")!.closest("div")!;
    const links = [...summary.querySelectorAll("ul a")].map((a) => a.getAttribute("href")!);
    expect(links.length).toBeGreaterThanOrEqual(6);
    expect(links.length).toBeLessThanOrEqual(8);
    for (const href of links) expect(container.querySelector(href)).not.toBeNull();
  });

  it("renders the collapsed details, so the facts stay on the page", () => {
    const { container } = render(<RetningslinjerPage />);
    expect(screen.getAllByRole("button", { name: "Detaljer" })).toHaveLength(9);
    for (const fact of ["AZURE_APP_PRE_AUTHORIZED_APPS", "introspection_failed", "-32020", "user_sse_cap", "MCP08"])
      expect(container.textContent).toContain(fact);
  });
});
