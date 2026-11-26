import type { CSSProperties, ReactNode } from "react";

/**
 * Horizontal scroll wrapper that keyboard users can reach and scroll.
 * A scrollable div is not focusable on its own; role, tabIndex and label make it a named stop.
 * `label` names the content, e.g. "Tabell: Kommandoer".
 */
export function ScrollRegion({
  label,
  className = "",
  style,
  children,
}: {
  label: string;
  className?: string;
  style?: CSSProperties;
  children: ReactNode;
}) {
  return (
    <div
      role="region"
      tabIndex={0}
      aria-label={label}
      className={`relative overflow-x-auto focus-visible:outline-3 focus-visible:outline-(--ax-border-focus) ${className}`.trim()}
      style={style}
    >
      {children}
    </div>
  );
}
