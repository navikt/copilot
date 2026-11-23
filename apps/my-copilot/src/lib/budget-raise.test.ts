import { describe, expect, it } from "vitest";
import { errorMessage, hasOwnBudget, type Budget } from "./budget-raise";

const b = (over: Partial<Budget>): Budget => ({
  budgetAmount: 150,
  consumedAmount: 10,
  isOverride: false,
  defaultBudget: 150,
  raises: [],
  ...over,
});
const raise = (status: string) => ({
  id: "1",
  createdAt: "",
  amountUsd: 150,
  category: "koding",
  teamId: "annet",
  expectedValue: "laere",
  freeText: null,
  status,
});

describe("hasOwnBudget", () => {
  it("is false on the default budget", () => expect(hasOwnBudget(b({}))).toBe(false));
  it("is true for an override with none of our raises", () => expect(hasOwnBudget(b({ isOverride: true }))).toBe(true));
  it("is false when the override comes from our own raise", () =>
    expect(hasOwnBudget(b({ isOverride: true, raises: [raise("applied")] }))).toBe(false));
  it("is unknown when the raises are missing", () =>
    expect(hasOwnBudget(b({ isOverride: true, raises: undefined }))).toBeNull());
});

describe("errorMessage", () => {
  it("has a message for each backend error", () => {
    for (const code of ["not_open", "cap_reached", "own_budget", "profile_required", "budget_writes_unavailable"]) {
      expect(errorMessage(code)).not.toBe(errorMessage(undefined));
    }
  });
});
