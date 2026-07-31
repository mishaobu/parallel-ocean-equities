# syntax=docker/dockerfile:1.7
FROM node:24.18.1-alpine3.24@sha256:f70403e87646dc51b45295f4b8b70cdad0b63d2297c4c9899119b03f7af7a6b3 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM node:24.18.1-alpine3.24@sha256:f70403e87646dc51b45295f4b8b70cdad0b63d2297c4c9899119b03f7af7a6b3 AS monetary
WORKDIR /src/monetary
COPY monetary/package.json monetary/package-lock.json ./
RUN npm ci
COPY monetary/ ./
RUN npm run build

FROM node:24.18.1-alpine3.24@sha256:f70403e87646dc51b45295f4b8b70cdad0b63d2297c4c9899119b03f7af7a6b3 AS macro
WORKDIR /src/macro
COPY macro/package.json macro/package-lock.json ./
RUN npm ci
COPY macro/ ./
RUN npm run build

FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS api
WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/equities ./cmd/equities

FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN addgroup -S -g 10001 equities && adduser -S -D -H -u 10001 -G equities equities
WORKDIR /app
COPY --from=api /out/equities /app/equities
COPY --from=web /src/web/dist /app/web
COPY --from=monetary /src/monetary/dist /app/monetary
COPY --from=macro /src/macro/dist /app/macro
COPY data/seed.json /app/data/seed.json
COPY scripts/verify-startup-refresh.sh /app/verify-startup-refresh
RUN chmod 0555 /app/verify-startup-refresh && mkdir /data && chown equities:equities /data
USER 10001:10001
EXPOSE 8080
ENV PORT=8080 BASE_PATH=/equities STATIC_DIR=/app/web MONETARY_PATH=/monetary MONETARY_STATIC_DIR=/app/monetary MACRO_PATH=/macro MACRO_STATIC_DIR=/app/macro DATA_FILE=/data/state.json SEED_FILE=/app/data/seed.json
ENTRYPOINT ["/app/equities"]
