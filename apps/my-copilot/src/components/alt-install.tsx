"use client";

import { useState } from "react";
import { CopyButton } from "@navikt/ds-react";

const APT_COMMAND = [
  "curl -fsSL https://navikt.github.io/apt/keyring/navikt-archive-keyring.gpg \\",
  "  | sudo tee /usr/share/keyrings/navikt-archive-keyring.gpg >/dev/null",
  'echo "deb [signed-by=/usr/share/keyrings/navikt-archive-keyring.gpg] https://navikt.github.io/apt stable main" \\',
  "  | sudo tee /etc/apt/sources.list.d/navikt.list",
  "sudo apt update && sudo apt install nav-pilot cplt",
].join("\n");

const INSTALL_SCRIPT_COMMAND =
  "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash";

const INSTALL_SAFE_COMMAND =
  "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh -o install.sh\ncat install.sh   # Inspect before running!\nbash install.sh";

function CodeRow({ command }: { command: string }) {
  return (
    <div
      className="rounded-lg overflow-hidden border border-gray-200 shadow-sm flex items-start justify-between"
      style={{ background: "#f1f5f9" }}
    >
      <pre
        className="font-mono flex-1 p-3 m-0 overflow-x-auto"
        style={{ fontSize: "0.75rem", color: "#334155", whiteSpace: "pre" }}
      >
        {command}
      </pre>
      <div className="shrink-0 pr-3 pt-3">
        <CopyButton copyText={command} size="xsmall" />
      </div>
    </div>
  );
}

export function AltInstall() {
  const [open, setOpen] = useState(false);

  return (
    <div style={{ marginTop: "0.5rem" }}>
      <button
        onClick={() => setOpen(!open)}
        style={{
          background: "none",
          border: "none",
          padding: 0,
          cursor: "pointer",
          fontSize: "0.8125rem",
          color: "#64748b",
          textDecoration: "underline",
          textDecorationStyle: "dotted",
          textUnderlineOffset: "2px",
        }}
        aria-expanded={open}
      >
        {open ? "Skjul" : "Ikke Homebrew? Linux / CI →"}
      </button>
      {open && (
        <div style={{ marginTop: "0.5rem" }}>
          <div style={{ fontSize: "0.75rem", color: "#334155", marginBottom: "0.375rem" }}>
            <strong>Debian og Ubuntu:</strong> installer fra apt-arkivet. Det er anbefalt vei på Linux, og du får både
            nav-pilot og cplt.
          </div>
          <CodeRow command={APT_COMMAND} />
          <div style={{ marginTop: "0.375rem", fontSize: "0.7rem", color: "#64748b", lineHeight: "1.5" }}>
            Arkivet oppdateres hver time fra den nyeste releasen, så en release som nettopp er kuttet kan bruke opptil
            en time på å bli installerbar. Det er et vanlig apt-arkiv som speiler releasene våre, ikke en distropakke
            med egen vedlikeholder.
          </div>

          <div style={{ fontSize: "0.75rem", color: "#334155", margin: "1rem 0 0.375rem" }}>
            <strong>Andre distroer og CI:</strong> bruk installasjonsskriptet.
          </div>
          <CodeRow command={INSTALL_SCRIPT_COMMAND} />
          <div
            style={{
              marginTop: "0.5rem",
              padding: "0.5rem 0.75rem",
              background: "#fefce8",
              border: "1px solid #fde047",
              borderRadius: "0.5rem",
              fontSize: "0.75rem",
              color: "#713f12",
              lineHeight: "1.5",
            }}
          >
            <strong>⚠ Sikkerhetsmerk:</strong> <code>curl | bash</code> kjører skriptet uten forhåndsverifikasjon. For
            CI eller sensitive miljøer: last ned og inspiser skriptet manuelt før kjøring.
            <div
              className="rounded-lg overflow-hidden border border-yellow-300 flex items-start justify-between mt-2"
              style={{ background: "#fefce8" }}
            >
              <pre
                className="font-mono flex-1 px-3 py-2 m-0"
                style={{ fontSize: "0.7rem", color: "#334155", whiteSpace: "pre" }}
              >
                {INSTALL_SAFE_COMMAND}
              </pre>
              <div className="shrink-0 pr-2 pt-2">
                <CopyButton copyText={INSTALL_SAFE_COMMAND} size="xsmall" />
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
