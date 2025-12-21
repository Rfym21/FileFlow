# FileFlow

FileFlow 是一个 Cloudflare R2 云存储管理应用，提供统一的 Web 界面管理多个 R2 存储账户，支持文件上传、下载、删除，以及账户配额和用量追踪。

## 功能特点

- **多账户管理** - 添加、编辑、删除多个 R2 存储账户，支持配额限制和启用/禁用控制
- **文件操作** - 懒加载目录浏览、拖拽上传、删除文件，支持生成公开访问链接
- **智能上传** - 自动选择用量最低的账户进行上传，超额时自动切换备用账户
- **用量同步** - 可配置的自动同步间隔（默认 5 分钟），支持热重载
- **清空存储桶** - 一键清空指定账户的所有文件
- **反向代理** - 内置反向代理 + 外置代理脚本（Workers/Deno/Go），隐藏 R2 源站地址
- **多数据库支持** - 支持 SQLite、MySQL、PostgreSQL、Redis、MongoDB、Turso
- **API Token** - 支持生成可撤销的 API Token，用于程序化访问
- **开放 API** - 完整的 RESTful API，支持文件列表、上传、删除、获取链接
- **内置指南** - Web 界面内置参数获取指南、代理部署教程、API 文档

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

**运行容器：**

```bash
docker run -d \
  -p 8080:8080 \
  -e FILEFLOW_ADMIN_USER=admin \
  -e FILEFLOW_ADMIN_PASSWORD=your_password \
  -e FILEFLOW_JWT_SECRET=your_jwt_secret \
  -e TZ=Asia/Shanghai \
  -v fileflow_data:/app/data \
  ghcr.io/rfym21/file-flow:latest
```

**Docker Compose：**

```yaml
services:
  file-flow:
    image: ghcr.io/rfym21/file-flow:latest
    container_name: FileFlow
    restart: always
    ports:
      - "8080:8080"
    environment:
      - FILEFLOW_ADMIN_USER=admin
      - FILEFLOW_ADMIN_PASSWORD=your_password
      - FILEFLOW_JWT_SECRET=your_jwt_secret
      - FILEFLOW_DATA_DIR=/app/data
      - TZ=Asia/Shanghai
    volumes:
      - data:/app/data

volumes:
  data:
```

## 环境变量

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `FILEFLOW_ADMIN_USER` | 否 | admin | 管理员用户名 |
| `FILEFLOW_ADMIN_PASSWORD` | 是 | - | 管理员密码 |
| `FILEFLOW_JWT_SECRET` | 是 | - | JWT 签名密钥 |
| `FILEFLOW_PORT` | 否 | 8080 | 服务端口 |
| `FILEFLOW_DATA_DIR` | 否 | ./data | 数据存储目录 |
| `FILEFLOW_DATABASE_URL` | 否 | - | 数据库连接 URL |

### 数据库配置

默认使用 SQLite 存储数据。通过 `FILEFLOW_DATABASE_URL` 可配置其他数据库：

| URL 格式 | 数据库类型 |
|----------|------------|
| 空值 / `sqlite:./data/fileflow.db` | SQLite（默认） |
| `libsql://xxx.turso.io?authToken=xxx` | Turso |
| `mysql://user:pass@host:port/db` | MySQL |
| `postgres://user:pass@host:port/db` | PostgreSQL |
| `redis://host:port/db` | Redis |
| `mongodb://host:port/db` | MongoDB |

### 系统设置

以下设置通过 Web 界面「设置 → 系统设置」进行配置，支持热重载：

- **同步间隔** - 账户用量自动同步间隔（分钟），默认 5 分钟
- **端点代理** - 启用反向代理，隐藏 R2 源站地址
- **代理 URL** - 反向代理 URL 前缀

## 反向代理

启用反向代理后，返回的文件 URL 将通过代理服务器转发，隐藏 R2 源站地址。

### 内置代理

在「设置 → 系统设置」中配置：
- 启用「端点代理」开关
- 设置代理 URL，如 `https://your-domain.com/p`

**URL 转换示例：**
- 原始：`https://pub-xxx.r2.dev/path/to/file.png`
- 代理：`https://your-domain.com/p/pub-xxx/path/to/file.png`

### 外置代理

如需独立部署代理服务（边缘加速、减轻主服务负载），可使用 `tools/` 目录下的脚本：

| 文件 | 运行环境 | 说明 |
|------|----------|------|
| `endpoint-proxy-worker.js` | Cloudflare Workers | 推荐，全球边缘加速 |
| `endpoint-proxy-deno.ts` | Deno / Deno Deploy | TypeScript，边缘部署 |
| `endpoint-proxy.go` | Go | 高性能，独立部署 |

详细部署说明请参考 Web 界面「代理部署」页面。

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

详细获取步骤请参考 Web 界面「参数指南」页面。

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

详细文档请参考 Web 界面「API 文档」页面。

## Web 界面

| 页面 | 路径 | 说明 |
|------|------|------|
| 仪表盘 | `/` | 账户概览、用量统计 |
| 文件管理 | `/files` | 文件浏览、上传、删除 |
| 设置 | `/settings` | 账户管理、Token 管理、系统设置 |
| 参数指南 | `/guide` | R2 账户参数获取教程 |
| 代理部署 | `/proxy-guide` | 外置代理部署教程 |
| API 文档 | `/api-docs` | 开放 API 使用说明 |

## 项目结构

```
FileFlow/
├── main.go              # 程序入口
├── server/
│   ├── api/             # HTTP 处理器
│   ├── config/          # 配置管理
│   ├── middleware/      # 中间件
│   ├── service/         # 业务逻辑
│   └── store/           # 数据存储（多数据库后端）
├── client/              # React 前端
│   ├── src/
│   │   ├── components/  # UI 组件
│   │   ├── pages/       # 页面
│   │   └── lib/         # 工具函数
│   └── dist/            # 构建产物
├── tools/               # 外置代理脚本
│   ├── endpoint-proxy-worker.js  # Cloudflare Workers
│   ├── endpoint-proxy-deno.ts    # Deno
│   └── endpoint-proxy.go         # Go
└── docker/              # Docker 配置
```

## License

MIT
