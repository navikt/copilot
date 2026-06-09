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
  return <section className="flex flex-col md:flex-row w-full bg-black min-h-[calc(100vh-52px)]">{children}</section>;
}
