FROM node@sha256:be23f54a88d34e8824c741b19b91064094f92c1c97b194144bfc8b50d67258e2
WORKDIR /deps
COPY benchmark/realistic/typescript/tilgang/workspace/package.json benchmark/realistic/typescript/tilgang/workspace/package-lock.json ./
# @navikt/oasis and @navikt/texas are only on GitHub Packages. The secret is an
# .npmrc with a read:packages token; it is not kept in any image layer.
RUN --mount=type=secret,id=npmrc,target=/root/.npmrc npm ci --ignore-scripts --no-audit --no-fund
