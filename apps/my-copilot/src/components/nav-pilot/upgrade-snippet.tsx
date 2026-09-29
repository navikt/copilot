"use client";

import { useState, useSyncExternalStore } from "react";
import { ReadMore, ToggleGroup, VStack } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import { detectOs, noSubscribe } from "@/components/install-picker";
import { UPGRADE_COMMANDS, type UpgradeMethod } from "@/lib/install-commands";

// "Oppgrader først" at the top of the nav-pilot guides. Opens on the visitor's
// OS: Homebrew on a Mac, apt elsewhere (the recommended Linux and WSL route).
export function UpgradeSnippet() {
  const os = useSyncExternalStore(noSubscribe, detectOs, () => "mac" as const);
  const [picked, setPicked] = useState<UpgradeMethod | null>(null);
  const method = picked ?? (os === "mac" ? "brew" : "apt");

  return (
    <ReadMore header="Oppgrader først: nav-pilot, cplt og klienten">
      <VStack gap="space-12">
        <ToggleGroup
          aria-label="Hvordan installerte du nav-pilot?"
          value={method}
          onChange={(v) => setPicked(v as UpgradeMethod)}
          size="small"
        >
          <ToggleGroup.Item value="brew" label="Homebrew" />
          <ToggleGroup.Item value="apt" label="apt" />
          <ToggleGroup.Item value="script" label="Skript" />
        </ToggleGroup>
        <CodeBlock compact>{UPGRADE_COMMANDS[method]}</CodeBlock>
      </VStack>
    </ReadMore>
  );
}
