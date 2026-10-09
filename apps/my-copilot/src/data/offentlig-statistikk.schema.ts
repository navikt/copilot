// Skjema for offentlig statistikk (#1511). Bare tall for hele Nav, per måned.
// Plan: docs/offentlig-statistikk-plan.md. Generatoren avrunder og håndhever terskler; testen sjekker.

/** Godkjente enkelttall og avrundingen hvert felt må ha. Aldri maks eller persentiler over p90. */
const FELT = {
  lisenser: 50,
  aktive_brukere: 50,
  credits_per_bruker_median: 10,
  credits_per_bruker_snitt: 10,
  kostnad_per_bruker_median_nok: 10,
  kostnad_per_bruker_snitt_nok: 10,
  katalog_elementer: 1,
  modeller_malt: 1,
} as const;

/** Faste bruksband. Et band under 20 brukere slås sammen med naboen. */
const BAND = ["lett", "middels", "tung", "lett_middels", "middels_tung"];

/**
 * Modellnavn → familie. Første treff vinner, så smale mønstre står først.
 * Navn normaliseres til små bokstaver med bindestrek, så «Claude Opus 6» og «claude-opus-6» treffer likt.
 * Familier under 20 brukere i måneden legges i «andre» av generatoren.
 */
const FAMILIER: [RegExp, string][] = [
  [/^claude-opus/, "claude_opus"],
  [/^claude-sonnet/, "claude_sonnet"],
  [/^claude-haiku/, "claude_haiku"],
  [/^claude-fable/, "claude_fable"],
  [/^gpt-.*(mini|nano|luna)/, "gpt_mini"],
  [/^gpt-/, "gpt"],
  [/^gemini/, "gemini"],
];
const FAMILIENAVN = [...new Set(FAMILIER.map(([, f]) => f)), "andre"];

/** Familien for et modellnavn fra fakturadataene. Ukjente navn gir «andre» og logges. */
export function modellfamilie(modell: string): string {
  const navn = modell
    .toLowerCase()
    .trim()
    .replace(/[\s_]+/g, "-");
  const treff = FAMILIER.find(([m]) => m.test(navn));
  if (!treff) console.warn(`ukjent modell, legges i «andre»: ${modell}`);
  return treff?.[1] ?? "andre";
}

type Felt = keyof typeof FELT;

type Kilde = {
  /** Tabell, endepunkt eller fil verdien kommer fra. */
  kilde: string;
  /** Dato (YYYY-MM-DD) dataene gjelder til og med. */
  dato: string;
};

type Verdi = Kilde & { verdi: number };
/** Andeler i hele prosent som summerer til omtrent 100. */
type Andeler = Kilde & { andeler: Record<string, number> };

export type OffentligStatistikk = {
  /** true for eksempeldata. Siden skal aldri vise en fil med eksempel: true. */
  eksempel: boolean;
  generert: string;
  /** Nøkkel: måned (YYYY-MM). */
  maaneder: Record<string, Partial<Record<Felt, Verdi>> & { bruksband?: Andeler; modellandeler?: Andeler }>;
};

const DATO = /^\d{4}-\d{2}-\d{2}$/;
const MAANED = /^\d{4}-\d{2}$/;
const isObj = (x: unknown): x is Record<string, unknown> => typeof x === "object" && x !== null && !Array.isArray(x);

function sjekkKilde(v: Record<string, unknown>, sti: string, tillatt: string[], feil: string[]) {
  for (const k of Object.keys(v)) if (!tillatt.includes(k)) feil.push(`${sti}: ukjent felt ${k}`);
  if (typeof v.kilde !== "string" || !v.kilde) feil.push(`${sti}: mangler kilde`);
  if (typeof v.dato !== "string" || !DATO.test(v.dato)) feil.push(`${sti}: mangler dato`);
}

function sjekkAndeler(v: Record<string, unknown>, sti: string, feil: string[], navnOk: (n: string) => boolean) {
  sjekkKilde(v, sti, ["andeler", "kilde", "dato"], feil);
  if (!isObj(v.andeler)) {
    feil.push(`${sti}: andeler må være et objekt`);
    return;
  }
  let sum = 0;
  for (const [navn, p] of Object.entries(v.andeler)) {
    if (!navnOk(navn)) feil.push(`${sti}.${navn}: navnet er ikke tillatt`);
    if (!Number.isInteger(p) || (p as number) < 0) feil.push(`${sti}.${navn}: må være hel prosent`);
    else sum += p as number;
  }
  // Avrunding til hele prosent kan gi 98–102.
  if (Math.abs(sum - 100) > 2) feil.push(`${sti}: andelene summerer til ${sum}, ikke omtrent 100`);
}

/** Returnerer brudd på sikkerhetsreglene. Tom liste betyr godkjent. */
export function valider(data: unknown): string[] {
  const feil: string[] = [];
  if (!isObj(data)) return ["roten må være et objekt"];
  for (const k of Object.keys(data))
    if (!["eksempel", "generert", "maaneder"].includes(k)) feil.push(`ukjent felt: ${k}`);
  if (typeof data.eksempel !== "boolean") feil.push("eksempel mangler");
  if (typeof data.generert !== "string" || !DATO.test(data.generert))
    feil.push("generert mangler eller har feil format");
  if (!isObj(data.maaneder)) return [...feil, "maaneder må være et objekt"];
  for (const [mnd, felt] of Object.entries(data.maaneder)) {
    if (!MAANED.test(mnd)) feil.push(`ugyldig måned: ${mnd}`);
    if (!isObj(felt)) {
      feil.push(`${mnd}: må være et objekt`);
      continue;
    }
    for (const [navn, v] of Object.entries(felt)) {
      const sti = `${mnd}.${navn}`;
      if (!isObj(v)) {
        feil.push(`${sti}: må være et objekt`);
        continue;
      }
      if (navn === "bruksband") {
        sjekkAndeler(v, sti, feil, (n) => BAND.includes(n));
      } else if (navn === "modellandeler") {
        // Alle familier hver måned. Terskelen på 20 brukere håndheves i generatoren; antallet publiseres ikke.
        sjekkAndeler(v, sti, feil, (n) => FAMILIENAVN.includes(n));
      } else if (Object.hasOwn(FELT, navn)) {
        sjekkKilde(v, sti, ["verdi", "kilde", "dato"], feil);
        const avrunding = FELT[navn as Felt];
        if (typeof v.verdi !== "number" || !Number.isFinite(v.verdi)) feil.push(`${sti}: verdi må være et tall`);
        else if (v.verdi % avrunding !== 0) feil.push(`${sti}: ikke avrundet til ${avrunding}`);
      } else {
        feil.push(`${sti}: feltet står ikke på listen`);
      }
    }
  }
  return feil;
}
