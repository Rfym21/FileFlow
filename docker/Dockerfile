# Stage 1: 构建前端
FROM node:20-alpine AS frontend
WORKDIR /app/client
# 先复制依赖文件，利用缓存
COPY client/package*.json ./
RUN npm ci --prefer-offline
# 再复制源码并构建
COPY client/ .
RUN npm run build

# Stage 2: 构建后端
FROM golang:1.24-alpine AS backend
WORKDIR /app
# 先下载依赖，利用缓存
COPY go.mod go.sum ./
RUN go mod download
# 复制源码
COPY main.go ./
COPY server/ ./server/
COPY --from=frontend /app/client/dist ./client/dist
# 构建
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o fileflow .

# Stage 3: 运行镜像
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/fileflow .
EXPOSE 8080
CMD ["./fileflow"]
