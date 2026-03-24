FROM golang:1.26.1-bookworm AS builder

WORKDIR /src/next/api

COPY next/api/go.mod next/api/go.sum ./
RUN go mod download

COPY next/api ./

RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    fonts-liberation \
    wkhtmltopdf \
    tzdata \
  && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --uid 10001 appuser

WORKDIR /app

COPY --from=builder /out/api /usr/local/bin/api

ENV PORT=8081
ENV CHROME_BIN=/usr/bin/chromium
ENV PDF_RENDERER=wkhtmltopdf

EXPOSE 8081

USER appuser

CMD ["api"]
