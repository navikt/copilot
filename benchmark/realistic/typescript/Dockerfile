FROM node@sha256:be23f54a88d34e8824c741b19b91064094f92c1c97b194144bfc8b50d67258e2
WORKDIR /deps
COPY benchmark/realistic/typescript/feature/workspace/package.json benchmark/realistic/typescript/feature/workspace/package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
