"use client";

import { captureException } from "@nais/apm";
import { useEffect } from "react";

export default function GlobalError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    captureException(error, { context: { digest: error.digest } });
  }, [error]);

  return (
    <html lang="nb">
      <body>
        <div style={{ padding: "4rem 2rem", textAlign: "center", maxWidth: "600px", margin: "0 auto" }}>
          <h1>Noe gikk galt</h1>
          <p>En uventet feil oppstod. Prøv igjen, eller kontakt oss hvis problemet vedvarer.</p>
          <button onClick={reset} style={{ padding: "0.5rem 1rem", cursor: "pointer" }}>
            Prøv igjen
          </button>
        </div>
      </body>
    </html>
  );
}
