import { Link } from "react-router-dom";
import { Copy, Download } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";

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
        <Button variant="ghost" size="sm" onClick={copy} aria-label={`复制${title}`}><Copy className="mr-2 h-3.5 w-3.5" />复制</Button>
      </div>
      <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs sm:text-sm"><code>{code}</code></pre>
    </div>
  );
}

export default function SkillDocs() {
  const baseUrl = window.location.origin;
  const downloadUrl = `${baseUrl}/assets/fileflow-skill.zip`;
  const windowsScript = '"$env:USERPROFILE\\.codex\\skills\\fileflow\\scripts\\fileflow.py"';
  const unixScript = '"$HOME/.codex/skills/fileflow/scripts/fileflow.py"';

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Skill 使用说明</h1>
        <p className="mt-2 text-muted-foreground">安装 FileFlow skill，让 Agent 使用配套脚本处理本地文件与团队操作。</p>
        <a href={downloadUrl} download="fileflow-skill.zip" className="mt-4 inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"><Download className="h-4 w-4" />下载 FileFlow Skill</a>
      </div>

      <nav aria-label="Skill 使用说明目录" className="flex flex-wrap gap-x-5 gap-y-3 border-y py-4 text-sm">
        <a href="#skill-overview" className="text-primary underline underline-offset-4">能力与限制</a>
        <a href="#skill-install" className="text-primary underline underline-offset-4">安装</a>
        <a href="#skill-auth" className="text-primary underline underline-offset-4">连接与权限</a>
        <a href="#skill-upload" className="text-primary underline underline-offset-4">上传本地文件</a>
        <a href="#skill-operations" className="text-primary underline underline-offset-4">其他操作</a>
      </nav>

      <section id="skill-overview" className="scroll-mt-20 space-y-4">
        <h2 className="text-xl font-semibold">能力与限制</h2>
        <p className="text-sm text-muted-foreground">Skill 包含 SKILL.md、操作参考和 Python 脚本。Agent 可自动识别说明，也可用 <code>$fileflow</code> 显式调用；脚本只依赖 Python 3.10 及以上版本的标准库。</p>
        <div className="overflow-x-auto border-y text-sm">
          <table className="w-full text-left">
            <thead className="border-b bg-muted/40 text-muted-foreground"><tr><th scope="col" className="px-3 py-3">方式</th><th scope="col" className="px-3 py-3">本地文件上传</th><th scope="col" className="px-3 py-3">限制</th></tr></thead>
            <tbody className="divide-y">
              <tr><td className="px-3 py-3">MCP 工具参数</td><td className="px-3 py-3 text-muted-foreground">Base64 放入 JSON</td><td className="px-3 py-3">解码后最多 8 MiB</td></tr>
              <tr><td className="px-3 py-3">Skill 配套脚本</td><td className="px-3 py-3 text-muted-foreground">分块传输 multipart 到服务器</td><td className="px-3 py-3">单文件最多 5 GiB，且符合账号可用容量和请求额度</td></tr>
            </tbody>
          </table>
        </div>
        <p className="text-sm text-muted-foreground">脚本不把文件正文放入 MCP 参数或 Agent 上下文。文件仍经过 FileFlow 服务器上传到 R2，大小核验、容量预留、计数和成员权限保持生效。反向代理也可能配置更小的请求体或超时限制。</p>
        <p className="text-sm text-muted-foreground">Skill 可以单独使用脚本，也可以配合<Link to="/mcp-docs" className="ml-1 text-primary underline underline-offset-4">MCP 接入</Link>：常规操作调用 MCP，大文件上传使用脚本。</p>
      </section>

      <section id="skill-install" className="scroll-mt-20 space-y-5">
        <h2 className="text-xl font-semibold">安装 Skill</h2>
        <p className="text-sm text-muted-foreground">下载包内顶层目录为 <code>fileflow</code>。个人 Codex 安装到 <code>~/.codex/skills/fileflow</code>；仅用于某个项目时放到该项目的 <code>.agents/skills/fileflow</code>。已有同名 skill 时先检查版本，再决定替换，下面的解压命令不会强制覆盖。</p>
        <CodeBlock title="Codex · Windows / PowerShell" code={[
          '$skillArchive = Join-Path $env:TEMP "fileflow-skill.zip"',
          `Invoke-WebRequest -Uri "${downloadUrl}" -OutFile $skillArchive`,
          '$skillRoot = Join-Path $env:USERPROFILE ".codex\\skills"',
          'New-Item -ItemType Directory -Force -Path $skillRoot | Out-Null',
          'Expand-Archive -LiteralPath $skillArchive -DestinationPath $skillRoot',
        ].join("\n")} />
        <CodeBlock title="Codex · macOS / Linux" code={[
          `curl -fSL "${downloadUrl}" -o fileflow-skill.zip`,
          'mkdir -p "$HOME/.codex/skills"',
          'unzip -n fileflow-skill.zip -d "$HOME/.codex/skills"',
        ].join("\n")} />
        <p className="text-sm text-muted-foreground">安装后让 Agent 重新发现 skill；若当前客户端尚未加载，重新打开会话。其他支持 SKILL.md 的 Agent 按其技能目录规则安装；是否自动发现取决于客户端。下载包不包含服务地址、成员密码或令牌。</p>
      </section>

      <section id="skill-auth" className="scroll-mt-20 space-y-5">
        <h2 className="text-xl font-semibold">连接与权限</h2>
        <p className="text-sm text-muted-foreground">在<Link to="/settings" className="mx-1 text-primary underline underline-offset-4">设置 → 令牌管理</Link>创建专用令牌。仅上传需要 write；浏览需要 read；删除需要 delete。全量管理需要所属成员为所有者或管理员，并授予 read、write、delete、manage。令牌和当前成员角色的权限交集决定实际操作范围。</p>
        <CodeBlock title="环境变量 · PowerShell" code={[
          `$env:FILEFLOW_URL = "${baseUrl}"`,
          '$env:FILEFLOW_TOKEN = "<YOUR_FILEFLOW_TOKEN>"',
          `python ${windowsScript} tools`,
        ].join("\n")} />
        <CodeBlock title="环境变量 · bash / zsh" code={[
          `export FILEFLOW_URL="${baseUrl}"`,
          'export FILEFLOW_TOKEN="<YOUR_FILEFLOW_TOKEN>"',
          `python3 ${unixScript} tools`,
        ].join("\n")} />
        <p className="text-sm text-muted-foreground"><code>FILEFLOW_URL</code> 是服务根地址，不带 <code>/mcp</code>。脚本优先读取 FILEFLOW_TOKEN，也兼容 FILEFLOW_MCP_TOKEN。环境变量必须能被 Agent 进程读取；令牌不作为命令行参数传入。远程连接使用 HTTPS。</p>
        <p className="text-sm text-muted-foreground">Skill 是操作说明和客户端脚本，不会授予额外权限。管理操作仍通过 MCP 校验；个人密码修改和撤销网页登录会话需要 JWT。</p>
      </section>

      <section id="skill-upload" className="scroll-mt-20 space-y-5">
        <h2 className="text-xl font-semibold">上传本地文件</h2>
        <CodeBlock title="上传压缩包 · PowerShell" code={`python ${windowsScript} upload "C:\\files\\report.zip" --account "<ACCOUNT_ID>" --path "documents" --keep-name --expiration-days 0 --request-id "report-upload-001"`} />
        <CodeBlock title="上传压缩包 · bash / zsh" code={`python3 ${unixScript} upload "/path/to/report.zip" --account "<ACCOUNT_ID>" --path "documents" --keep-name --expiration-days 0 --request-id "report-upload-001"`} />
        <dl className="divide-y border-y text-sm">
          <div className="grid gap-2 py-3 sm:grid-cols-[170px_1fr]"><dt className="font-medium">--account</dt><dd>FileFlow 存储账号 ID；省略时由服务器选择可用账号。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[170px_1fr]"><dt className="font-medium">--path</dt><dd>目标目录前缀；省略使用服务器默认路径。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[170px_1fr]"><dt className="font-medium">--keep-name</dt><dd>保留原文件名，拒绝覆盖现有对象；省略时随机命名。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[170px_1fr]"><dt className="font-medium">--expiration-days</dt><dd>-1 或省略使用系统默认，0 永久，1–3650 为指定天数。</dd></div>
          <div className="grid gap-2 py-3 sm:grid-cols-[170px_1fr]"><dt className="font-medium">--request-id</dt><dd>同一文件重试复用该 ID；省略自动生成 UUID。脚本会在结果中返回 requestId。</dd></div>
        </dl>
        <p className="text-sm text-muted-foreground">可以告诉 Agent：「使用 $fileflow 把这个压缩包上传到指定账号，保留文件名，永久保存」。脚本按块读取本地文件，通过 POST /api/upload 传输，不要求单独配置 MCP Server，也不需要 R2 上传 CORS。</p>
        <p className="text-sm text-muted-foreground">脚本不自动重试。请求失败或中断时保留返回的 requestId，先核实结果，再使用相同文件、参数和 ID 重试。服务器绑定身份、参数和内容 SHA-256；已完成上传的去重信息保留 24 小时。超过窗口后先检查文件列表，再决定发起新上传。</p>
        <p className="text-sm text-muted-foreground">成功输出包含账号、最终 key、文件大小和链接。HTTP 错误、MCP 业务错误或批量部分失败会返回非零退出码，Agent 应检查 JSON 结果。</p>
      </section>

      <section id="skill-operations" className="scroll-mt-20 space-y-5">
        <h2 className="text-xl font-semibold">工具发现与其他操作</h2>
        <CodeBlock title="发现当前凭证有权调用的工具" code={`python3 ${unixScript} tools`} />
        <CodeBlock title="搜索文件 · bash / zsh" code={`python3 ${unixScript} call fileflow_list_files --args '{"idGroup":"<ACCOUNT_ID>","flat":true,"q":"report","limit":20}'`} />
        <CodeBlock title="使用参数文件 · 跨平台" code={'python <SKILL_DIR>/scripts/fileflow.py call fileflow_list_files --args-file search.json'} />
        <CodeBlock title="search.json" code={JSON.stringify({ idGroup: "<ACCOUNT_ID>", flat: true, q: "report", limit: 20 }, null, 2)} />
        <p className="text-sm text-muted-foreground">call 接受当前凭证可用的所有 MCP 工具，完整工具表和参数见<Link to="/mcp-docs" className="ml-1 text-primary underline underline-offset-4">MCP 使用说明</Link>。按每个账号的 nextCursor 继续分页，空结果页也可能有后续游标。</p>
        <p className="text-sm text-muted-foreground">默认 JSON 输出隐藏凭证值。创建凭证后需要保留完整结果时，添加 <code>--output new-result.json</code> 保存到新的私有文件；脚本拒绝覆盖已有文件。只有明确需要输出密钥时使用 <code>--show-secrets</code>。</p>
      </section>
    </div>
  );
}
