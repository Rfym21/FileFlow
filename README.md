# FileFlow

FileFlow 是一个 Cloudflare R2 云存储管理应用，提供统一的 Web 界面管理多个 R2 存储账户，支持文件上传、下载、删除，以及账户配额和用量追踪。

## 功能特点

- **多账户管理** - 添加、编辑、删除多个 R2 存储账户，支持配额限制和激活/停用控制
- **文件操作** - 列表展示、上传、删除文件，支持生成公开访问链接
- **智能上传** - 自动选择用量最低的账户进行上传，超额时自动切换备用账户
- **用量同步** - 定时同步 Cloudflare 存储用量和操作次数统计
- **API Token** - 支持生成可撤销的 API Token，用于程序化访问
- **管理仪表盘** - 账户概览、用量统计、Token 管理、API 文档

## 快速开始

### 环境要求

- Go 1.25.0+
- Node.js 20+
- Docker & docker-compose（可选，用于容器化部署）

### 本地运行

1. **配置环境变量**

```bash
cp .env.example .env
```

编辑 `.env` 文件，设置必要的配置项：

```env
FILEFLOW_ADMIN_USER=admin
FILEFLOW_ADMIN_PASSWORD=your_password
FILEFLOW_JWT_SECRET=your_jwt_secret
FILEFLOW_PORT=8080
FILEFLOW_DATA_DIR=./data
```

2. **构建前端**

```bash
cd client
npm ci
npm run build
cd ..
```

3. **运行服务**

```bash
go run main.go
```

服务启动后访问 `http://localhost:8080`

## Docker 部署

```bash
cd docker
docker-compose up -d
```

Docker Compose 配置示例：

```yaml
services:
  fileflow:
    image: ghcr.io/your-org/fileflow:latest
    ports:
      - "8080:8080"
    environment:
      - FILEFLOW_ADMIN_USER=admin
      - FILEFLOW_ADMIN_PASSWORD=your_password
      - FILEFLOW_JWT_SECRET=your_jwt_secret
    volumes:
      - ./data:/app/data
```

## 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `FILEFLOW_ADMIN_USER` | 否 | admin | 管理员用户名 |
| `FILEFLOW_ADMIN_PASSWORD` | 是 | - | 管理员密码 |
| `FILEFLOW_JWT_SECRET` | 是 | - | JWT 签名密钥 |
| `FILEFLOW_PORT` | 否 | 8080 | 服务端口 |
| `FILEFLOW_DATA_DIR` | 否 | ./data | 数据存储目录 |

## R2 账户配置

添加 R2 存储账户时需要提供：

- **Cloudflare Account ID** - Cloudflare 账户 ID
- **Access Key ID / Secret** - R2 访问密钥
- **Bucket Name** - 存储桶名称
- **Endpoint** - R2 端点 URL（如 `https://{accountid}.r2.cloudflarestorage.com`）
- **Public Domain** - 公开访问域名（用于生成文件链接）
- **API Token**（可选）- 用于获取用量统计的 Cloudflare API Token
