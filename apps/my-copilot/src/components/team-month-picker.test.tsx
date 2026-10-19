import { fireEvent, render, screen } from "@testing-library/react";
import TeamMonthPicker from "./team-month-picker";

it("provides independent month navigation without usage data", () => {
  render(<TeamMonthPicker month="2026-09" />);
  expect(screen.getByRole("form", { name: "Velg måned" })).toHaveAttribute("action", "/innsikt/team");
  expect(screen.getByLabelText("Måned")).toHaveValue("2026-09");
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
  expect(screen.getByRole("option", { name: "september 2026" })).toBeInTheDocument();
});

it("allows selecting the current month but not future months", () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-02T00:00:00Z"));
  render(<TeamMonthPicker month="2026-10" />);
  expect(screen.getByRole("option", { name: "oktober 2026" })).toBeInTheDocument();
  expect(screen.queryByRole("option", { name: "november 2026" })).not.toBeInTheDocument();
  vi.useRealTimers();
});

it("updates the selected month after navigation, discarding an unsubmitted edit", () => {
  const { rerender } = render(<TeamMonthPicker month="2026-09" />);
  fireEvent.change(screen.getByLabelText("Måned"), { target: { value: "2026-07" } });
  rerender(<TeamMonthPicker month="2026-08" />);
  expect(screen.getByLabelText("Måned")).toHaveValue("2026-08");
});

it.each([
  ["?mine=1", "1"],
  ["", null],
])("keeps mine=1 on submit only when it is in the URL (%s)", (search, expected) => {
  window.history.pushState({}, "", `/innsikt/team${search}`);
  render(<TeamMonthPicker month="2026-09" />);
  const form = screen.getByRole("form", { name: "Velg måned" }) as HTMLFormElement;
  fireEvent.submit(form);
  expect(new FormData(form).get("mine")).toBe(expected);
  window.history.pushState({}, "", "/");
});
