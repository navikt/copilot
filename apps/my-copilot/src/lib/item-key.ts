// A skill and an instruction can share a name, and so an id (#1036). The key
// that names one item is "<type>:<id>". A bare id, as older links carry, still
// resolves, to the first item with that id.
export function itemKey(item: { type: string; id: string }): string {
  return `${item.type}:${item.id}`;
}

export function findByItemKey<T extends { type: string; id: string }>(items: T[], key: string): T | undefined {
  const i = key.indexOf(":");
  const type = i < 0 ? undefined : key.slice(0, i);
  const id = i < 0 ? key : key.slice(i + 1);
  return items.find((item) => item.id === id && (type === undefined || item.type === type));
}
