# ---- 1. 构建前端 ----
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- 2. 构建 Go 程序（前端打包进可执行文件） ----
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/linkscope ./cmd/linkscope

# ---- 3. 运行镜像 ----
FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates
COPY --from=build /out/linkscope /usr/local/bin/linkscope
ENV LS_PORT=8090 \
    LS_DATA_DIR=/config \
    TZ=Asia/Singapore
VOLUME /config
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- "http://127.0.0.1:${LS_PORT}/api/config" >/dev/null || exit 1
ENTRYPOINT ["linkscope"]
