import { useState } from "react";
import { Link } from "react-router-dom";
import { Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const tools = [
  ["fileflow_check", "当前成员、认证方式及有效权限", "认证"],
  ["fileflow_change_password", "修改个人密码；currentPassword、newPassword", "JWT"],
  ["fileflow_logout", "撤销本人所有网页登录会话", "JWT"],
  ["fileflow_list_files", "目录、搜索、扁平浏览；idGroup、prefix、q、flat、cursor、limit", "read"],
  ["fileflow_get_link", "公开文件链接；idGroup、key", "read"],
  ["fileflow_preview_file", "文本、ZIP 索引、预览及下载签名链接；idGroup、key", "read"],
  ["fileflow_upload_content", "小文件中转上传；filename、contentBase64、requestId?，最多 8 MiB", "write"],
  ["fileflow_upload_url", "从公网 URL 下载后中转上传；url、requestId?", "write"],
  ["fileflow_create_directory", "创建目录标记；idGroup、key", "write"],
  ["fileflow_copy_file", "同账号复制文件，拒绝覆盖；idGroup、key、destinationKey", "read + write"],
  ["fileflow_move_file", "同账号移动 / 重命名文件；idGroup、key、destinationKey", "read + write + delete"],
  ["fileflow_delete_file", "删除文件；idGroup、key", "delete"],
  ["fileflow_batch_delete_files", "批量删除，逐项返回结果；accountId、keys", "delete"],
  ["fileflow_set_files_expiration", "设置文件到期时间或永久；accountId、keys、expiresAt", "write"],
  ["fileflow_list_expirations", "查看文件到期计划", "read"],
  ["fileflow_update_expiration", "修改单条到期计划；id、expiresAt", "write"],
  ["fileflow_batch_update_expirations", "批量修改到期计划；ids、expiresAt", "write"],
  ["fileflow_delete_expiration", "立即删除计划对应的文件与记录；id", "delete"],
  ["fileflow_batch_delete_expirations", "立即批量删除计划对应的文件；ids", "delete"],
  ["fileflow_list_accounts", "存储账号列表，默认隐藏 R2 密钥；page、pageSize", "read"],
  ["fileflow_get_account", "存储账号详情，默认隐藏 R2 密钥；id", "read"],
  ["fileflow_get_account_credentials", "读取账号完整配置及 R2 密钥；id", "manage"],
  ["fileflow_account_stats", "账号数量、容量与请求用量统计", "read"],
  ["fileflow_create_account", "添加存储账号；字段沿用 REST 账号配置", "manage"],
  ["fileflow_update_account", "修改存储账号；id 及完整账号配置", "manage"],
  ["fileflow_delete_account", "删除存储账号配置；id", "manage"],
  ["fileflow_sync_accounts", "同步用量；accountId 可选，不填同步全部", "manage"],
  ["fileflow_clear_bucket", "清空账号存储桶；id", "manage"],
  ["fileflow_delete_old_files", "删除指定日期及之前的文件；accountIds、beforeDate（YYYY-MM-DD）", "manage"],
  ["fileflow_get_settings", "读取系统设置", "read"],
  ["fileflow_update_settings", "更新系统设置；传入完整设置对象", "manage"],
  ["fileflow_list_users", "查看团队成员", "manage"],
  ["fileflow_create_user", "创建成员；username、password、role、displayName", "manage"],
  ["fileflow_update_user", "修改成员、角色、启停或重置密码；id、displayName、role、isActive 等", "manage"],
  ["fileflow_list_tokens", "查看有权管理的 API 令牌，隐藏现有令牌值", "manage"],
  ["fileflow_create_token", "创建 API 令牌；name、permissions、userId", "manage"],
  ["fileflow_delete_token", "撤销 API 令牌；id", "manage"],
  ["fileflow_list_webdav_credentials", "查看有权管理的 WebDAV 凭证，隐藏现有密码", "manage"],
  ["fileflow_create_webdav_credential", "创建 WebDAV 凭证；accountId、userId、permissions 等", "manage"],
  ["fileflow_update_webdav_credential", "修改 WebDAV 凭证权限与启停；id、description、permissions、isActive", "manage"],
  ["fileflow_delete_webdav_credential", "撤销 WebDAV 凭证；id", "manage"],
  ["fileflow_audit_logs", "查看最近的团队操作记录；limit 为 1–2000", "manage"],
] as const;

function CodeBlock({ title, code }: { title: string; code: string }) {
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success("已复制");
    } catch {
      toast.error("复制失败，请手动选择代码复制");
    }
  };

  return (
    <div>
      <div className="mb-2 flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium">{title}</h3>
        <Button variant="ghost" size="sm" onClick={copy} aria-label={`复制${title}`}>
          <Copy className="mr-2 h-3.5 w-3.5" />复制
        </Button>
      </div>
      <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs sm:text-sm"><code>{code}</code></pre>
    </div>
  );
}

