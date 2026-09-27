import type { NextConfig } from "next";
import path from "node:path";

const isProduction = process.env.NODE_ENV === "production";

const securityHeaders = [
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "no-referrer-when-downgrade" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  {
    key: "Content-Security-Policy",
    value: [
      "default-src 'self'",
      `script-src 'self' 'unsafe-inline'${isProduction ? "" : " 'unsafe-eval'"}`,
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' blob: data: https://avatars.githubusercontent.com https://github.com https://storage.googleapis.com https://*.storage.googleapis.com https://*.googleusercontent.com",
      "media-src 'self' blob: data: https://storage.googleapis.com",
      "font-src 'self' data: https://cdn.nav.no",
      "connect-src 'self' https://telemetry.ekstern.dev.nav.no https://telemetry.nav.no",
      "frame-ancestors 'self'",
      "base-uri 'self'",
      "form-action 'self'",
      "object-src 'none'",
      ...(isProduction ? ["upgrade-insecure-requests"] : []),
    ].join("; "),
  },
];

// The app was called min-copilot until 2026. The old ingresses stay in
// .nais/*.yaml so old links keep working; this sends them to the new name.
// Paths that must stay on the host they came in on are left alone:
// /oauth2 (Wonderwall, which answers before Next anyway), /api, /internal,
// /health (probes use the pod IP and never match; kept for manual checks).
// Next never redirects /_next itself, so assets for pages already open on the
// old host keep loading.
const OLD_HOSTS: Record<string, string> = {
  "min-copilot.intern.nav.no": "ki-utvikling.nav.no",
  "min-copilot.ansatt.nav.no": "ki-utvikling.nav.no",
  "min-copilot.intern.dev.nav.no": "ki-utvikling.ekstern.dev.nav.no",
  "min-copilot.ansatt.dev.nav.no": "ki-utvikling.ekstern.dev.nav.no",
};
const HOST_REDIRECT_SOURCE = "/:path((?!(?:oauth2|api|internal|health)(?:/|$)).*)";

const nextConfig: NextConfig = {
  output: "standalone",
  serverExternalPackages: ["pino", "thread-stream", "@google-cloud/bigquery"],
  // sharp 0.35 flyttet lasteren fra lib/*.js til dist/*.cjs og rullet ut
  // plattformvalget til en switch med literale `@img/sharp-<plattform>/sharp.node`.
  // Next sin filsporing finner da .node-bindingen, men aldri søsterpakken
  // `@img/sharp-libvips-*` som den lenker mot, fordi den lenken er en native
  // dyld-lenke og ikke en require. Uten dette havner standalone-utdataene uten
  // libvips, sharp kaster ERR_DLOPEN_FAILED, og Next serverer stille
  // originalbildene i stedet for å skalere dem. Se PR #525.
  outputFileTracingIncludes: {
    "/**/*": ["./node_modules/**/@img/**"],
  },
  images: {
    remotePatterns: [{ hostname: "avatars.githubusercontent.com" }, { hostname: "storage.googleapis.com" }],
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
  async redirects() {
    return [
      // Next matches `host` against the Host header, which the ingress and
      // Wonderwall pass through unchanged. The query string is kept.
      ...Object.entries(OLD_HOSTS).map(([oldHost, newHost]) => ({
        source: HOST_REDIRECT_SOURCE,
        has: [{ type: "host" as const, value: oldHost }],
        destination: `https://${newHost}/:path`,
        permanent: true,
      })),
      { source: "/en", destination: "/en/news", permanent: false },
      // /nyheter has no index page; the news list is the front page. It used to
      // answer 200 with the not-found body, so old links to it exist.
      { source: "/nyheter", destination: "/", permanent: false },
      { source: "/best-practices", destination: "/praksis", permanent: true },
      { source: "/practice", destination: "/praksis", permanent: true },
      { source: "/customizations", destination: "/verktoy", permanent: true },
      { source: "/usage", destination: "/statistikk", permanent: true },
      { source: "/stats", destination: "/statistikk", permanent: true },
      { source: "/overview", destination: "/kostnad", permanent: true },
      { source: "/cost", destination: "/kostnad", permanent: true },
      // The WRAP guide was merged into the prompt guide in #321.
      { source: "/praksis/guide/wrap-metoden", destination: "/praksis/guide/skrive-presise-prompts", permanent: true },
    ];
  },
  // Enable Cache Components (Partial Prerendering) — disabled in dev because the
  // per-request Prerender environment it spawns in __NEXT_DEV_SERVER mode causes
  // sustained high CPU (500%+) and memory growth. Only enable in production builds.
  ...(isProduction ? { cacheComponents: true } : {}),
  // IMPORTANT: devIndicators must be false in dev. The indicator component injects
  // an executionId that changes on each lazy-compile, causing chunk hash mismatches
  // that trigger location.reload() loops during Turbopack startup on large pages.
  ...(!isProduction ? { devIndicators: false } : {}),
  turbopack: {
    // Dev: monorepo root so Turbopack resolves workspace deps and serves chunks after HMR.
    // Prod: app root so standalone output isn't nested under apps/my-copilot/ (see 06f6c00d).
    root: isProduction ? path.resolve(".") : path.resolve("../.."),
  },
  experimental: {
    // A global not-found needs this flag in 16.x. Without it Next wraps the
    // page in a builtin layout that already renders html/body, and this file
    // renders its own, so an unmatched URL served nested documents.
    globalNotFound: true,
    optimizePackageImports: ["@navikt/ds-react", "@navikt/aksel-icons"],
    // IMPORTANT (dev only): Without staleTimes, Turbopack HMR events during cold
    // BigQuery cache warmup trigger client-side refetches that produce different
    // server output (error→success or vice versa), causing hash mismatches that
    // loop location.reload() 15-20x. The 30s dynamic stale window prevents this
    // by letting the router cache serve the initial render while the backend warms.
    ...(!isProduction ? { staleTimes: { dynamic: 30, static: 180 } } : {}),
  },
  ...(isProduction
    ? {
        // Cache configuration for different data types
        cacheLife: {
          // GitHub API data refreshes infrequently
          github: {
            stale: 300, // 5 minutes until considered stale
            revalidate: 3600, // 1 hour until revalidated
            expire: 86400, // 1 day until expired
          },
          // User session data
          session: {
            stale: 60, // 1 minute until considered stale
            revalidate: 300, // 5 minutes until revalidated
            expire: 3600, // 1 hour until expired
          },
          // Static content like navigation
          static: {
            stale: 3600, // 1 hour until considered stale
            revalidate: 86400, // 1 day until revalidated
            expire: 604800, // 1 week until expired
          },
        },
      }
    : {}),
  // Keep webpack config for compatibility
  webpack: (config, { isServer }) => {
    if (isServer) {
      config.externals = [...(config.externals || []), "pino", "thread-stream"];
    }
    return config;
  },
};

export default nextConfig;
