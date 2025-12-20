# FileFlow

FileFlow 是一个 Cloudflare R2 云存储管理应用，提供统一的 Web 界面管理多个 R2 存储账户，支持文件上传、下载、删除，以及账户配额和用量追踪。

## 功能特点

- **多账户管理** - 添加、编辑、删除多个 R2 存储账户，支持配额限制和启用/禁用控制
- **文件操作** - 懒加载目录浏览、上传、删除文件，支持生成公开访问链接
- **智能上传** - 自动选择用量最低的账户进行上传，超额时自动切换备用账户
- **用量同步** - 每 5 分钟自动同步存储用量和操作次数统计
- **清空存储桶** - 一键清空指定账户的所有文件
- **反向代理** - 可选的内置反向代理，隐藏 R2 源站地址
- **API Token** - 支持生成可撤销的 API Token，用于程序化访问
- **开放 API** - 完整的 RESTful API，支持文件列表、上传、删除、获取链接

## 快速开始

### 环境要求

- Go 1.23+
- Node.js 20+
- Docker（可选，用于容器化部署）

### 本地开发

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

### 编译构建

```bash
# 构建前端
cd client && npm ci && npm run build && cd ..

# 构建后端（前端会嵌入到二进制文件中）
go build -o fileflow .
```

## Docker 部署

**构建镜像：**

```bash
docker build -f docker/Dockerfile -t fileflow .
```

**运行容器：**

```bash
docker run -d \
  -p 8080:8080 \
  -e FILEFLOW_ADMIN_USER=admin \
  -e FILEFLOW_ADMIN_PASSWORD=your_password \
  -e FILEFLOW_JWT_SECRET=your_jwt_secret \
  -v ./data:/app/data \
  fileflow
```

**Docker Compose：**

```yaml
services:
  fileflow:
    build:
      context: .
      dockerfile: docker/Dockerfile
    ports:
      - "8080:8080"
    environment:
      - FILEFLOW_ADMIN_USER=admin
      - FILEFLOW_ADMIN_PASSWORD=your_password
      - FILEFLOW_JWT_SECRET=your_jwt_secret
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

## 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `FILEFLOW_ADMIN_USER` | 否 | admin | 管理员用户名 |
| `FILEFLOW_ADMIN_PASSWORD` | 是 | - | 管理员密码 |
| `FILEFLOW_JWT_SECRET` | 是 | - | JWT 签名密钥 |
| `FILEFLOW_PORT` | 否 | 8080 | 服务端口 |
| `FILEFLOW_DATA_DIR` | 否 | ./data | 数据存储目录 |
| `ENDPOINT_PROXY` | 否 | false | 是否启用反向代理 |
| `ENDPOINT_PROXY_URL` | 否 | - | 反向代理 URL 前缀 |

### 反向代理配置

启用反向代理后，返回的文件 URL 将通过 FileFlow 服务器转发，隐藏 R2 源站地址：

```env
ENDPOINT_PROXY=true
ENDPOINT_PROXY_URL=https://your-domain.com/p
```

**URL 转换示例：**
- 原始：`https://pub-xxx.r2.dev/path/to/file.png`
- 代理：`https://your-domain.com/p/pub-xxx/path/to/file.png`

## R2 账户配置

添加 R2 存储账户时需要提供：

| 字段 | 说明 |
|------|------|
| Cloudflare Account ID | Cloudflare 账户 ID |
| Access Key ID | R2 访问密钥 ID |
| Secret Access Key | R2 访问密钥 |
| Bucket Name | 存储桶名称 |
| Endpoint | R2 端点 URL（如 `https://{accountid}.r2.cloudflarestorage.com`） |
| Public Domain | 公开访问域名（用于生成文件链接） |
| API Token | Cloudflare API Token（用于获取用量统计，可选） |

## 开放 API

FileFlow 提供 RESTful API 供外部应用调用，需使用 API Token 认证。

### 认证方式

```bash
curl -H "Authorization: Bearer your-api-token" https://your-domain/api/files
```

### API 端点

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/files` | read | 获取文件列表（懒加载+分页） |
| POST | `/api/upload` | write | 上传文件 |
| GET | `/api/link` | read | 获取文件公开链接 |
| DELETE | `/api/file` | delete | 删除文件 |

### 请求参数

**GET /api/files**
- `idGroup` - 账户 ID（逗号分隔多个）
- `prefix` - 目录前缀
- `cursor` - 分页游标
- `limit` - 每页数量（默认 50，最大 100）

**POST /api/upload**（multipart/form-data）
- `file` - 上传的文件（必填）
- `path` - 自定义存储路径
- `idGroup` - 指定账户 ID

**GET /api/link** / **DELETE /api/file**
- `idGroup` - 账户 ID（必填）
- `key` - 文件路径（必填）

## 项目结构

```
FileFlow/
├── main.go              # 程序入口
├── server/
│   ├── api/             # HTTP 处理器
│   ├── config/          # 配置管理
│   ├── middleware/      # 中间件
│   ├── service/         # 业务逻辑
│   └── store/           # 数据存储
├── client/              # React 前端
│   ├── src/
│   │   ├── components/  # UI 组件
│   │   ├── pages/       # 页面
│   │   └── lib/         # 工具函数
│   └── dist/            # 构建产物
└── docker/              # Docker 配置
```

## License

MIT
