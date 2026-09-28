import { act, render, within } from "@testing-library/react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import NavPilotPage from "./page";

vi.mock("next/navigation", () => ({
  usePathname: () => "/nav-pilot",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

beforeEach(() => {
  // The page asks GitHub for the star count; the test must not.
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify({ stargazers_count: 1 })))
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
  Reflect.deleteProperty(navigator, "userAgentData");
});

describe("/nav-pilot", () => {
  it("gir hvert sitat en egen key (#1097)", async () => {
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    render(await NavPilotPage());
    const duplicate = error.mock.calls.find((args) => String(args[0]).includes("the same key"));
    error.mockRestore();
    expect(duplicate).toBeUndefined();
  });

  it("hydrerer uten avvik når installvelgeren finner et annet OS enn macOS (#1151)", async () => {
    // The server renders macOS. A Windows visitor with Linux stored must get
    // the same markup at hydration, and Linux only after it.
    Object.defineProperty(navigator, "userAgentData", { configurable: true, get: () => ({ platform: "Windows" }) });
    localStorage.setItem("install-os", "linux");
    const page = await NavPilotPage();
    const container = document.createElement("div");
    // Render the server markup without the browser APIs a server lacks, so a
    // component that reads them during render shows up as a mismatch below.
    vi.stubGlobal("navigator", undefined);
    vi.stubGlobal("localStorage", undefined);
    container.innerHTML = renderToString(page);
    vi.unstubAllGlobals();
    document.body.appendChild(container);

    const errors: unknown[] = [];
    const error = vi.spyOn(console, "error").mockImplementation((...args) => errors.push(args));
    await act(async () => {
      hydrateRoot(container, page, { onRecoverableError: (e) => errors.push(e) });
    });
    error.mockRestore();

    expect(errors).toEqual([]);
    expect(within(container).getByRole("radio", { name: "Linux" })).toHaveAttribute("aria-checked", "true");
    container.remove();
  });
});
