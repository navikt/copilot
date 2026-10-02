type Sak = { id: number; status: "apen" | "lukket" };

const saker: Sak[] = [
  { id: 101, status: "apen" },
  { id: 102, status: "lukket" },
];

export function listSaker(status: string): Sak[] {
  if (!["alle", "apen", "lukket"].includes(status)) {
    throw new Error(`Ukjent status: ${status}`);
  }
  return status === "alle" ? saker : saker.filter((sak) => sak.status === status);
}
