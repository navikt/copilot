import { slugify } from "@/components/linkable-heading";

export interface SourceHeading {
  tag: string;
  // The explicit id="…", or for a LinkableHeading whose children are one
  // plain string, the slug the component renders. Otherwise undefined.
  id?: string;
  // The heading as it reads, or undefined when it holds an expression.
  text?: string;
}

// Headings in a page's source: <LinkableHeading>, <Heading> and <h1>–<h6>.
// The link guard and the search index both read them, so they read them the
// same way (#1098).
// ponytail: regex over the source, not a render. An attribute holding a ">"
// cuts the match short; a self-closing heading is skipped.
export function sourceHeadings(src: string): SourceHeading[] {
  return [...src.matchAll(/<(LinkableHeading|Heading|h[1-6])\b((?:[^>"]|"[^"]*")*)(?<!\/)>([\s\S]*?)<\/\1>/g)].map(
    (m) => {
      const [, tag, attrs, children] = m;
      const spaced = children.replace(/\{" "\}/g, " ");
      // Our own source, shown as text: split on tags, then drop any stray < or >.
      const text = spaced.includes("{")
        ? undefined
        : spaced
            .split(/<[^>]*>/)
            .join("")
            .replace(/[<>]/g, "")
            .replace(/\s+/g, " ")
            .trim() || undefined;
      // The component slugs a single string child, and only without an id prop.
      const slug = tag === "LinkableHeading" && text && !/\bid=/.test(attrs) && !/[<{]/.test(children);
      const id = attrs.match(/\bid="([^"]+)"/)?.[1] ?? (slug ? slugify(text) : undefined);
      return { tag, id, text };
    }
  );
}
