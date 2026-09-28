import { render } from "@testing-library/react";
import NavPilotPage from "./page";

vi.mock("next/navigation", () => ({
  usePathname: () => "/nav-pilot",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

describe("/nav-pilot", () => {
  it("gir hvert sitat en egen key (#1097)", async () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    render(await NavPilotPage());
    const duplicate = error.mock.calls.find((args) => String(args[0]).includes("the same key"));
    error.mockRestore();
    expect(duplicate).toBeUndefined();
  });
});
