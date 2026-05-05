import type { NextConfig } from "next";

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
      "img-src 'self' blob: data: https://avatars.githubusercontent.com",
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
    remotePatterns: [{ hostname: "avatars.githubusercontent.com" }],
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
  // Enable Cache Components (Partial Prerendering)
  cacheComponents: true,
  turbopack: {
    // Empty config to silence Turbopack migration warning
  },
  experimental: {
    optimizePackageImports: ["@navikt/ds-react", "@navikt/aksel-icons"],
  },
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
  // Keep webpack config for compatibility
  webpack: (config, { isServer }) => {
    if (isServer) {
      config.externals = [...(config.externals || []), "pino", "thread-stream"];
    }
    return config;
  },
};

export default nextConfig;
