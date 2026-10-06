import { describe, it, expect } from "vitest";
import { MODEL_PRICING } from "./model-pricing";
import { getCurrentOsloDate } from "./news";

describe("promotionEndsOn", () => {
  // Utledet fra dataene, ikke fra modellnavn: GitHub fjerner modeller jevnlig,
  // og en test som navngir rader brekker den daglige prissynken hver gang.
  const throughDate = (note?: string) => {
    const match = note?.match(/through (\w+ \d{1,2}, \d{4})/);
    if (!match) return undefined;
    const date = new Date(`${match[1]} UTC`);
    return date.toISOString().slice(0, 10);
  };

  it("har rader å sjekke", () => {
    expect(MODEL_PRICING.length).toBeGreaterThan(0);
  });

  it("merker nøyaktig radene med kampanjefotnote hos GitHub, med sluttdatoen fra fotnoten", () => {
    for (const m of MODEL_PRICING) {
      expect(m.promotionEndsOn, m.model).toBe(throughDate(m.note));
    }
  });

  it("leser sluttdatoen fra fotnoteteksten", () => {
    expect(throughDate("promotional pricing through December 31, 2026.")).toBe("2026-12-31");
    expect(throughDate("no end date here")).toBeUndefined();
  });
});

describe("utløpte kampanjer", () => {
  // Denne vakta er med vilje offline. Prisene er tredjeparts data, og
  // `.github/workflows/pricing-sync.yaml` argumenterer i fila for hvorfor et
  // nettverkskall ikke skal gate en PR: da blir GitHubs prisendring en rød
  // build på main for den som tilfeldigvis pusher etterpå. Det holder. Men
  // spørsmålet «lover vår egen committede fil en kampanje som har gått ut?»
  // trenger ikke nett, og det er nøyaktig feilen 4. september.
  it("lover ingen kampanje som allerede har gått ut", () => {
    // Datoene er rene datoer, ikke tidspunkt, så de sammenlignes som ISO-
    // strenger. YYYY-MM-DD sorterer kronologisk, og da slipper vi Date-
    // aritmetikk som kan bomme med et døgn over et døgnskille.
    //
    // Klokka er Oslo-tid, ikke UTC og ikke kjørerens sone. CI kjører i UTC,
    // og en dato-only sammenligning mot feil sone flytter grensa med noen
    // timer. `getCurrentOsloDate` finnes allerede i news.ts til nettopp dette.
    const today = getCurrentOsloDate();
    expect(today, "Oslo-datoen lot seg ikke lese. Vakta under er da avskrudd, ikke grønn.").toMatch(
      /^\d{4}-\d{2}-\d{2}$/
    );

    // GitHub skriver «through September 3, 2026», altså til og med. Kampanjen
    // gjelder fortsatt den 3., og er utløpt fra den 4. Derfor streng `<`.
    const expired = MODEL_PRICING.filter((m) => m.promotionEndsOn && m.promotionEndsOn < today).map(
      (m) => `${m.model} (t.o.m. ${m.promotionEndsOn})`
    );

    expect(
      expired,
      [
        `model-pricing.ts oppgir en kampanjepris som gikk ut før ${today}:`,
        ...expired.map((row) => `  ${row}`),
        "",
        "Dette er ikke en ødelagt test, og den skal ikke slettes. Prisen i fila",
        "er utdatert, og prissiden er offentlig, så den viser nå en pris Nav ikke",
        "får. Kjør `mise run pricing:sync` og commit den regenererte fila, eller",
        "merge PR-en fra .github/workflows/pricing-sync.yaml hvis den står åpen.",
        "",
        "Rødt her er meningen: det er det eneste som kobler en utløpt kampanje til",
        "noen som ser den. Skru den av, og fila blir stille feil i stedet.",
      ].join("\n")
    ).toEqual([]);
  });
});
