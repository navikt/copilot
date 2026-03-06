FROM node:24 AS base

RUN corepack enable

FROM base AS deps
WORKDIR /app

COPY apps/my-copilot/package.json apps/my-copilot/pnpm-lock.yaml apps/my-copilot/.npmrc ./
RUN pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY apps/my-copilot/ .
COPY docs/news/articles ./docs/news/articles

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

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