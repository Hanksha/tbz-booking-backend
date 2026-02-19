# --- Build stage ---
FROM golang:1.25 AS builder
WORKDIR /app
# Cache deps
COPY go.mod go.sum ./
RUN go mod download
# Copy source
COPY . .
# Build a static-ish binary (works well on Alpine runtime)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/server ./
# --- Runtime stage ---
FROM alpine:3.20
WORKDIR /app
# (optional) SSL certs if you ever call HTTPS from the app
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/server /app/server
# Gin default in your logs is :8080
EXPOSE 8080
# Your app reads DATABASE_URL, and Gin respects PORT if you use it.
# (Your code currently calls r.Run() which uses PORT if set, else 8080.)
ENV PORT=8080
CMD ["/app/server"]