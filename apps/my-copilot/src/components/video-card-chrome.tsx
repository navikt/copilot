"use client";

// Shared visual tokens and presentational chrome for the shorts feed cards.
//
// Pure presentation: every component here is a function of its props and emits no
// side-effects of its own. Interaction is delegated upward via callbacks so the
// controller stays the single owner of media/state logic. The `HeaderToken`
// family is the shared visual language (pill height, scrim, focus ring) reused by
// the episode/duration/link row *and* the completed overlay so the surface stays
// consistent.

import { ArrowCirclepathIcon, LinkIcon, PlayIcon } from "@navikt/aksel-icons";
import { Box, Heading } from "@navikt/ds-react";
import { useCopyToClipboard } from "./use-copy-to-clipboard";

export const HEADER_TOKEN_BASE =
  "inline-flex h-7 items-center rounded-[0.4rem] px-[var(--ax-space-8)] text-[11px] font-medium shadow-sm backdrop-blur-sm";
const HEADER_TOKEN_NEUTRAL = "bg-black/70 text-white";
const HEADER_TOKEN_ACTION =
  "transition-colors hover:bg-black/80 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70";

export function HeaderToken({
  children,
  className = "",
  style,
}: {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <span className={`${HEADER_TOKEN_BASE} ${className}`.trim()} style={style}>
      {children}
    </span>
  );
}

export function HeaderLinkToken({
  href,
  ariaLabel,
  children,
  className = "",
}: {
  href: string;
  ariaLabel: string;
  children: React.ReactNode;
  className?: string;
}) {
  const { copied, copy } = useCopyToClipboard(1200);

  return (
    <a
      href={href}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        void copy(href);
      }}
      onKeyDown={(event) => {
        event.stopPropagation();
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          void copy(href);
        }
      }}
      className={`${HEADER_TOKEN_BASE} ${HEADER_TOKEN_NEUTRAL} ${HEADER_TOKEN_ACTION} cursor-pointer gap-1 ${className}`.trim()}
      aria-label={copied ? "Delt" : ariaLabel}
      title={copied ? "Lenke kopiert" : "Del"}
    >
      {copied ? (
        <>
          <span aria-hidden>✓</span>
          <span>Kopiert</span>
        </>
      ) : (
        children
      )}
    </a>
  );
}

// The single central action. A real <button> so it is reachable by keyboard and
// touch; `pointer-events-auto` lets it stay clickable even though its overlay
// wrapper is click-through. Kept mounted while playing so pausing is always one
// interaction away.
export function PlaybackControls({
  ariaLabel,
  playing,
  showSkip,
  onToggle,
  onSeekBackward,
  onSeekForward,
  title,
}: {
  ariaLabel: string;
  playing: boolean;
  showSkip: boolean;
  onToggle: () => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  title: string;
}) {
  const sideButtonClass =
    "pointer-events-auto inline-flex h-10 w-10 items-center justify-center rounded-full bg-black/60 text-white transition-transform hover:scale-105 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70";

  return (
    <div className="pointer-events-auto flex items-center gap-2">
      {showSkip ? (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onSeekBackward();
          }}
          className={sideButtonClass}
          aria-label={`Spol 5 sek tilbake for ${title}`}
          title="Spol 5 sek tilbake"
        >
          <span aria-hidden className="relative inline-flex h-6 w-6 items-center justify-center">
            <ArrowCirclepathIcon aria-hidden fontSize="1.35rem" className="absolute inset-0 m-auto rotate-180" />
            <span className="absolute inset-0 flex items-center justify-center text-[8px] font-semibold leading-none">
              5
            </span>
          </span>
        </button>
      ) : (
        <div className="h-9 min-w-9" aria-hidden />
      )}

      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          onToggle();
        }}
        className="inline-flex h-16 w-16 items-center justify-center rounded-full bg-black/60 text-white transition-transform hover:scale-105 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70"
        aria-label={ariaLabel}
        title={playing ? "Pause" : "Spill av"}
      >
        {playing ? (
          <span aria-hidden className="inline-flex items-center gap-[0.15rem]">
            <span className="block h-4 w-0.5 rounded-full bg-current" />
            <span className="block h-4 w-0.5 rounded-full bg-current" />
          </span>
        ) : (
          <PlayIcon aria-hidden fontSize="1.5rem" />
        )}
      </button>

      {showSkip ? (
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            onSeekForward();
          }}
          className={sideButtonClass}
          aria-label={`Spol 5 sek frem for ${title}`}
          title="Spol 5 sek frem"
        >
          <span aria-hidden className="relative inline-flex h-6 w-6 items-center justify-center">
            <ArrowCirclepathIcon aria-hidden fontSize="1.35rem" className="absolute inset-0 m-auto" />
            <span className="absolute inset-0 flex items-center justify-center text-[8px] font-semibold leading-none">
              5
            </span>
          </span>
        </button>
      ) : (
        <div className="h-9 min-w-9" aria-hidden />
      )}
    </div>
  );
}

export function CornerFullscreenButton({ title, onClick }: { title: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={`${HEADER_TOKEN_BASE} ${HEADER_TOKEN_NEUTRAL} ${HEADER_TOKEN_ACTION} pointer-events-auto absolute bottom-3 right-3 z-20 !h-9 !w-9 !min-w-0 !px-0 justify-center`}
      aria-label={`Gå til fullskjerm for ${title}`}
      title="Fullskjerm"
    >
      <span aria-hidden className="text-sm leading-none">
        ⛶
      </span>
    </button>
  );
}

export function IdleCaption({ title }: { title: string }) {
  return (
    <Box
      as="div"
      paddingInline="space-12"
      paddingBlock="space-8"
      className="absolute inset-x-0 bottom-0 z-20 text-white"
    >
      <Heading size="xsmall" level="3" className="text-white">
        {title}
      </Heading>
    </Box>
  );
}

// Completed state: large replay button (center) + copy link button (below).
// Full-screen overlay with centered layout.
export function CompletedOverlay({
  title,
  shareHref,
  onReplay,
}: {
  title: string;
  shareHref: string;
  onReplay: () => void;
}) {
  const { copied, copy } = useCopyToClipboard(1400);

  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center pointer-events-auto text-white gap-8">
      {/* Replay button (same size as play/pause center control) */}
      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          onReplay();
        }}
        className="flex h-16 w-16 items-center justify-center rounded-full bg-white text-black shadow-lg transition-transform hover:scale-105 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70"
        aria-label={`Spill av på nytt: ${title}`}
        title="Spill av på nytt"
      >
        <ArrowCirclepathIcon aria-hidden fontSize="1.5rem" />
      </button>

      {/* Copy link button (below) */}
      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          void copy(shareHref);
        }}
        className={`${HEADER_TOKEN_BASE} bg-white text-black transition-colors hover:bg-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70`.trim()}
        aria-label={`Kopier lenke for ${title}`}
        title={copied ? "Lenke kopiert" : "Kopier lenke"}
      >
        {copied ? <span aria-hidden>✓</span> : <LinkIcon aria-hidden fontSize="0.9rem" />}
        <span>{copied ? "Kopiert" : "Kopier lenke"}</span>
      </button>
    </div>
  );
}
