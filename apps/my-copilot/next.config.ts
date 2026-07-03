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

const nextConfig: NextConfig = {
  output: "standalone",
  serverExternalPackages: ["pino", "thread-stream", "@google-cloud/bigquery"],
  images: {
    remotePatterns: [{ hostname: "avatars.githubusercontent.com" }, { hostname: "storage.googleapis.com" }],
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
  async redirects() {
    return [
      { source: "/best-practices", destination: "/praksis", permanent: true },
      { source: "/practice", destination: "/praksis", permanent: true },
      { source: "/customizations", destination: "/verktoy", permanent: true },
      { source: "/usage", destination: "/statistikk", permanent: true },
      { source: "/stats", destination: "/statistikk", permanent: true },
      { source: "/overview", destination: "/kostnad", permanent: true },
      { source: "/cost", destination: "/kostnad", permanent: true },
    ];
  },
  // Enable Cache Components (Partial Prerendering) — disabled in dev because the
  // per-request Prerender environment it spawns in __NEXT_DEV_SERVER mode causes
  // sustained high CPU (500%+) and memory growth. Only enable in production builds.
  ...(isProduction ? { cacheComponents: true } : {}),
  // Disable dev indicators to avoid the executionId-based reload loop during startup.
  ...(!isProduction ? { devIndicators: false } : {}),
  turbopack: {
    // Dev: monorepo root so Turbopack resolves workspace deps and serves chunks after HMR.
    // Prod: app root so standalone output isn't nested under apps/my-copilot/ (see 06f6c00d).
    root: isProduction ? path.resolve(".") : path.resolve("../.."),
  },
  experimental: {
    optimizePackageImports: ["@navikt/ds-react", "@navikt/aksel-icons"],
    // In dev, give the router cache a generous stale window so Turbopack HMR events
    // during slow backend renders don't trigger refetches that restart the render.
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
