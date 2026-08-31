/**
 * Kampanjepriser, vedlikeholdt for hånd.
 *
 * GitHub merker kampanjepriser med fotnoter i pristabellen sin, og
 * `scripts/sync-model-pricing.mjs` kaster fotnotene når den genererer
 * `model-pricing.ts` (den strippede `<sup>`-en er footnote-lenken). `ModelPrice`
 * har allerede et `note`-felt som kunne båret dette, men bare generatoren
 * skriver den fila, og pricing-sync-workflowen overskriver alt som legges inn
 * for hånd. Til parseren lærer å ta med fotnotene (#503) lever sluttdatoene her.
 *
 * Kilde: PRICING_SOURCE_URL, avlest 31. august 2026.
 */
interface ModelPromotion {
  /** Modellnavnene i MODEL_PRICING har tier-suffiks, så vi matcher på prefiks. */
  prefix: string;
  /** Sluttdato slik den vises til leseren. Dette er poenget med hele lista. */
  endsOn: string;
}

const MODEL_PROMOTIONS: ModelPromotion[] = [
  // "50% off standard rates" ut 3. september 2026. Fotnoten oppgir ikke
  // standardprisen; doblet kampanjepris gir $4.00 / $20.00.
  { prefix: "GPT-5.6 Sol", endsOn: "3. september 2026" },
  { prefix: "Gemini 3.6 Flash", endsOn: "31. desember 2026" },
  { prefix: "Gemini 3.7 Flash", endsOn: "31. desember 2026" },
];

export function promotionFor(model: string): ModelPromotion | undefined {
  return MODEL_PROMOTIONS.find((p) => model.startsWith(p.prefix));
}
