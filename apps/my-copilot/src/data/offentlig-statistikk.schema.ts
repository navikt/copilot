// Skjema for offentlig statistikk (#1511). Bare totaler for hele Nav, per måned.
// Plan: docs/offentlig-statistikk-plan.md. Generatoren avrunder; testen sjekker.

/** Godkjente felt og avrundingen hvert felt må ha. Nye felt krever godkjenning fra produkteier. */
const FELT = {
  lisenser: 50,
  aktive_brukere: 50,
  ai_credits: 10_000,
  kostnad_nok: 10_000,
  katalog_elementer: 1,
  modeller_malt: 1,
} as const;

type Felt = keyof typeof FELT;

type Verdi = {
  verdi: number;
  /** Tabell, endepunkt eller fil verdien kommer fra. */
  kilde: string;
  /** Dato (YYYY-MM-DD) dataene gjelder til og med. */
  dato: string;
};

export type OffentligStatistikk = {
  /** true for eksempeldata. Siden skal aldri vise en fil med eksempel: true. */
  eksempel: boolean;
  generert: string;
  /** Nøkkel: måned (YYYY-MM). */
  maaneder: Record<string, Partial<Record<Felt, Verdi>>>;
};

const DATO = /^\d{4}-\d{2}-\d{2}$/;
const MAANED = /^\d{4}-\d{2}$/;
const isObj = (x: unknown): x is Record<string, unknown> => typeof x === "object" && x !== null && !Array.isArray(x);

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
      if (!Object.hasOwn(FELT, navn)) {
        feil.push(`${sti}: feltet står ikke på listen`);
        continue;
      }
      if (!isObj(v)) {
        feil.push(`${sti}: må være { verdi, kilde, dato }`);
        continue;
      }
      for (const k of Object.keys(v))
        if (!["verdi", "kilde", "dato"].includes(k)) feil.push(`${sti}: ukjent felt ${k}`);
      const avrunding = FELT[navn as Felt];
      if (typeof v.verdi !== "number" || !Number.isFinite(v.verdi)) feil.push(`${sti}: verdi må være et tall`);
      else if (v.verdi % avrunding !== 0) feil.push(`${sti}: ikke avrundet til ${avrunding}`);
      if (typeof v.kilde !== "string" || !v.kilde) feil.push(`${sti}: mangler kilde`);
      if (typeof v.dato !== "string" || !DATO.test(v.dato)) feil.push(`${sti}: mangler dato`);
    }
  }
  return feil;
}
