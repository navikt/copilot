import { describe, expect, it } from "vitest";
import eksempel from "./__fixtures__/offentlig-statistikk.eksempel.json";
import { valider, type OffentligStatistikk } from "./offentlig-statistikk.schema";

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
    ["kostnad totalt", medMaaned({ kostnad_nok: v })],
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
    ["antall brukere i modellandel", med({ modellandeler: { ...andeler({ A: 100 }), brukere: { A: 25 } } })],
    [
      "for mange modeller",
      med({ modellandeler: andeler({ A: 15, B: 15, C: 15, D: 15, E: 15, F: 15, G: 5, andre: 5 }) }),
    ],
    ["modellandeler som liste", med({ modellandeler: { ...andeler({}), andeler: [100] } })],
  ])("avviser %s", (_, data) => {
    expect(valider(data)).not.toEqual([]);
  });
});
