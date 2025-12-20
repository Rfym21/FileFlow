# Stage 1: 构建前端
FROM node:20-alpine AS frontend
WORKDIR /app/client
COPY client/ .
RUN npm install && npm run build

# Stage 2: 构建后端
FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY server/ ./server/
COPY --from=frontend /app/client/dist ./client/dist
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o fileflow .

# Stage 3: 运行镜像
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/fileflow .
EXPOSE 8080
CMD ["./fileflow"]
