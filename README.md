# FileFlow

FileFlow 是一个自托管的团队文件管理应用，提供统一的 Web 界面管理多个 Cloudflare R2 存储账户，支持成员独立登录、文件上传、下载、删除，以及账户配额和用量追踪。一个部署服务一个团队，团队成员共享文件，并按角色区分操作权限。

## 功能特点

- **团队成员** - 所有者、管理员、可读写成员和只读成员，支持添加成员、修改角色、停用及重置密码
- **操作记录** - 记录登录、上传、删除、修改成员及设置等操作，保留最近 2000 条记录供管理员查看
- **多账户管理** - 添加、编辑、删除多个 R2 存储账户，支持配额限制和启用/禁用控制
- **文件操作** - 懒加载目录浏览、拖拽上传、删除文件，支持生成公开访问链接
- **服务器上传** - 文件和公网 URL 均经过服务器上传到 R2，支持保留原文件名并拒绝同名覆盖
- **搜索与扁平浏览** - 按文件名或完整路径搜索，跨目录表格浏览、批量删除及修改到期时间
- **文件预览** - 支持图片、音视频、PDF、文本预览，以及 ZIP 目录分级浏览
- **智能上传** - 自动选择用量最低的账户进行上传，超额时自动切换备用账户，支持从公网 URL 下载后上传
- **文件到期管理** - 支持设置文件有效期、批量修改到期时间和批量删除，自动删除过期文件
- **用量同步** - 可配置的自动同步间隔（默认 5 分钟），支持热重载
- **清空存储桶** - 一键清空指定账户的所有文件
- **反向代理** - 内置反向代理 + 外置代理脚本（Workers/Deno/Go），隐藏 R2 源站地址
- **多数据库支持** - 支持 SQLite、MySQL、PostgreSQL、Redis、MongoDB、Turso
- **API Token** - 支持生成可撤销的 API Token，用于程序化访问
- **WebDAV 接口** - 标准 WebDAV 协议，支持 Windows/macOS/Linux 文件管理器、Cyberduck 等客户端
- **开放 API** - 完整的 RESTful API，支持文件列表、上传、删除、获取链接
- **MCP 接口** - Agent 通过 Streamable HTTP 发现并调用文件、到期计划、团队、账号、设置和凭证管理工具，沿用成员权限、配额和审计
- **Skill 接入** - 可下载的 FileFlow skill 与 Python 脚本，支持服务器分块上传本地文件，单文件最大 5 GiB
- **内置指南** - Web 界面内置参数获取指南、代理部署教程、API 文档

## 快速开始

### 环境要求

- Go 1.25+
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

### 团队使用

首次启动时，`FILEFLOW_ADMIN_USER` 和 `FILEFLOW_ADMIN_PASSWORD` 用于创建团队所有者；从个人版本升级时，已有存储账号、文件到期记录、API Token 和 WebDAV 凭证会保留，现有凭证归属所有者。升级后需要重新登录。

成员及密码保存在数据库中，成员密码使用 bcrypt 哈希。初始化后，修改环境变量中的管理员密码不会覆盖数据库中的密码；请通过个人信息弹窗修改自己的密码，或由有权管理该成员的管理员重置密码。后续启动可以不配置初始化密码，但仍须保留 `FILEFLOW_JWT_SECRET`。

| 角色 | 文件权限 | 管理权限 |
| --- | --- | --- |
| 所有者 | 浏览、预览、下载、上传、删除、修改到期时间 | 管理存储账号、系统设置、全部成员及访问凭证；所有者不可停用或改为其他角色 |
| 管理员 | 与所有者相同 | 管理存储账号、系统设置、普通成员，以及自身与普通成员的访问凭证 |
| 可读写成员 | 浏览、预览、下载、上传、删除、修改到期时间 | 无系统管理权限 |
| 只读成员 | 浏览、预览、下载 | 无系统管理权限 |

