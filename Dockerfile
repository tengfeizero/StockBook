# 多阶段构建：前端 → 内嵌 → 单文件二进制
# 构建：docker build -t huoben .
# 运行：docker run -d -p 8080:8080 -v huoben-data:/app/data --name huoben huoben

FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /src/web/dist ./internal/webembed/webdist/
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/huoben ./cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 huoben
WORKDIR /app
COPY --from=build /out/huoben /app/huoben
RUN mkdir -p /app/data && chown -R huoben:huoben /app
USER huoben

ENV HUOBEN_PORT=8080 \
    HUOBEN_DB_PATH=/app/data/huoben.db
VOLUME ["/app/data"]
EXPOSE 8080

# 镜像内不设置 HUOBEN_JWT_SECRET：首次启动会自动生成 /app/data/.jwt_secret，
# 容器重建只要挂载了数据卷，登录态就能延续。
ENTRYPOINT ["/app/huoben"]
