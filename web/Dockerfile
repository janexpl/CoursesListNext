FROM node:22-bookworm-slim AS builder

WORKDIR /app

# Projekt używa pnpm (pole packageManager w package.json wskazuje wersję).
# Corepack pobiera dokładnie ją, więc obraz buduje się tym samym menedżerem
# co środowisko deweloperskie - inaczej w drzewie zależności lądują dwie kopie
# tych samych pakietów (np. unhead) i aplikacja wywala się przy renderowaniu.
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

RUN pnpm build

FROM node:22-bookworm-slim

WORKDIR /app

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000

COPY --from=builder /app/.output ./.output
COPY --from=builder /app/package.json ./package.json

EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
