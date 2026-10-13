import { render, screen } from "@testing-library/react";
import { CHART_ANNOTATIONS } from "../../reisen/milestones";
import { Events, PeriodSelect } from "./controls";

describe("hendelser på /innsikt/trender", () => {
  it("viser én nummerert liste med lenke til kilden for hver hendelse", () => {
    render(<Events />);
    screen.getByRole("button", { name: /Hendelser/ }).click();
    const lists = screen.getAllByRole("list", { name: "Hendelser" });
    expect(lists).toHaveLength(1);
    expect(lists[0].querySelectorAll("li")).toHaveLength(CHART_ANNOTATIONS.length);
    expect(screen.getAllByRole("link")).toHaveLength(CHART_ANNOTATIONS.filter((a) => a.url).length);
  });

  it("sender valget for hendelser i URL-en sammen med perioden", () => {
    const { rerender } = render(<PeriodSelect value="12" all={false} />);
    const box = screen.getByRole("checkbox", { name: "Vis alle hendelser i diagrammene" }) as HTMLInputElement;
    expect(box.name).toBe("hendelser");
    expect(box.value).toBe("alle");
    expect(box.checked).toBe(false);
    expect(box.form?.getAttribute("method")).toBe("get");
    rerender(<PeriodSelect value="12" all />);
    expect((screen.getByRole("checkbox") as HTMLInputElement).checked).toBe(true);
  });
});
