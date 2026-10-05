export type Sak = { id: number; tittel: string; prioritet: "normal" | "hoy" };

const saker: Sak[] = [{ id: 101, tittel: "Eksisterende sak", prioritet: "normal" }];

export function listSaker(): Sak[] {
  return [...saker];
}
