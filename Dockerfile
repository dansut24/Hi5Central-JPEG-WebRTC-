# Build only the standalone JPEG signalling relay and web viewer.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/relay ./cmd/relay
COPY web ./web
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /jpeg-relay ./cmd/relay

FROM alpine:3.22
RUN addgroup -S jpeg && adduser -S -G jpeg jpeg
COPY --from=build /jpeg-relay /usr/local/bin/jpeg-relay
COPY --from=build /src/web /opt/jpeg/web
ENV JPEG_LISTEN=:8787 JPEG_WEB_DIR=/opt/jpeg/web
USER jpeg
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/jpeg-relay"]
