export type Sak = { id: number; tittel: string; status: "apen" | "lukket" };
export type InternSak = Sak & { internNotat: string };

const saker: InternSak[] = [
  { id: 101, tittel: "Søknad om dagpenger", status: "apen", internNotat: "Venter på arbeidsgiver" },
  { id: 102, tittel: "Klage på vedtak", status: "lukket", internNotat: "Omgjort etter klage" },
];

export function listSaker(): Sak[] {
  return saker.map(({ internNotat: _, ...sak }) => sak);
}

export function listInterneSaker(): InternSak[] {
  return [...saker];
}

export function finnInternSak(id: number): InternSak | undefined {
  return saker.find((sak) => sak.id === id);
}

export function leggTilNotat(id: number, tekst: string): InternSak | undefined {
  const sak = finnInternSak(id);
  if (sak) sak.internNotat = `${sak.internNotat}\n${tekst}`;
  return sak;
}
