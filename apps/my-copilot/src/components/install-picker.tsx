"use client";

import { useState, useSyncExternalStore } from "react";
import { BodyShort, CopyButton, ReadMore, Theme, ToggleGroup } from "@navikt/ds-react";
import { type InstallOs, installOsFromPlatform } from "@/lib/install-commands";

const STORAGE_KEY = "install-os";

const TEXT = {
  nb: {
    label: "Velg operativsystem",
    copy: "Kopier kommandoen",
    apt: "Bruk apt-arkivet (Debian/Ubuntu)",
    aptNote:
      "Apt-arkivet oppdaterer seg selv. Det bygges hver time fra nyeste versjon, så en helt fersk versjon kan ta opptil en time før den kan installeres. Feiler nedlastingen, stopper kommandoen og viser installasjonsskriptet i stedet.",
  },
  en: {
    label: "Choose your operating system",
    copy: "Copy the command",
    apt: "Use the apt archive (Debian/Ubuntu)",
    aptNote:
      "The archive updates itself and is rebuilt hourly from the latest release, so a release cut minutes ago can take up to an hour to become installable. If the download fails, the command stops and prints the install script instead.",
  },
};

interface InstallPickerProps {
  lang: "nb" | "en";
  /** One-line command for macOS (Homebrew). */
  mac: string;
  /** One-line command for Linux and WSL (install script). */
  linux: string;
  /** Multi-line apt-archive block, shown behind a disclosure in the Linux and Windows views. */
  apt: string;
  /** Short WSL note shown above the command in the Windows view. */
  windowsNote: string;
}

function readStoredOs(): InstallOs | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return v === "mac" || v === "linux" || v === "windows" ? v : null;
  } catch {
    return null;
  }
}

const noSubscribe = () => () => {};

function detectOs(): InstallOs {
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
  return readStoredOs() ?? installOsFromPlatform(nav.userAgentData?.platform || nav.userAgent);
}

function Command({ command, copyTitle }: { command: string; copyTitle: string }) {
  return (
    <div
      className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-2 rounded-lg pl-4 pr-2 py-1.5 text-left"
      style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)" }}
    >
      {/* minmax(0,1fr) keeps the long line from widening the page; it scrolls in the box. tabIndex lets keyboard users scroll it. */}
      <pre
        tabIndex={0}
        className="font-mono overflow-x-auto m-0 py-1.5"
        style={{ fontSize: "0.8rem", color: "rgba(255,255,255,0.75)", whiteSpace: "pre" }}
      >
        {command}
      </pre>
      <CopyButton copyText={command} title={copyTitle} size="small" />
    </div>
  );
}

export function InstallPicker({ lang, mac, linux, apt, windowsNote }: InstallPickerProps) {
  const t = TEXT[lang];
  // The server renders macOS; the visitor's last choice or OS is applied after hydration.
  const detected = useSyncExternalStore(noSubscribe, detectOs, () => "mac" as InstallOs);
  const [picked, setPicked] = useState<InstallOs | null>(null);
  const os = picked ?? detected;

  const choose = (value: string) => {
    const next = value as InstallOs;
    setPicked(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Private mode or blocked storage: the choice just isn't remembered.
    }
  };

  return (
    <Theme theme="dark" hasBackground={false}>
      <div className="w-full max-w-3xl min-w-0 flex flex-col gap-3">
        <ToggleGroup
          aria-label={t.label}
          value={os}
          onChange={choose}
          size="small"
          variant="neutral"
          className="self-center"
        >
          <ToggleGroup.Item value="mac" label="macOS" />
          <ToggleGroup.Item value="linux" label="Linux" />
          <ToggleGroup.Item value="windows" label="Windows" />
        </ToggleGroup>

        {os === "windows" && (
          <BodyShort size="small" className="text-center" style={{ color: "rgba(255,255,255,0.6)" }}>
            {windowsNote}
          </BodyShort>
        )}

        <Command command={os === "mac" ? mac : linux} copyTitle={t.copy} />

        {os !== "mac" && (
          <ReadMore header={t.apt} size="small">
            <div className="flex flex-col gap-2">
              <Command command={apt} copyTitle={t.copy} />
              <p style={{ fontSize: "0.75rem", lineHeight: 1.6, color: "rgba(255,255,255,0.6)" }}>{t.aptNote}</p>
            </div>
          </ReadMore>
        )}
      </div>
    </Theme>
  );
}
