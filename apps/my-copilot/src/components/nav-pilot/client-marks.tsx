interface MarkProps {
  size?: number;
  className?: string;
}

/**
 * Abstract, non-infringing marks used to differentiate the two supported
 * clients in the picker. Intentionally simple geometric glyphs rather than
 * the official brand logos.
 */
export function CopilotMark({ size = 24, className }: MarkProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      role="img"
      aria-label="GitHub Copilot"
      className={className}
    >
      <rect x="3" y="7" width="18" height="11" rx="5.5" fill="currentColor" opacity="0.12" />
      <rect x="3" y="7" width="18" height="11" rx="5.5" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="9" cy="12.5" r="1.6" fill="currentColor" />
      <circle cx="15" cy="12.5" r="1.6" fill="currentColor" />
      <path d="M12 7V4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="12" cy="3.5" r="1.2" fill="currentColor" />
    </svg>
  );
}

export function OpenCodeMark({ size = 24, className }: MarkProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      role="img"
      aria-label="OpenCode"
      className={className}
    >
      <rect x="3" y="4" width="18" height="16" rx="4" fill="currentColor" opacity="0.12" />
      <rect x="3" y="4" width="18" height="16" rx="4" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M8.5 10L6 12.5L8.5 15"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M15.5 10L18 12.5L15.5 15"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M13 9L11 16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
