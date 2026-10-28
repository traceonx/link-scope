# 单镜像：Go 后端 + 前端页面（嵌入可执行文件），同一端口对外提供服务。
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod VERSION ./
COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY --from=web /src/web/dist ./web/dist
ARG APP_VERSION= GIT_SHA=dev BUILD_TIME=
RUN v="${APP_VERSION:-$(cat VERSION)}" && pkg=github.com/traceonx/link-scope/internal/version && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X $pkg.Version=$v -X $pkg.Commit=${GIT_SHA} -X '$pkg.BuildTime=${BUILD_TIME}'" \
      -o /out/linkscope ./cmd/linkscope

FROM alpine:3.20 AS app
RUN apk add --no-cache tzdata ca-certificates su-exec
ENV LS_PORT=8090 LS_DATA_DIR=/app/data TZ=Asia/Shanghai
RUN mkdir -p /app/data
COPY --from=build /out/linkscope /usr/local/bin/linkscope
COPY --chmod=755 deploy/entrypoint.sh /entrypoint.sh
VOLUME /app/data
EXPOSE 8090
HEALTHCHECK --interval=30s --start-period=10s --timeout=5s \
  CMD wget -qO- "http://127.0.0.1:${LS_PORT}/api/health" >/dev/null || exit 1
ENTRYPOINT ["/entrypoint.sh"]
