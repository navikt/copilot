import { fireEvent, render, screen, within } from "@testing-library/react";
import PriserPage from "./page";

vi.mock("next/navigation", () => ({
  usePathname: () => "/priser",
}));

function pristabell(): HTMLTableElement {
  return screen.getByRole("table") as HTMLTableElement;
}

function radFor(navn: string) {
  return within(pristabell()).getByText(navn).closest("tr")!;
}

describe("prissiden", () => {
  it("viser bare modeller Nav har aktivert uten status- eller nav-pilot-kolonner", () => {
    render(<PriserPage />);

    expect(radFor("GPT-6 Sol (Default, ≤ 272K)")).toBeInTheDocument();
    expect(radFor("Gemini 3.8 Flash (Default)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("GPT-5.4 (Default, ≤ 272K)")).toBeNull();
    expect(within(pristabell()).queryByRole("columnheader", { name: "Nav-status" })).toBeNull();
    expect(within(pristabell()).queryByRole("columnheader", { name: "nav-pilot" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "nav-pilots modellvalg" })).toBeNull();
    expect(screen.queryByText("Aktivert i Nav")).toBeNull();
    expect(screen.queryByText("Ikke aktivert i Nav")).toBeNull();
  });

  it("beholder filtrering og sortering for modellene som vises", () => {
    render(<PriserPage />);

    fireEvent.click(screen.getByText("Kategori: Alle"));
    const versatileToggle = screen.getByRole("checkbox", { name: "Versatile" });
    fireEvent.click(versatileToggle);

    expect(within(pristabell()).queryByText("Claude Sonnet 4")).toBeNull();
    expect(radFor("GPT-6 Sol (Default, ≤ 272K)")).toBeInTheDocument();

    fireEvent.click(versatileToggle);
    expect(radFor("Claude Sonnet 4")).toBeInTheDocument();

    const cacheWriteSort = screen.getByRole("button", { name: /Sorter etter Cache write/ });
    fireEvent.click(cacheWriteSort);
    expect(cacheWriteSort).toHaveAccessibleName("Sorter etter Cache write, stigende");
    expect(screen.getByRole("columnheader", { name: "Cache write" })).toHaveAttribute("aria-sort", "ascending");
    fireEvent.click(cacheWriteSort);
    expect(screen.getByRole("columnheader", { name: "Cache write" })).toHaveAttribute("aria-sort", "descending");
  });

  it("filtrerer på prisintervall og søk", () => {
    render(<PriserPage />);

    fireEvent.change(screen.getByRole("spinbutton", { name: "Cache write min" }), {
      target: { value: "0.2" },
    });

    expect(radFor("GPT-5.6 Luna (Default, ≤ 200K)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("GPT-6 Luna (Default, ≤ 272K)")).toBeNull();
    expect(within(pristabell()).queryByText("GPT-5.3-Codex (Default)")).toBeNull();

    fireEvent.change(screen.getByRole("spinbutton", { name: "Cache write min" }), {
      target: { value: "" },
    });
    fireEvent.change(screen.getByRole("searchbox", { name: "Filtrer modeller" }), {
      target: { value: "Gemini 3.8" },
    });
    expect(radFor("Gemini 3.8 Flash (Default)")).toBeInTheDocument();
    expect(within(pristabell()).queryByText("GPT-5.6 Luna (Default, ≤ 200K)")).toBeNull();

    fireEvent.change(screen.getByRole("searchbox", { name: "Filtrer modeller" }), {
      target: { value: "" },
    });
    expect(radFor("GPT-5.6 Luna (Default, ≤ 200K)")).toBeInTheDocument();
  });

  it("viser kampanjepris og cache write for synlige modeller", () => {
    render(<PriserPage />);

    const promotion = screen.getByText("Kampanjepris t.o.m. 31. desember 2026");
    expect(promotion.closest("td")).toHaveAccessibleName(
      "Gemini 3.8 Flash (Default) Kampanjepris t.o.m. 31. desember 2026"
    );
    expect(promotion).not.toHaveAttribute("title");
    expect(promotion).not.toHaveAttribute("aria-label");
    expect(promotion).not.toHaveAttribute("aria-describedby");
    expect(promotion).toHaveAccessibleDescription("");
    expect(document.body.textContent).not.toContain("promotional pricing");
    expect(within(radFor("GPT-6 Sol (Default, ≤ 272K)")).getByText("$2.50")).toBeInTheDocument();
    expect(within(radFor("GPT-6 Sol (Long context, 272K)")).getByText("$5.00")).toBeInTheDocument();
    expect(within(radFor("GPT-5.3-Codex (Default)")).getByText("—")).toBeInTheDocument();
  });
});
