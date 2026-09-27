// Anchors that have moved: old "path#anchor" to new "path#anchor".
//
// A published anchor must keep working. When one moves or goes away, add an
// entry here; src/link-inventory.test.ts checks every target exists.
// HashAnchorScroll does not read this map yet (#1045), so today an entry only
// records the move.
export const LEGACY_ANCHORS: Record<string, string> = {};
