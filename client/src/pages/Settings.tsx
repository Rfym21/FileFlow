import { useState } from "react";
import { Button } from "@/components/ui/button";
import { HardDrive, Key, Settings as SettingsIcon, Server, FolderTree } from "lucide-react";
import AccountsManager from "@/components/AccountsManager";
import TokensManager from "@/components/TokensManager";
import S3CredentialsManager from "@/components/S3CredentialsManager";
import WebDAVCredentialsManager from "@/components/WebDAVCredentialsManager";
import SystemSettings from "@/components/SystemSettings";

type Tab = "accounts" | "tokens" | "s3" | "webdav" | "system";

export default function Settings() {
  const [activeTab, setActiveTab] = useState<Tab>("accounts");

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">设置</h1>
        <p className="text-muted-foreground">管理 R2 账户、API 令牌、S3 凭证、WebDAV 凭证和系统配置</p>
      </div>

      {/* 选项卡 */}
      <div className="flex gap-2 border-b overflow-x-auto">
        <Button
          variant={activeTab === "accounts" ? "default" : "ghost"}
          className="rounded-b-none flex-shrink-0"
          onClick={() => setActiveTab("accounts")}
        >
          <HardDrive className="mr-2 h-4 w-4" />
          账户管理
        </Button>
        <Button
          variant={activeTab === "tokens" ? "default" : "ghost"}
          className="rounded-b-none flex-shrink-0"
          onClick={() => setActiveTab("tokens")}
        >
          <Key className="mr-2 h-4 w-4" />
          令牌管理
        </Button>
        <Button
          variant={activeTab === "s3" ? "default" : "ghost"}
          className="rounded-b-none flex-shrink-0"
          onClick={() => setActiveTab("s3")}
        >
          <Server className="mr-2 h-4 w-4" />
          S3 凭证
        </Button>
        <Button
          variant={activeTab === "webdav" ? "default" : "ghost"}
          className="rounded-b-none flex-shrink-0"
          onClick={() => setActiveTab("webdav")}
        >
          <FolderTree className="mr-2 h-4 w-4" />
          WebDAV 凭证
        </Button>
        <Button
          variant={activeTab === "system" ? "default" : "ghost"}
          className="rounded-b-none flex-shrink-0"
          onClick={() => setActiveTab("system")}
        >
          <SettingsIcon className="mr-2 h-4 w-4" />
          系统设置
        </Button>
      </div>

      {/* 内容 */}
      {activeTab === "accounts" && <AccountsManager />}
      {activeTab === "tokens" && <TokensManager />}
      {activeTab === "s3" && <S3CredentialsManager />}
      {activeTab === "webdav" && <WebDAVCredentialsManager />}
      {activeTab === "system" && <SystemSettings />}
    </div>
  );
}
