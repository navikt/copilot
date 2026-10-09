import type { Ref } from "react";

// Decorative band in the flow of a centred hero, below the headline, so it never sits behind text.
// Hidden from assistive tech and from the pointer, so touch scrolls the page as before.
export function HeroBand({ ref }: { ref: Ref<HTMLDivElement> }) {
  return (
    <div
      ref={ref}
      aria-hidden="true"
      className="relative w-full max-w-4xl mx-auto h-52 md:h-64 pointer-events-none [&>canvas]:absolute [&>canvas]:inset-0 [&>canvas]:w-full [&>canvas]:h-full [mask-image:linear-gradient(90deg,transparent,black_12%,black_88%,transparent)]"
    />
  );
}
