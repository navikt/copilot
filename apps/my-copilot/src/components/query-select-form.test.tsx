import { fireEvent, render, screen } from "@testing-library/react";
import { QuerySelectForm } from "./query-select-form";

const props = {
  action: "/x",
  formLabel: "Velg",
  label: "Felt",
  name: "felt",
  options: [
    { value: "a", label: "A" },
    { value: "b", label: "B" },
  ],
};

it("renders a GET form that submits the select under the given name", () => {
  render(<QuerySelectForm {...props} value="b" />);
  const form = screen.getByRole("form", { name: "Velg" });
  expect(form).toHaveAttribute("action", "/x");
  expect(form).toHaveAttribute("method", "get");
  const select = screen.getByLabelText("Felt") as HTMLSelectElement;
  expect(select.name).toBe("felt");
  expect(select).toHaveValue("b");
  expect(screen.getByRole("button", { name: "Vis" })).toHaveAttribute("type", "submit");
});

it("resets an unsubmitted edit when the value changes", () => {
  const { rerender } = render(<QuerySelectForm {...props} value="a" />);
  fireEvent.change(screen.getByLabelText("Felt"), { target: { value: "b" } });
  rerender(<QuerySelectForm {...props} value="b" />);
  rerender(<QuerySelectForm {...props} value="a" />);
  expect(screen.getByLabelText("Felt")).toHaveValue("a");
});
