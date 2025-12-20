# ============================================
# Stage 1: 构建前端
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/client

# 复制前端依赖配置文件
COPY client/package*.json ./

# 安装依赖
RUN npm ci

# 复制前端源码
COPY client/ ./

# 构建前端
RUN npm run build

# ============================================
# Stage 2: 构建后端
# ============================================
FROM golang:1.23-alpine AS backend-builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git

# 复制 Go 依赖配置文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY main.go ./
COPY server/ ./server/

# 复制前端构建产物（用于 embed）
COPY --from=frontend-builder /app/client/dist ./client/dist

# 构建后端（静态链接，便于在 alpine 中运行）
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o fileflow .

# ============================================
# Stage 3: 最终运行镜像
# ============================================
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1000 fileflow && \
    adduser -D -u 1000 -G fileflow fileflow

# 创建数据目录
RUN mkdir -p /app/data && \
    chown -R fileflow:fileflow /app

WORKDIR /app

# 从构建阶段复制可执行文件
COPY --from=backend-builder --chown=fileflow:fileflow /app/fileflow ./fileflow

# 切换到非 root 用户
USER fileflow

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# 启动应用
CMD ["./fileflow"]
