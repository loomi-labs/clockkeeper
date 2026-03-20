# Stage 1: Generate protobuf TypeScript code
FROM bufbuild/buf:latest AS protogen
WORKDIR /app
COPY buf.yaml buf.gen.yaml ./
COPY proto/ ./proto/
RUN buf generate

# Stage 2: Build frontend
FROM node:22-alpine AS frontend
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
COPY --from=protogen /app/web/src/lib/gen/ ./src/lib/gen/
RUN pnpm run build

# Stage 3: Build Go binary
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/build ./web/build
RUN go build -o clockkeeper ./cmd/

# Stage 4: Atlas
FROM arigaio/atlas:latest-alpine AS atlas

# Stage 5: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/clockkeeper .
COPY --from=atlas /atlas /usr/local/bin/atlas
COPY --from=backend /app/ent/migrate/migrations /app/migrations
EXPOSE 8080
CMD ["./clockkeeper"]
