import { useState } from "react";
import { Button } from "@/components/ui/button";
import { HardDrive, Key } from "lucide-react";
import AccountsManager from "@/components/AccountsManager";
import TokensManager from "@/components/TokensManager";

type Tab = "accounts" | "tokens";

export default function Settings() {
  const [activeTab, setActiveTab] = useState<Tab>("accounts");

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">设置</h1>
        <p className="text-muted-foreground">管理 R2 账户和 API Token</p>
      </div>

      {/* 选项卡 */}
      <div className="flex gap-2 border-b">
        <Button
          variant={activeTab === "accounts" ? "default" : "ghost"}
          className="rounded-b-none"
          onClick={() => setActiveTab("accounts")}
        >
          <HardDrive className="mr-2 h-4 w-4" />
          账户管理
        </Button>
        <Button
          variant={activeTab === "tokens" ? "default" : "ghost"}
          className="rounded-b-none"
          onClick={() => setActiveTab("tokens")}
        >
          <Key className="mr-2 h-4 w-4" />
          Token 管理
        </Button>
      </div>

      {/* 内容 */}
      {activeTab === "accounts" ? <AccountsManager /> : <TokensManager />}
    </div>
  );
}
