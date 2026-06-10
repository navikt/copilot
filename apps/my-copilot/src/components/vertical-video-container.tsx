import type { ReactNode } from "react";

interface VerticalVideoContainerProps {
  children: ReactNode;
}

/**
 * Cinematic two-column layout for the video detail page.
 *
 * Mobile (< 768px): stacked — video on top, metadata below.
 * Desktop (≥ 768px): side-by-side — narrow video column on left, metadata panel on right.
 *
 * The container itself is fully dark (black) so the entire page feels like a video
 * experience, not a document. Column backgrounds are set by the children.
 */
export function VerticalVideoContainer({ children }: VerticalVideoContainerProps) {
  return <section className="flex w-full flex-col bg-black md:min-h-0 md:flex-1 md:flex-row">{children}</section>;
}
