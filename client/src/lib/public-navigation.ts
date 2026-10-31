import { BookOpen, Bot, FileCode2, FolderTree, Network, Terminal } from "lucide-react";

export const publicNavigation = [
  {
    id: "guides", title: "使用指南", description: "配置与部署你的文件空间",
    items: [
      { to: "/guide", title: "参数指南", description: "获取 R2 账号与存储桶参数", icon: BookOpen },
      { to: "/proxy-guide", title: "代理部署", description: "配置文件访问代理与公开域名", icon: Network },
    ],
  },
  {
    id: "interfaces", title: "接口文档", description: "连接应用与文件管理工具",
    items: [
      { to: "/api-docs", title: "API 文档", description: "通过 HTTP 上传和管理文件", icon: FileCode2 },
      { to: "/webdav-docs", title: "WebDAV 接口", description: "使用标准文件客户端连接", icon: FolderTree },
    ],
  },
  {
    id: "agents", title: "Agent 接入", description: "让 Agent 参与你的文件工作",
    items: [
      { to: "/mcp-docs", title: "MCP 使用说明", description: "发现工具并操作团队文件", icon: Bot },
      { to: "/skill-docs", title: "Skill 使用说明", description: "安装技能，用脚本上传本地文件", icon: Terminal },
    ],
  },
] as const;
