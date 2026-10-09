import { describe, expect, it } from "vitest";
import eksempel from "./__fixtures__/offentlig-statistikk.eksempel.json";
import { valider, type OffentligStatistikk } from "./offentlig-statistikk.schema";

const medMaaned = (felt: unknown) => ({ ...eksempel, maaneder: { "2000-01": felt } });
const v = { verdi: 500, kilde: "EKSEMPEL", dato: "2000-01-31" };

describe("offentlig statistikk", () => {
  it("godtar eksempelfilen, som er merket som eksempel", () => {
    const data: OffentligStatistikk = eksempel;
    expect(valider(data)).toEqual([]);
    expect(data.eksempel).toBe(true);
  });

  it.each([
    ["felt som ikke står på listen", medMaaned({ aktive_brukere_per_team: v })],
    ["gruppering under et felt", medMaaned({ aktive_brukere: { ...v, per_modell: { a: 1 } } })],
    ["liste som verdi", medMaaned({ aktive_brukere: [v] })],
    ["liste som måned", medMaaned([v])],
    ["verdi uten kilde", medMaaned({ aktive_brukere: { verdi: 500, dato: "2000-01-31" } })],
    ["verdi uten dato", medMaaned({ aktive_brukere: { verdi: 500, kilde: "EKSEMPEL" } })],
    ["verdi som ikke er avrundet", medMaaned({ aktive_brukere: { ...v, verdi: 517 } })],
    ["ukjent felt på toppnivå", { ...eksempel, team: {} }],
  ])("avviser %s", (_, data) => {
    expect(valider(data)).not.toEqual([]);
  });
});
