FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o foundry-scout ./cmd/foundry-scout

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    adduser -D -u 65532 -g 65532 scout && \
    mkdir -p /data && chown 65532:65532 /data

WORKDIR /app
COPY --from=builder /build/foundry-scout .

USER 65532:65532

ENTRYPOINT ["./foundry-scout"]
