"use client";

import { useState } from "react";
import { CopyButton } from "@navikt/ds-react";

const INSTALL_SCRIPT_COMMAND =
  "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash";

const INSTALL_SAFE_COMMAND =
  "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh -o install.sh\ncat install.sh   # Inspect before running!\nbash install.sh";

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
          <div
            className="rounded-lg overflow-hidden border border-gray-200 shadow-sm flex items-center justify-between"
            style={{ background: "#f1f5f9" }}
          >
            <code className="font-mono whitespace-nowrap flex-1 p-3" style={{ fontSize: "0.75rem", color: "#334155" }}>
              {INSTALL_SCRIPT_COMMAND}
            </code>
            <div className="shrink-0 pr-3">
              <CopyButton copyText={INSTALL_SCRIPT_COMMAND} size="xsmall" />
            </div>
          </div>
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
