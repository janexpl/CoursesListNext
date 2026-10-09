# Obrazy bazowe z lustra Docker Huba prowadzonego przez Google (te same oficjalne obrazy).
# Anonimowe pobrania z Docker Huba na runnerach GitHuba kończyły się 429 Too Many Requests
# i blokowały deploy.
FROM mirror.gcr.io/library/golang:1.26.1-bookworm AS builder

WORKDIR /src/notifications

COPY go.mod ./

COPY . .

RUN CGO_ENABLED=0 go build -o /out/notifications ./cmd/notifications

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata \
  && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --uid 10001 appuser
RUN mkdir -p /data && chown appuser:appuser /data

WORKDIR /app

COPY --from=builder /out/notifications /usr/local/bin/notifications

USER appuser

CMD ["notifications"]
