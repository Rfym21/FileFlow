import { Link } from "react-router-dom";
import { ArrowRight, Bot, Files, HardDrive, Search, ShieldCheck, Terminal } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";

const features = [
  { icon: Files, title: "团队共享文件", description: "一个部署服务一个团队。成员独立登录，共享文件，并按只读、读写和管理角色分配权限。" },
  { icon: HardDrive, title: "统一管理 R2 账号", description: "集中管理多个存储账号，跟踪容量和请求用量，上传前核验配额并为并发任务预留容量。" },
  { icon: Search, title: "浏览、预览与批量管理", description: "搜索完整文件路径，扁平化浏览跨目录文件，预览常见媒体、文本和 ZIP，批量删除或修改到期时间。" },
  { icon: Bot, title: "让 Agent 参与文件工作", description: "MCP 工具覆盖文件、团队、账号、设置和凭证操作，沿用成员权限与审计记录。" },
  { icon: Terminal, title: "脚本上传本地大文件", description: "安装 FileFlow skill，使用配套 Python 脚本流式上传本地文件；无需把文件正文放进 MCP 参数。" },
  { icon: ShieldCheck, title: "在自己的服务中运行", description: "服务器中转上传、可撤销访问令牌、文件到期管理和操作记录，让团队统一管理文件访问。" },
];

const documents = [
  { path: "/guide", title: "参数指南", description: "获取 Cloudflare R2 的账号参数和存储配置。" },
  { path: "/proxy-guide", title: "代理部署", description: "配置内置代理，或部署独立的文件访问代理。" },
  { path: "/api-docs", title: "API 文档", description: "通过 HTTP 接口上传、浏览和管理文件。" },
  { path: "/webdav-docs", title: "WebDAV 接口", description: "连接文件管理器和标准 WebDAV 客户端。" },
  { path: "/mcp-docs", title: "MCP 使用说明", description: "为 Agent 配置 MCP，查看权限与完整工具列表。" },
  { path: "/skill-docs", title: "Skill 使用说明", description: "下载 skill，安装脚本并上传本地大文件。" },
];

export default function Home() {
  const { isAuthenticated } = useAuth();

  return (
    <div className="mx-auto max-w-5xl space-y-16 py-6 sm:py-12">
      <section className="max-w-3xl space-y-6">
        <p className="text-sm font-medium text-primary">团队文件 · 自托管 · Agent 接入</p>
        <h1 className="text-4xl font-semibold leading-tight tracking-tight sm:text-5xl">让团队与 Agent<br className="hidden sm:block" />在同一个文件空间协作。</h1>
        <p className="max-w-2xl text-lg leading-relaxed text-muted-foreground">FileFlow 将多个 Cloudflare R2 账号连接到一个团队文件入口。浏览、上传、预览和管理文件，也可以通过 MCP 或 Skill 交给 Agent 完成。</p>
        <div className="flex flex-wrap items-center gap-3">
          <Link to={isAuthenticated ? "/dashboard" : "/login"} className="inline-flex items-center gap-2 rounded-md bg-primary px-5 py-3 text-sm font-medium text-primary-foreground hover:bg-primary/90">{isAuthenticated ? "进入工作台" : "登录团队"}<ArrowRight className="h-4 w-4" /></Link>
          <a href="#home-documents" className="inline-flex items-center gap-2 rounded-md border px-5 py-3 text-sm font-medium hover:bg-accent">查看使用文档<ArrowRight className="h-4 w-4" /></a>
        </div>
      </section>

      <section className="space-y-5">
        <h2 className="text-2xl font-semibold">从文件上传到团队管理</h2>
        <dl className="divide-y border-y">
          {features.map(({ icon: Icon, title, description }) => <div key={title} className="grid gap-3 py-5 sm:grid-cols-[250px_1fr]"><dt className="flex items-center gap-3 font-medium"><Icon className="h-5 w-5 shrink-0 text-primary" />{title}</dt><dd className="text-sm leading-relaxed text-muted-foreground">{description}</dd></div>)}
        </dl>
      </section>

      <section id="home-documents" className="scroll-mt-24 space-y-5">
        <div><h2 className="text-2xl font-semibold">使用文档</h2><p className="mt-2 text-sm text-muted-foreground">无需登录即可查看配置、接口和 Agent 接入说明。</p></div>
        <ul className="divide-y border-y">
          {documents.map(({ path, title, description }) => <li key={path}><Link to={path} className="group flex items-center justify-between gap-4 py-5"><div><h3 className="font-medium group-hover:text-primary">{title}</h3><p className="mt-1 text-sm text-muted-foreground">{description}</p></div><ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground group-hover:text-primary" /></Link></li>)}
        </ul>
      </section>
    </div>
  );
}
