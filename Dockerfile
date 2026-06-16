# syntax=docker/dockerfile:1

# ---- Stage 1: build the frontend ----
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- Stage 2: build the Go server (pure Go, no CGO) ----
FROM golang:1.25-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/murmur .

# ---- Stage 3: minimal runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 murmur
WORKDIR /app
COPY --from=server /out/murmur /app/murmur
COPY --from=web /web/dist /app/web/dist
ENV PORT=8080 \
    DB_PATH=/data/murmur.db \
    UPLOAD_DIR=/data/uploads
RUN mkdir -p /data && chown -R murmur /data /app
USER murmur
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/murmur"]
