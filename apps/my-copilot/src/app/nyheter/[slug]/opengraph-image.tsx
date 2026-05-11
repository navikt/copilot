import { ImageResponse } from "next/og";
import { getArticle, CATEGORY_CONFIG } from "@/lib/news";
import { formatDate } from "@/lib/format";

export const runtime = "edge";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

interface Props {
  params: Promise<{ slug: string }>;
}

const CATEGORY_GRADIENTS: Record<string, string> = {
  copilot: "linear-gradient(165deg, #0a1a2e 0%, #162447 35%, #1f4068 65%, #1b3045 100%)",
  nav: "linear-gradient(165deg, #0a1f0c 0%, #0d2818 35%, #143d2b 65%, #0a1f14 100%)",
  "nav-pilot": "linear-gradient(165deg, #0a0a1a 0%, #1a1040 35%, #2d1b69 65%, #0f0a2a 100%)",
  praksis: "linear-gradient(165deg, #1a0f0a 0%, #2e1a0d 35%, #4a2e1b 65%, #221510 100%)",
  oppsummering: "linear-gradient(165deg, #0f0f0f 0%, #1a1a1a 35%, #2a2a2a 65%, #0a0a0a 100%)",
};

const CATEGORY_COLORS: Record<string, string> = {
  copilot: "#60a5fa",
  nav: "#10b981",
  "nav-pilot": "#a78bfa",
  praksis: "#fb923c",
  oppsummering: "#94a3b8",
};

export default async function Image({ params }: Props) {
  const { slug } = await params;
  const article = getArticle(slug);

  if (!article) {
    return new ImageResponse(
      <div
        style={{
          background: "#0f172a",
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <div style={{ fontSize: "48px", color: "white" }}>Artikkel ikke funnet</div>
      </div>,
      { ...size }
    );
  }

  const categoryConfig = CATEGORY_CONFIG[article.category] ?? { label: article.category, variant: "info" as const };
  const gradient = CATEGORY_GRADIENTS[article.category] ?? CATEGORY_GRADIENTS.copilot;
  const accentColor = CATEGORY_COLORS[article.category] ?? CATEGORY_COLORS.copilot;

  return new ImageResponse(
    <div
      style={{
        background: gradient,
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        alignItems: "flex-start",
        justifyContent: "space-between",
        padding: "60px",
      }}
    >
      {/* Header with category and date */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "16px",
        }}
      >
        <div
          style={{
            background: `${accentColor}20`,
            border: `2px solid ${accentColor}`,
            borderRadius: "24px",
            padding: "8px 20px",
            fontSize: "18px",
            fontWeight: 700,
            color: accentColor,
          }}
        >
          {categoryConfig.label}
        </div>
        <div
          style={{
            fontSize: "18px",
            color: "#94a3b8",
          }}
        >
          {formatDate(article.date)}
        </div>
      </div>

      {/* Title */}
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "24px",
          maxWidth: "1000px",
        }}
      >
        <div
          style={{
            fontSize: "56px",
            fontWeight: 800,
            color: "white",
            lineHeight: 1.2,
            display: "flex",
            flexDirection: "column",
          }}
        >
          {article.title}
        </div>
        {article.excerpt && (
          <div
            style={{
              fontSize: "24px",
              color: "#cbd5e1",
              lineHeight: 1.5,
            }}
          >
            {article.excerpt.length > 150 ? `${article.excerpt.slice(0, 150)}...` : article.excerpt}
          </div>
        )}
      </div>

      {/* Footer */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          width: "100%",
        }}
      >
        {article.author && (
          <div
            style={{
              fontSize: "18px",
              color: "#94a3b8",
            }}
          >
            {article.author}
          </div>
        )}
        <div
          style={{
            fontSize: "18px",
            color: "#64748b",
            marginLeft: "auto",
          }}
        >
          copilot.nav.no
        </div>
      </div>
    </div>,
    { ...size }
  );
}

export async function generateStaticParams() {
  // This is handled by the parent route
  return [];
}
