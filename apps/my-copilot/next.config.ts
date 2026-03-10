import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  serverExternalPackages: ["pino", "thread-stream", "@google-cloud/bigquery"],
  async redirects() {
    return [
      { source: "/best-practices", destination: "/practice", permanent: true },
      { source: "/usage", destination: "/stats", permanent: true },
      { source: "/overview", destination: "/cost", permanent: true },
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