在“团队成员”页面添加成员并设置初始密码；创建 API Token 或 WebDAV 凭证时选择归属成员，其实际访问权限为凭证权限与成员当前权限的交集。停用成员后，该成员的登录及所属凭证无法继续访问；修改角色或密码会要求该成员重新登录。退出登录会结束该成员的全部网页登录会话，网页登录会话最长有效期为 24 小时。

所有成员共享现有 R2 文件，第一版不区分项目空间或个人目录。受限文件应放在私有 R2 存储桶中；公开 R2 域名和公开代理地址仍可由持有地址的人访问。预览生成的临时签名链接最多有效 15 分钟，成员停用或权限变更后，已签发的链接仍可使用至其过期。

当前版本适合单个服务实例。成员与权限迁移覆盖现有数据库后端，但运行多个实例需要额外的数据库并发写入及后台任务协调。

### 上传与配额

`POST /api/upload` 接收文件或公网 URL，统一由 FileFlow 服务上传到 R2。单文件最大 5 GiB，服务器校验实际文件字节数，并核验对象后保存容量与到期记录。保留原文件名时使用条件写入拒绝覆盖现有对象。完整参数和示例见网页 `/api-docs`。

后端原子预留并发操作容量与核验读取次数，并在授权写入时计入不可退回的上传次数。账号可配置 Class A 和 Class B 限额，Class B 为 0 时保持旧配置的不限读取行为。目录列表和搜索计 Class A，HEAD/GET 计 Class B；未完成操作只有经清理核验后才释放容量，取消客户端请求不会立即释放预留。

此限额控制 FileFlow 授权的操作与请求。预览/下载签名 URL 的外部重复使用、公开域名访问和其他客户端操作无法由应用硬性封顶；远程用量同步作为保守下限，仍需监控 R2 实际用量。月份以 UTC 计算。网页上传无需配置 R2 上传 CORS。

### 编译构建

```bash
# 构建前端
cd client && npm ci && npm run build && cd ..

# 构建后端（前端会嵌入到二进制文件中）
go build -o fileflow .
```

### 自动发布

更新 `.version` 中的版本号并推送到 `main`，或在 GitHub Actions 中手动运行 `Build and Release`，即可构建并发布。版本号支持 `1.11.3`、`1.12.0-rc.1` 等 SemVer 格式；同一版本只能对应同一提交。

发布流程构建一次前端，为 Linux、macOS、Windows 的 amd64 和 arm64 架构生成压缩包，并复用 Linux 二进制发布双架构 GHCR 镜像。压缩包命名如 `file-flow_1.11.3_linux_amd64.tar.gz`，Windows 使用 `.zip`；包内附带 `.version`、本说明和 `.env.example`。下载后可通过 `sha256sum --ignore-missing -c checksums.txt` 校验文件。

镜像提供版本号、`v` 前缀版本号及提交 SHA 标签，仅默认分支的正式版本更新 `latest`。GitHub Release 在镜像发布成功后创建；预发布版本会标记为 prerelease。

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
| `FILEFLOW_MCP_ALLOWED_ORIGINS` | 否 | - | MCP 浏览器客户端允许的精确 Origin，多个用逗号分隔；默认仅接受同站点 Origin 或无 Origin 的原生客户端 |
| `FILEFLOW_MCP_ALLOWED_HOSTS` | 否 | - | 经本机反向代理接入 MCP 时允许的公开 Host（可含端口），多个用逗号分隔；未配置时保留 SDK 的 DNS 重新绑定保护 |

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
- **默认文件到期时间** - 文件默认有效期（天），0 表示永久，默认 30 天
- **到期检查间隔** - 自动检查并删除过期文件的间隔（分钟），默认 720 分钟（12 小时）

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

