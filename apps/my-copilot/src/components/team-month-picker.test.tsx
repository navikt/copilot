import { fireEvent, render, screen } from "@testing-library/react";
import TeamMonthPicker from "./team-month-picker";

it("provides independent month navigation without usage data", () => {
  render(<TeamMonthPicker month="2026-09" />);
  expect(screen.getByRole("form", { name: "Velg måned" })).toHaveAttribute("action", "/innsikt/team");
  expect(screen.getByLabelText("Måned")).toHaveValue("2026-09");
  expect(screen.getByRole("link", { name: "Forrige måned" })).toHaveAttribute("href", "/innsikt/team?month=2026-08");
});

it("updates the selected month after navigation, discarding an unsubmitted edit", () => {
  const { rerender } = render(<TeamMonthPicker month="2026-09" />);
  fireEvent.change(screen.getByLabelText("Måned"), { target: { value: "2026-07" } });
  rerender(<TeamMonthPicker month="2026-08" />);
  expect(screen.getByLabelText("Måned")).toHaveValue("2026-08");
});
