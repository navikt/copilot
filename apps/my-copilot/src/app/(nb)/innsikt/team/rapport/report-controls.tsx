"use client";

import { useState } from "react";
import { BodyLong, Button, Textarea } from "@navikt/ds-react";
import { PrinterSmallIcon } from "@navikt/aksel-icons";

/** Editable on screen; in print only the text shows, and nothing when it is empty. */
export function ReportNote({ label, description }: { label: string; description: string }) {
  const [text, setText] = useState("");
  return (
    <>
      <Textarea
        className="print-hidden"
        label={label}
        description={description}
        value={text}
        onChange={(e) => setText(e.target.value)}
        minRows={3}
        resize="vertical"
      />
      {text.trim() && (
        <BodyLong className="print-only" style={{ whiteSpace: "pre-wrap" }}>
          <strong>{label}:</strong> {text}
        </BodyLong>
      )}
    </>
  );
}

export function PrintButton() {
  return (
    <Button
      className="print-hidden"
      type="button"
      variant="secondary"
      icon={<PrinterSmallIcon aria-hidden />}
      onClick={() => window.print()}
    >
      Skriv ut / lagre som PDF
    </Button>
  );
}