FileFlow 提供 RESTful API 供外部应用调用，使用 `Authorization: Bearer ...` 认证。文件接口支持 API Token 或成员登录获得的 JWT；账号、到期计划和系统管理等网页接口要求 JWT，管理操作还要求当前成员具有 `manage` 权限。

### 认证方式

```bash
curl -H "Authorization: Bearer your-api-token" https://your-domain/api/files
```

### API 端点

| 方法 | 路径 | 权限 | 认证 | 说明 |
| --- | --- | --- | --- | --- |
| GET | `/api/files` | read | JWT / Token | 目录浏览、搜索、扁平浏览和分页 |
| POST | `/api/upload` | write | JWT / Token | 文件或公网 URL 经服务器中转上传 |
| GET | `/api/link` | read | JWT / Token | 获取文件公开链接 |
| GET | `/api/file-preview` | read | JWT / Token | 文本、ZIP 索引和预览/下载签名链接 |
| DELETE | `/api/file` | delete | JWT / Token | 删除单个文件 |
| POST | `/api/files/batch-delete` | delete | JWT / Token | 按 key 批量删除，逐项返回结果 |
| PATCH | `/api/files/expirations` | write | JWT | 为文件设置到期时间或永久 |
| GET | `/api/file-expirations` | read | JWT | 查看到期计划 |
| PATCH | `/api/file-expirations` | write | JWT | 按计划 ID 批量修改到期时间 |
| PATCH | `/api/file-expirations/:id` | write | JWT | 修改单条到期计划 |
| POST | `/api/file-expirations/batch-delete` | delete | JWT | 立即批量删除计划对应的文件 |
| DELETE | `/api/file-expirations/:id` | delete | JWT | 立即删除计划对应的文件 |
| GET | `/api/accounts`、`/api/accounts/:id`、`/api/accounts/stats` | read | JWT | 账号列表、详情和统计 |
| GET | `/api/settings` | read | JWT | 读取系统设置 |

团队成员、存储账号、API Token、WebDAV 凭证、系统设置修改和审计接口均要求 JWT 与 `manage`。长期管理 Agent 可使用下述 MCP 管理令牌接入 `/mcp`。

### 请求参数

**GET /api/files**

- `idGroup` - 账户 ID（逗号分隔多个）
- `prefix` - 目录前缀
- `cursor` - 分页游标
- `limit` - 每页数量（默认 50，最大 100）
- `q` - 不区分大小写的完整文件路径搜索；非空时启用扁平浏览
- `flat` - `true` 表示跨目录扁平浏览

搜索和扁平浏览初始请求最多查询 20 个账号，游标续页必须指定单个账号。每个账号最多读取一个 LIST 批次；当前页没有匹配文件也可能有 `nextCursor`，应继续检查后续游标。

**POST /api/upload**（multipart/form-data）

- `file` - 上传的文件（与 url 二选一）
- `url` - 远程文件 URL，从该地址下载后上传（与 file 二选一）
- `path` - 自定义存储路径
- `idGroup` - 指定账户 ID
- `expirationDays` - 文件有效期（天），不填或 -1=使用系统默认，0=永久，>0=指定天数
- `randomFilename` - 默认 `true` 随机命名；设为 `false` 保留原文件名
- `requestId` - 可选客户端上传请求 ID，最多 128 个字符；同一文件上传重试时复用，新上传使用新 ID

默认文件名为 `{uuid}_{timestamp}.{ext}`；保留原文件名时拒绝同名覆盖。上传经过服务器，沿用容量预留、请求计数及到期规则。单文件最大 5 GiB。

传入 `requestId` 时，服务器将请求绑定到当前凭证、上传参数和文件内容 SHA-256。已完成上传的相同请求重试返回原结果，避免重复 PUT；相同 ID 使用不同内容或参数返回 409。MCP 的文件内容和 URL 上传工具也支持此参数。

已完成上传的去重信息保留 24 小时。超过此窗口后，先检查目标文件是否已上传，再决定是否使用新的请求 ID 上传，避免重复文件。

