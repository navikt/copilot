import { readFileSync } from "node:fs";
import { join } from "node:path";
import { render, screen } from "@testing-library/react";
import { Table, TableBody, TableHeaderCell, TableRow } from "./aksel-table";

// Aksel's larger text size broke short labels onto two lines («Registrer MCP- / server»).
describe("labels stay on one line", () => {
  it("globals.css keeps button, tag and tab labels unwrapped from 480 px", () => {
    const css = readFileSync(join(__dirname, "../app/globals.css"), "utf8");
    expect(css).toMatch(
      /@media \(min-width: 480px\) \{\s*\.aksel-button,\s*\.aksel-tag,\s*\.aksel-tabs__tab \{\s*white-space: nowrap;/
    );
  });

  it("the fleet status tags opt out of wrapping", () => {
    const src = readFileSync(join(__dirname, "../app/(nb)/fleet/fleet-ui.tsx"), "utf8");
    expect(src).toMatch(/const StatusTag[^]*?<Tag [^>]*className="whitespace-nowrap"/);
  });

  it("a short table header does not wrap, a long one may", () => {
    render(
      <Table>
        <TableBody>
          <TableRow>
            <TableHeaderCell>Publisert versjon</TableHeaderCell>
            <TableHeaderCell>Jeg er bekymret for at KI kan svekke min egen dype forståelse</TableHeaderCell>
          </TableRow>
        </TableBody>
      </Table>
    );
    expect(screen.getByText("Publisert versjon").closest("th")).toHaveClass("md:whitespace-nowrap");
    expect(screen.getByText(/Jeg er bekymret/).closest("th")).not.toHaveClass("md:whitespace-nowrap");
  });
});
