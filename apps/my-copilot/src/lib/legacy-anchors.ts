// Anchors that have moved: old "path#anchor" to new "path#anchor".
//
// A published anchor must keep working. When one moves or goes away, add an
// entry here. The key's path is the page the browser lands on after any
// redirect in next.config.ts. The test does not follow a redirect() inside a
// page. Write ø and å as-is, not percent-encoded. HashAnchorScroll sends the
// reader on to the target. src/link-inventory.test.ts checks that every
// target exists and that no key is a real id on its page.
export const LEGACY_ANCHORS: Record<string, string> = {};