**GET /api/link** / **GET /api/file-preview** / **DELETE /api/file**

- `idGroup` - 账户 ID（必填）
- `key` - 文件路径（必填）

**POST /api/files/batch-delete** 使用 JSON `{"accountId":"...","keys":["path/file.txt"]}`，每批最多 100 个 key。**PATCH /api/files/expirations** 在相同字段上增加 `expiresAt`，使用未来 RFC3339 时间，或 `null` 表示永久。

详细请求、响应和额度说明请参考 Web 界面 `/api-docs`。

## MCP 接口

FileFlow 内置 Model Context Protocol 服务，端点为 `/mcp`，使用无状态 Streamable HTTP 和 Bearer 认证。Agent 可通过 `tools/list` 发现当前凭证有权使用的工具，并通过 `tools/call` 执行文件浏览、搜索、上传、预览、下载链接、批量删除、到期计划，以及团队、存储账号、系统设置、API Token、WebDAV 凭证和审计管理。

所有者或管理员在「设置 → 令牌管理」创建专用令牌。选择 **MCP 管理** 会同时授予 `read`、`write`、`delete`、`manage`，允许 Agent 全量操作；仅处理文件的 Agent 可选择相应文件权限。管理权限只在 MCP 生效，REST 管理接口仍仅接受 JWT。成员停用、角色变更和令牌撤销在后续请求中立即生效。

例如，在 Agent 进程的环境变量中设置 `FILEFLOW_MCP_TOKEN`，并为 Codex 配置：

```toml
[mcp_servers.fileflow]
url = "https://your-domain/mcp"
bearer_token_env_var = "FILEFLOW_MCP_TOKEN"
```

MCP 支持 Base64 文件上传（解码后最多 8 MiB）和公网 URL 上传（单文件最多 5 GiB），均经过服务器。大于 8 MiB 的 Agent 本地文件使用 multipart `POST /api/upload`，最多 5 GiB。复制/移动通过服务器读取、临时保存并条件写入目标；工具不会读取任意服务器本地路径。上传容量预留、Class A/B 计数和审计规则与现有接口一致。

