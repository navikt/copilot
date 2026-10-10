import { renderToString } from "react-dom/server";
import { Greeting, getGreeting } from "./greeting";

describe("Greeting", () => {
  it("renders the same neutral text on the server at any hour", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 0, 1, 23));
    expect(renderToString(<Greeting />)).toContain("Hei!");
    vi.useRealTimers();
  });

  it("picks a greeting by hour", () => {
    expect(getGreeting(3)).toBe("God natt!");
    expect(getGreeting(8)).toBe("God morgen!");
    expect(getGreeting(12)).toBe("Hei!");
    expect(getGreeting(20)).toBe("God kveld!");
  });
});
