FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/kryla-server ./cmd/kryla-server

# ── runtime ─────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/kryla-server /usr/local/bin/kryla-server

EXPOSE 8443

ENTRYPOINT ["kryla-server"]
