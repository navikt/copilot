"use client";

import { faro } from "@grafana/faro-web-sdk";
import { useEffect } from "react";

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  useEffect(() => {
    faro.api?.pushError(error);
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