export default function MCPDocs() {
  const endpoint = `${window.location.origin}/mcp`;
  const [query, setQuery] = useState("");
  const filteredTools = tools.filter((tool) => tool.join(" ").toLowerCase().includes(query.trim().toLowerCase()));
  const headers = `-H "Authorization: Bearer $FILEFLOW_MCP_TOKEN" \\\n  -H "Content-Type: application/json" \\\n  -H "Accept: application/json, text/event-stream"`;

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <div>
        <h1 className="text-3xl font-bold">MCP 使用说明</h1>
        <p className="mt-2 text-muted-foreground">让 Agent 通过 Model Context Protocol 管理团队文件与系统配置。</p>
        <p className="mt-3 text-sm text-muted-foreground">本地大文件可通过配套脚本上传，安装步骤见<Link to="/skill-docs" className="ml-1 text-primary underline underline-offset-4">Skill 使用说明</Link>。</p>
      </div>

      <nav aria-label="MCP 使用说明目录" className="flex flex-wrap gap-x-5 gap-y-3 border-y py-4 text-sm">
        <a href="#mcp-connection" className="text-primary underline underline-offset-4">连接信息</a>
        <a href="#mcp-token" className="text-primary underline underline-offset-4">令牌与权限</a>
        <a href="#mcp-client" className="text-primary underline underline-offset-4">客户端配置</a>
        <a href="#mcp-tools" className="text-primary underline underline-offset-4">工具列表</a>
        <a href="#mcp-examples" className="text-primary underline underline-offset-4">调用示例</a>
        <a href="#mcp-files" className="text-primary underline underline-offset-4">上传与额度</a>
      </nav>

      <section id="mcp-connection" className="scroll-mt-20 space-y-4">
        <h2 className="text-xl font-semibold">连接信息</h2>
        <dl className="divide-y border-y text-sm">
          <div className="grid gap-2 py-3 sm:grid-cols-[140px_1fr]"><dt className="font-medium">端点</dt><dd className="break-all"><code>{endpoint}</code></dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[140px_1fr]"><dt className="font-medium">传输协议</dt><dd>Streamable HTTP，无状态；使用 POST 发送 JSON-RPC，不需要 Session ID。GET / DELETE 返回 405。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[140px_1fr]"><dt className="font-medium">认证</dt><dd>Authorization: Bearer &lt;API Token 或 JWT&gt;，每个请求重新校验成员与权限。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[140px_1fr]"><dt className="font-medium">工具发现</dt><dd>tools/list 只返回当前凭证有权调用的工具，参数类型及限制由 inputSchema 描述。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[140px_1fr]"><dt className="font-medium">操作指南资源</dt><dd>resources/read 读取 <code>fileflow://guide</code>，获取 Agent 上传、分页、配额和结果处理指南。</dd></div>
        </dl>
      </section>

      <section id="mcp-token" className="scroll-mt-20 space-y-4">
        <h2 className="text-xl font-semibold">创建 Agent 令牌</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
          <li>所有者或管理员进入<Link to="/settings" className="text-primary underline underline-offset-4">设置 → 令牌管理</Link>，创建专用令牌并选择所属成员。</li>
          <li>全量操作选择 MCP 管理，将同时勾选读取、写入和删除。MCP 管理仅能授予所有者或管理员，默认不勾选；取消任一文件权限会取消 MCP 管理。</li>
          <li>将令牌放入 Agent 进程的环境变量 <code>FILEFLOW_MCP_TOKEN</code>，然后配置下方端点。修改或撤销令牌后重新连接。</li>
        </ol>
        <p className="text-sm text-muted-foreground">令牌有效权限取授权与成员当前角色的交集。普通成员与只读成员也可连接 MCP；停用成员、撤销令牌立即阻止后续调用。REST 管理接口保持仅接受 JWT，令牌中的 manage 只在 MCP 生效。</p>
        <p className="text-sm text-muted-foreground">管理令牌必须包含 read、write、delete、manage 全部权限。它可以配置系统和创建访问凭证，适合明确授权的管理 Agent；普通文件任务选择相应文件权限即可。</p>
        <p className="text-sm text-muted-foreground">个人密码修改和退出全部网页登录会话需要 JWT；API Token 无法调用这两项。令牌列表与 WebDAV 凭证列表隐藏已有密钥，新建凭证时返回新密钥。</p>
      </section>

      <section id="mcp-client" className="scroll-mt-20 space-y-5">
        <h2 className="text-xl font-semibold">客户端配置</h2>
        <CodeBlock title="Codex · config.toml" code={`[mcp_servers.fileflow]\nurl = "${endpoint}"\nbearer_token_env_var = "FILEFLOW_MCP_TOKEN"`} />
        <p className="text-xs text-muted-foreground">环境变量必须能被启动 Codex 的进程读取。配置字段参考<a href="https://developers.openai.com/codex/mcp/" target="_blank" rel="noreferrer" className="text-primary underline underline-offset-4">官方 OpenAI MCP 文档</a>。</p>
        <CodeBlock title="通用 HTTP 客户端 · JSON 配置示例" code={JSON.stringify({ mcpServers: { fileflow: { type: "http", url: endpoint, headers: { Authorization: "Bearer <YOUR_FILEFLOW_TOKEN>" } } } }, null, 2)} />
        <p className="text-xs text-muted-foreground">不同客户端的配置格式可能不同，选择 Streamable HTTP 并设置 Authorization 请求头。此 JSON 中的令牌为占位符；支持环境变量的客户端应优先使用环境变量。</p>
        <CodeBlock title="环境变量 · PowerShell" code={'$env:FILEFLOW_MCP_TOKEN = "<YOUR_FILEFLOW_TOKEN>"'} />
        <CodeBlock title="环境变量 · bash / zsh" code={'export FILEFLOW_MCP_TOKEN="<YOUR_FILEFLOW_TOKEN>"'} />
      </section>

      <section id="mcp-tools" className="scroll-mt-20 space-y-4">
        <h2 className="text-xl font-semibold">工具列表</h2>
        <p className="text-sm text-muted-foreground">下表展示全部工具；实际可用工具及完整参数以当前凭证的 tools/list 响应为准。对象路径使用 R2 key，账号使用 FileFlow 账号 ID。</p>
        <Input aria-label="搜索 MCP 工具" placeholder="搜索工具名称、操作或权限" value={query} onChange={(event) => setQuery(event.target.value)} />
        <div className="overflow-x-auto border-y">
          <table className="w-full min-w-[690px] text-left text-sm">
            <caption className="sr-only">MCP 工具列表</caption>
            <thead className="border-b bg-muted/40 text-muted-foreground"><tr><th scope="col" className="px-3 py-3">工具</th><th scope="col" className="px-3 py-3">操作与主要参数</th><th scope="col" className="px-3 py-3">权限</th></tr></thead>
            <tbody className="divide-y">{filteredTools.map(([name, description, permission]) => <tr key={name} className="hover:bg-muted/30"><td className="px-3 py-3"><code className="text-xs">{name}</code></td><td className="px-3 py-3 text-muted-foreground">{description}</td><td className="px-3 py-3"><code>{permission}</code></td></tr>)}{filteredTools.length === 0 && <tr><td colSpan={3} className="py-8 text-center text-muted-foreground">没有匹配的工具</td></tr>}</tbody>
          </table>
        </div>
        <p className="text-xs text-muted-foreground">计划删除工具会立即删除对应文件。清空存储桶、删除旧文件和批量删除会修改 R2 数据；工具附带 destructiveHint，Agent 应按用户意图决定是否执行。</p>
      </section>

      <section id="mcp-examples" className="scroll-mt-20 space-y-5">
        <h2 className="text-xl font-semibold">协议调用示例</h2>
        <p className="text-sm text-muted-foreground">以下 curl 命令使用 bash / zsh。正常客户端会自动完成初始化和工具发现；每个请求均携带 Bearer 认证。</p>
        <CodeBlock title="1. 初始化" code={`curl "${endpoint}" \\\n  ${headers} \\\n  --data '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"fileflow-agent","version":"1.0.0"}}}'`} />
        <CodeBlock title="2. 发现工具" code={`curl "${endpoint}" \\\n  ${headers} \\\n  -H "MCP-Protocol-Version: 2025-06-18" \\\n  --data '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'`} />
        <CodeBlock title="3. 搜索文件" code={`curl "${endpoint}" \\\n  ${headers} \\\n  -H "MCP-Protocol-Version: 2025-06-18" \\\n  --data '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"fileflow_list_files","arguments":{"idGroup":"<ACCOUNT_ID>","q":"report","flat":true,"limit":20}}}'`} />
        <CodeBlock title="工具返回示例" code={JSON.stringify({ jsonrpc: "2.0", id: 3, result: { content: [{ type: "text", text: "工具结果的 JSON 文本" }], structuredContent: { status: 200, data: [] }, isError: false } }, null, 2)} />
        <p className="text-sm text-muted-foreground">业务错误通过 isError: true 和 structuredContent.status / data 返回，工具参数校验失败时 status 为 400。未认证请求返回 HTTP 401；JSON-RPC 格式、未知方法或工具名称等错误使用协议错误响应。</p>
      </section>

      <section id="mcp-files" className="scroll-mt-20 space-y-3">
        <h2 className="text-xl font-semibold">上传、预览与请求额度</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
          <li>所有上传均经过 FileFlow 服务。小文件使用 fileflow_upload_content 提交标准 Base64，解码后最多 8 MiB；fileflow_upload_url 接收公网 URL，单文件最大 5 GiB。</li>
          <li>大于 8 MiB 的 Agent 本地文件可使用<Link to="/skill-docs" className="mx-1 text-primary underline underline-offset-4">Skill 配套脚本</Link>，或用同一 Bearer 凭证提交 multipart POST /api/upload，最大 5 GiB。文件由服务器上传到 R2；工具不会读取任意服务器本地路径。</li>
          <li>服务器核验实际字节数、预留容量与请求额度，在写入和对象核验完成后返回结果。请求中断后先通过列表 / 预览核实最终路径，再决定重试。</li>
        </ol>
        <CodeBlock title="大文件 · multipart 上传" code={[
          `curl "${window.location.origin}/api/upload" \\`,
          '  -H "Authorization: Bearer $FILEFLOW_MCP_TOKEN" \\',
          '  -F "file=@/path/to/report.zip" \\',
          '  -F "idGroup=<ACCOUNT_ID>" \\',
          '  -F "path=agent-uploads" \\',
          '  -F "requestId=report-upload-001" \\',
          '  -F "randomFilename=false"',
        ].join("\n")} />
        <p className="text-sm text-muted-foreground">randomFilename 默认为 true，设为 false 保留原文件名并拒绝同名覆盖。expirationDays 不传或为 -1 使用默认设置，0 为永久，正值为有效天数。网页上传无需配置 R2 上传 CORS。</p>
        <p className="text-sm text-muted-foreground">MCP 文件内容 / URL 上传和 multipart 上传支持可选 requestId，最多 128 个字符。同一文件重试复用相同 ID，新上传使用新 ID。服务器绑定当前凭证、参数和内容 SHA-256，已完成的相同请求返回原结果，避免重复 PUT；相同 ID 的内容或参数变化返回 409。</p>
        <p className="text-sm text-muted-foreground">已完成上传的去重信息保留 24 小时。超过窗口后先用文件列表 / 预览核实目标文件，再决定是否使用新 requestId 上传，避免重复文件。</p>
        <p className="text-sm text-muted-foreground">预览工具返回最多 256 KiB 文本内容、ZIP 目录索引以及 url / downloadUrl 签名链接；truncated 为 true 表示文本被截断。图片、PDF、音视频和完整文件由 Agent 使用链接下载。链接约 15 分钟有效，过期后重新调用预览。</p>
        <p className="text-sm text-muted-foreground">复制和移动通过服务器读取源文件、保存到有大小限制的临时文件、条件写入目标并核验。操作使用同一账号中的精确 key，单个对象最多 5 GiB，拒绝覆盖并保留源到期时间。移动需要临时容纳一份副本的容量；目录递归由 Agent 分页列出文件后逐项操作。</p>
        <p className="text-sm text-muted-foreground">复制 / 移动成功时 data 包含 destination、destinationConfirmed、sourceKey、sourceDeleted 等字段；部分失败时恢复信息位于 data.result。destinationConfirmed 为 false 时目标未核验，等待服务器清理并避免盲目重试；为 true 时检查两条 key、到期计划及 sourceDeleted 后再处理。sourceDeleted 已为 true 时，后续错误可能来自旧计划清理。</p>
        <p className="text-sm text-muted-foreground">移动在删除前再次检查源 ETag；R2 不支持条件删除，其他客户端直接改写存储桶时仍可能在核验和删除之间产生冲突，需要协调同一路径的并发写入。</p>
        <p className="text-sm text-muted-foreground">MCP 沿用上传预留、配额核验及请求计数。目录列表/搜索计 Class A；HEAD/GET 和预览计 Class B；修改操作进入团队审计。tools/list 和系统元数据查询不访问 R2。Agent 应按 nextCursor 分页，即使当前页没有匹配结果，也应检查后续游标。</p>
        <p className="text-sm text-muted-foreground">限额覆盖 FileFlow 授权任务及计数。签名地址被外部重复使用、公开域名访问或其他客户端的 R2 操作无法由应用硬性封顶。部署到远程地址时使用 HTTPS；凭证与签名链接应保存在 Agent 的私有配置中。</p>
        <p className="text-sm text-muted-foreground">MCP 请求体上限为 12 MiB。服务接受无 Origin 的原生客户端请求或同站点 Origin；浏览器跨站接入需通过 FILEFLOW_MCP_ALLOWED_ORIGINS 配置精确来源，多个来源用逗号分隔，不能使用通配符。</p>
        <p className="text-sm text-muted-foreground">经本机反向代理接入公开域名时，用 FILEFLOW_MCP_ALLOWED_HOSTS 配置精确 Host（如 files.example.com 或 files.example.com:8443），多个用逗号分隔，不含协议、路径或通配符；保留 DNS 重新绑定保护。</p>
        <Link to="/api-docs" className="inline-block text-sm text-primary underline underline-offset-4">查看 REST 上传参数和配额说明</Link>
      </section>
    </div>
  );
}
