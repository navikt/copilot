import { ImageResponse } from "next/og";

export const runtime = "edge";
export const alt = "nav-pilot — Copilot i Nav";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function Image() {
  return new ImageResponse(
    <div
      style={{
        background: "linear-gradient(165deg, #0a0a1a 0%, #1a1040 35%, #2d1b69 65%, #0f0a2a 100%)",
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        padding: "60px",
      }}
    >
      <div
        style={{
          fontSize: "72px",
          fontWeight: 800,
          color: "#a78bfa",
          fontFamily: "monospace",
          marginBottom: "24px",
        }}
      >
        nav-pilot
      </div>
      <div
        style={{
          fontSize: "36px",
          fontWeight: 700,
          color: "white",
          textAlign: "center",
          marginBottom: "20px",
        }}
      >
        Copilot med Navs kunnskap
      </div>
      <div
        style={{
          fontSize: "20px",
          color: "#94a3b8",
          textAlign: "center",
          maxWidth: "800px",
        }}
      >
        Nais · TokenX · Aksel · Kotlin/Ktor · Rapids &amp; Rivers · rett i editoren
      </div>
      <div
        style={{
          position: "absolute",
          bottom: "40px",
          right: "60px",
          fontSize: "18px",
          color: "#64748b",
        }}
      >
        navikt/nav-pilot
      </div>
    </div>,
    { ...size }
  );
}
