# syntax=docker/dockerfile:1

# ---- 前端构建（在原生构建平台上跑，产物是平台无关的静态文件）----
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- Go 构建（交叉编译到目标架构，原生跑、不走 QEMU 模拟，很快）----
FROM --platform=$BUILDPLATFORM golang:1.25 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# modernc.org/sqlite 是纯 Go，CGO_ENABLED=0 可静态编译
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/omc ./cmd/server

# ---- 运行时 ----
# 运行时阶段只做 COPY/ENV/ENTRYPOINT，不执行任何目标架构二进制，
# 这样在 arm64 机器上交叉构建 amd64 镜像时无需 QEMU 模拟。
FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/omc /app/omc
COPY --from=web /web/dist /app/web/dist
# 从构建镜像带上 CA 证书，供访问 DashScope / 火山 Ark 的 HTTPS 使用
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# DATA_DIR/DB_PATH 指向挂载卷 /data（由 compose 提供，无需 mkdir）；WEB_DIST 让 Go 托管前端 SPA
ENV WEB_DIST=/app/web/dist \
    DATA_DIR=/data \
    DB_PATH=/data/oh-my-commic.db \
    PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/omc"]