完整配置、工具表、协议示例和额度说明见 [MCP 接入文档](docs/mcp.md)，或 Web 界面的 `/mcp-docs`。Codex 配置字段见[官方 OpenAI 文档](https://developers.openai.com/codex/mcp/)。

## Skill 接入

网页 `/skill-docs` 提供独立使用说明和 FileFlow skill 下载；构建包位于 `/assets/fileflow-skill.zip`，也提供公开接口 `GET /api/skill/download`。也可使用仓库中的 `skills/fileflow`。将整个 `fileflow` 目录安装到 `~/.codex/skills/fileflow`，或项目的 `.agents/skills/fileflow`，供 Agent 发现 `SKILL.md`。

配套脚本仅依赖 Python 3.10 及以上版本标准库。设置 `FILEFLOW_URL` 为服务根地址，`FILEFLOW_TOKEN` 为专用 API Token，然后执行：

```bash
python3 ~/.codex/skills/fileflow/scripts/fileflow.py tools
python3 ~/.codex/skills/fileflow/scripts/fileflow.py upload /path/to/report.zip --account "<ACCOUNT_ID>" --path documents --keep-name --expiration-days 0 --request-id report-upload-001
```

脚本按块读取本地文件，直接提交 FileFlow 的 multipart 上传接口，不把文件放进 MCP JSON 或 Agent 上下文，因此可上传大于 8 MiB 的文件。单文件仍不得超过 5 GiB，且必须满足账号可用容量和请求额度；上传经过服务器。其他文件和管理操作可用脚本 `call` 调用相应 MCP 工具，权限沿用专用令牌及所属成员当前角色。

脚本不自动重试，结果包含 `requestId`；核实结果后，在 24 小时去重窗口内用相同文件、参数和 ID 重试。完整安装、命令与输出说明见 [Skill 使用说明](docs/skill.md)。下载包不包含任何部署凭证。

## WebDAV 接口

FileFlow 提供标准 WebDAV 协议支持，可使用各类 WebDAV 客户端直接访问。

### 端点信息

| 项目 | 值 |
|------|-----|
| 端点 URL | `http://your-domain:8080/webdav` |
| 认证方式 | HTTP Basic Auth |

### 创建 WebDAV 凭证

1. 进入「设置 → WebDAV 凭证」页面
2. 点击「创建凭证」，选择关联账户和权限
3. 保存生成的用户名和密码

### 客户端配置示例

**Windows 文件资源管理器：**

1. 右键「此电脑」→「映射网络驱动器」
2. 输入地址：`http://your-domain:8080/webdav`
3. 输入用户名和密码

**macOS Finder：**

1. 按 Command + K（或菜单「前往 → 连接服务器」）
2. 输入地址：`http://your-domain:8080/webdav`
3. 点击「连接」，输入用户名和密码

**Linux（Nautilus）：**

```bash
# 在文件管理器地址栏输入
dav://your-domain:8080/webdav
```

**Cyberduck：**

1. 新建连接，选择「WebDAV (HTTP)」或「WebDAV (HTTPS)」
2. 服务器：`your-domain:8080`
3. 路径：`/webdav`
4. 输入用户名和密码

**命令行（curl）：**

```bash
# 列出文件
curl -u username:password "http://localhost:8080/webdav/" -X PROPFIND

# 上传文件
curl -u username:password -T file.txt "http://localhost:8080/webdav/path/file.txt"

# 下载文件
curl -u username:password "http://localhost:8080/webdav/path/file.txt" -o file.txt

# 删除文件
curl -u username:password -X DELETE "http://localhost:8080/webdav/path/file.txt"
```

### 支持的 WebDAV 方法

| 方法 | 描述 |
|------|------|
| PROPFIND | 列出文件和目录 |
| GET | 下载文件 |
| PUT | 上传文件 |
| DELETE | 删除文件/目录 |
| MKCOL | 创建目录 |
| COPY | 复制文件/目录 |
| MOVE | 移动/重命名文件 |
| LOCK/UNLOCK | 锁定/解锁（兼容性实现） |

详细文档请参考 Web 界面「WebDAV 接口」页面。

## Web 界面

首页及参数指南、代理部署、API、WebDAV、MCP、Skill 使用说明均可免登录查看。仪表盘与文件管理需要登录，团队成员和设置还需管理权限。

| 页面 | 路径 | 说明 |
|------|------|------|
| 首页 | `/` | 产品介绍、公开文档与登录入口 |
| 仪表盘 | `/dashboard` | 账户概览、用量统计，需登录 |
| 文件管理 | `/files` | 文件浏览、上传、删除 |
| 计划删除 | `/scheduled-deletions` | 查看和管理计划删除的文件 |
| 设置 | `/settings` | 账户管理、令牌管理、WebDAV 凭证、系统设置 |
| 参数指南 | `/guide` | R2 账户参数获取教程 |
| 代理部署 | `/proxy-guide` | 外置代理部署教程 |
| API 文档 | `/api-docs` | 开放 API 使用说明 |
| WebDAV 接口 | `/webdav-docs` | WebDAV 接口使用说明 |
| MCP 使用说明 | `/mcp-docs` | 独立页面，包含 Agent 连接配置、权限、完整工具表及协议示例 |
| Skill 使用说明 | `/skill-docs` | Skill 下载、安装、Python 脚本与大文件上传说明 |

## 项目结构

```
FileFlow/
├── main.go              # 程序入口
├── server/
│   ├── api/             # HTTP 处理器
│   ├── config/          # 配置管理
│   ├── middleware/      # 中间件
│   ├── webdav/          # WebDAV 协议实现
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
