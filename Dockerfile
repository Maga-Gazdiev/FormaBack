FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /converter ./cmd/api
FROM debian:bookworm-slim
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN sed -i 's|http://deb.debian.org|https://deb.debian.org|g' /etc/apt/sources.list.d/debian.sources && apt-get -o Acquire::Retries=2 -o Acquire::https::Timeout=30 update && apt-get install -y --no-install-recommends libreoffice-writer libreoffice-calc libreoffice-impress poppler-utils imagemagick pandoc ffmpeg fonts-dejavu fonts-liberation && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --uid 10001 converter
COPY --from=build /converter /usr/local/bin/converter
USER converter
ENV HOME=/home/converter
EXPOSE 8080
CMD ["converter"]
