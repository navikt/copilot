"use client";

import { Chat2Icon, FileTextIcon, LightningIcon, PuzzlePieceIcon, RobotIcon } from "@navikt/aksel-icons";
import { Tag } from "@navikt/ds-react";
import type { CustomizationType } from "@/lib/customization-types";
import { TYPE_LABELS } from "@/lib/customization-types";

export const TYPE_ORDER: CustomizationType[] = ["agent", "instruction", "skill", "mcp"];

export function typeClassName(type: CustomizationType): string {
  return `verktoy-type-${type}`;
}

export function TypeIcon({ type, size = "1rem" }: { type: CustomizationType; size?: string }) {
  switch (type) {
    case "agent":
      return <RobotIcon fontSize={size} aria-hidden />;
    case "instruction":
      return <FileTextIcon fontSize={size} aria-hidden />;
    case "skill":
      return <LightningIcon fontSize={size} aria-hidden />;
    case "mcp":
      return <PuzzlePieceIcon fontSize={size} aria-hidden />;
    case "prompt":
      return <Chat2Icon fontSize={size} aria-hidden />;
  }
}

export function TypeIconBadge({ type, size = "md" }: { type: CustomizationType; size?: "sm" | "md" | "lg" }) {
  const sizeClass = size === "sm" ? "verktoy-type-icon--sm" : size === "lg" ? "verktoy-type-icon--lg" : "";
  const iconSize = size === "sm" ? "1rem" : size === "lg" ? "1.375rem" : "1.125rem";

  return (
    <span className={`verktoy-type-icon ${sizeClass} ${typeClassName(type)}`.trim()} aria-hidden="true">
      <TypeIcon type={type} size={iconSize} />
    </span>
  );
}

export function TypeBadge({ type, size = "xsmall" }: { type: CustomizationType; size?: "xsmall" | "small" }) {
  return (
    <Tag size={size} variant="neutral" className={`verktoy-type-badge ${typeClassName(type)}`}>
      {TYPE_LABELS[type]}
    </Tag>
  );
}
