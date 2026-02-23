# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/server ./cmd/server/

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/bin/server ./bin/server
COPY --from=frontend /app/web/dist ./web/dist
RUN mkdir -p /data

ENV PORT=8080
ENV DB_PATH=/data/cluster.db

EXPOSE 8080
VOLUME ["/data"]
CMD ["./bin/server"]
