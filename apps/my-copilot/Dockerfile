FROM node:24 AS base

RUN corepack enable

FROM base AS deps
WORKDIR /app

COPY apps/my-copilot/package.json apps/my-copilot/pnpm-lock.yaml apps/my-copilot/pnpm-workspace.yaml apps/my-copilot/.npmrc ./
# @nais/apm is published to GitHub Package Registry, which requires an
# authenticated token (read:packages) even for public packages. The token is
# passed as a BuildKit secret — never an ARG/ENV — so it is not persisted in any
# image layer. .npmrc reads it from the GITHUB_PACKAGES_TOKEN env var.
RUN --mount=type=secret,id=github_packages_token \
    GITHUB_PACKAGES_TOKEN="$(cat /run/secrets/github_packages_token)" pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY apps/my-copilot/ .
COPY docs/news/articles ./docs/news/articles

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV GITHUB_ORG=navikt

ARG APP_VERSION=unknown
ENV NEXT_PUBLIC_APP_VERSION=${APP_VERSION}

RUN pnpm build

FROM gcr.io/distroless/nodejs24:nonroot AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Copy standalone output first
COPY --from=builder --chown=nonroot:nonroot /app/.next/standalone ./
# Copy static assets into standalone structure
COPY --from=builder --chown=nonroot:nonroot /app/.next/static ./.next/static
COPY --from=builder --chown=nonroot:nonroot /app/public ./public
# Copy news article markdown files for server-side rendering
COPY --from=builder --chown=nonroot:nonroot /app/docs/news/articles ./docs/news/articles

USER nonroot

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME=0.0.0.0

CMD ["server.js"]
