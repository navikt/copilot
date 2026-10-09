import { describe, expect, it } from "vitest";
import eksempel from "./__fixtures__/offentlig-statistikk.eksempel.json";
import { modellfamilie, valider, type OffentligStatistikk } from "./offentlig-statistikk.schema";

describe("modellfamilie", () => {
  it.each([
    ["Claude Opus 5.5", "claude_opus"],
    ["claude-opus-6", "claude_opus"],
    ["Claude Sonnet 4.6", "claude_sonnet"],
    ["Claude Haiku 5.5", "claude_haiku"],
    ["Claude Fable 5.1", "claude_fable"],
    ["GPT-6 Sol", "gpt"],
    ["GPT-5.3-Codex", "gpt"],
    ["GPT-5.4 mini", "gpt_mini"],
    ["GPT-5.4 nano", "gpt_mini"],
    ["GPT-6 Luna", "gpt_mini"],
    ["Gemini 3.8 Flash", "gemini"],
    ["Kimi K3", "andre"],
  ])("%s → %s", (modell, familie) => {
    expect(modellfamilie(modell)).toBe(familie);
  });
});

const mnd = eksempel.maaneder["2000-01"];
const medMaaned = (felt: unknown) => ({ ...eksempel, maaneder: { "2000-01": felt } });
const med = (ekstra: object) => medMaaned({ ...mnd, ...ekstra });
const v = { verdi: 500, kilde: "EKSEMPEL", dato: "2000-01-31" };
const andeler = (a: object) => ({ andeler: a, kilde: "EKSEMPEL", dato: "2000-01-31" });

describe("offentlig statistikk", () => {
  it("godtar eksempelfilen, som er merket som eksempel", () => {
    const data: OffentligStatistikk = eksempel;
    expect(valider(data)).toEqual([]);
    expect(data.eksempel).toBe(true);
  });

  it("godtar sammenslåtte band og avrundingsavvik", () => {
    expect(valider(med({ bruksband: andeler({ lett: 60, middels_tung: 41 }) }))).toEqual([]);
  });

  it.each([
    ["felt som ikke står på listen", medMaaned({ aktive_brukere_per_team: v })],
    ["maksverdi", medMaaned({ credits_per_bruker_maks: v })],
    ["persentil over p90", medMaaned({ credits_per_bruker_p95: v })],
    ["kostnad i kroner", medMaaned({ kostnad_per_bruker_median_nok: v })],
    ["gruppering under et felt", medMaaned({ aktive_brukere: { ...v, per_modell: { a: 1 } } })],
    ["liste som verdi", medMaaned({ aktive_brukere: [v] })],
    ["liste som måned", medMaaned([v])],
    ["verdi uten kilde", medMaaned({ aktive_brukere: { verdi: 500, dato: "2000-01-31" } })],
    ["verdi uten dato", medMaaned({ aktive_brukere: { verdi: 500, kilde: "EKSEMPEL" } })],
    ["verdi som ikke er avrundet", medMaaned({ aktive_brukere: { ...v, verdi: 517 } })],
    ["ukjent felt på toppnivå", { ...eksempel, team: {} }],
    ["band med ukjent navn", med({ bruksband: andeler({ lett: 50, p99: 50 }) })],
    ["band som ikke summerer til 100", med({ bruksband: andeler({ lett: 50, tung: 20 }) })],
    ["andel som ikke er hel prosent", med({ bruksband: andeler({ lett: 50.5, tung: 49.5 }) })],
    ["antall brukere i modellandel", med({ modellandeler: { ...andeler({ gpt: 100 }), brukere: { gpt: 25 } } })],
    ["modellversjon i stedet for familie", med({ modellandeler: andeler({ "Claude Opus 5.5": 60, andre: 40 }) })],
    ["modellandeler som ikke summerer til 100", med({ modellandeler: andeler({ gpt: 60, andre: 20 }) })],
    ["modellandeler som liste", med({ modellandeler: { ...andeler({}), andeler: [100] } })],
  ])("avviser %s", (_, data) => {
    expect(valider(data)).not.toEqual([]);
  });
});
