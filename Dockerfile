# syntax=docker/dockerfile:1.7
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM node:20-alpine AS monetary
WORKDIR /src/monetary
COPY monetary/package.json monetary/package-lock.json ./
RUN npm ci
COPY monetary/ ./
RUN npm run build

FROM node:20-alpine AS macro
WORKDIR /src/macro
COPY macro/package.json macro/package-lock.json ./
RUN npm ci
COPY macro/ ./
RUN npm run build

FROM golang:1.23-alpine AS api
WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/equities ./cmd/equities

FROM alpine:3.21
RUN addgroup -S -g 10001 equities && adduser -S -D -H -u 10001 -G equities equities
WORKDIR /app
COPY --from=api /out/equities /app/equities
COPY --from=web /src/web/dist /app/web
COPY --from=monetary /src/monetary/dist /app/monetary
COPY --from=macro /src/macro/dist /app/macro
COPY data/seed.json /app/data/seed.json
RUN mkdir /data && chown -R equities:equities /data /app
USER 10001:10001
EXPOSE 8080
ENV PORT=8080 BASE_PATH=/equities STATIC_DIR=/app/web MONETARY_PATH=/monetary MONETARY_STATIC_DIR=/app/monetary MACRO_PATH=/macro MACRO_STATIC_DIR=/app/macro DATA_FILE=/data/state.json SEED_FILE=/app/data/seed.json
ENTRYPOINT ["/app/equities"]
