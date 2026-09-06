import { ImageResponse } from "next/og";

export const runtime = "edge";
export const alt = "cplt — Sandbox for AI coding agents";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function Image() {
  return new ImageResponse(
    <div
      style={{
        background: "linear-gradient(165deg, #0a0f0c 0%, #0d2118 35%, #143d2b 65%, #0a1f14 100%)",
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
          color: "#10b981",
          fontFamily: "monospace",
          marginBottom: "24px",
        }}
      >
        cplt
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
        Sandbox for AI coding agents
      </div>
      <div
        style={{
          fontSize: "20px",
          color: "#94a3b8",
          textAlign: "center",
          maxWidth: "800px",
        }}
      >
        Kernel-level isolation · macOS &amp; Linux · Network proxy · Credential protection
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
        navikt/cplt
      </div>
    </div>,
    { ...size }
  );
}
