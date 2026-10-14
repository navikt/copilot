import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { PrintHeader } from "./print-header";

afterEach(() => vi.useRealTimers());

it("stamps the export time in Oslo time, and again on beforeprint without waiting for React", () => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-14T08:00:00Z"));
  render(<PrintHeader title="Høsten 2026" lines={["142 svar"]} />);
  expect(screen.getByTestId("print-header")).toHaveTextContent("Eksportert 14. oktober 2026 kl. 10:00");

  vi.setSystemTime(new Date("2026-10-14T11:30:00Z"));
  window.dispatchEvent(new Event("beforeprint"));
  expect(screen.getByTestId("print-header")).toHaveTextContent("Eksportert 14. oktober 2026 kl. 13:30");
});
