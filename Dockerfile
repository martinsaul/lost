# syntax=docker/dockerfile:1

# ---- Stage 1: build the React SPA ----
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# ---- Stage 2: build the Go binary (SPA embedded) ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Bring in the freshly built SPA so //go:embed all:web/dist picks it up.
COPY --from=web /web/dist ./web/dist
# Pure-Go (CGO_ENABLED=0) — modernc sqlite needs no C toolchain, so the result
# is a fully static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/lost .

# ---- Stage 3: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/lost /app/lost
# Default data dir for the SQLite file (override LOST_DB_URL for Postgres).
VOLUME ["/data"]
ENV LOST_ADDR=":8080" \
    LOST_DB_URL="sqlite:///data/lost.db"
EXPOSE 8080
ENTRYPOINT ["/app/lost"]
