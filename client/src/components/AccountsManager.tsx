import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  getAccounts,
  createAccount,
  updateAccount,
  deleteAccount,
  clearBucket,
  type Account,
  type AccountRequest,
} from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/utils";
import { Plus, Pencil, Trash2, RefreshCw, X, Eraser } from "lucide-react";
import { Badge } from "@/components/ui/badge";

const defaultAccountForm: AccountRequest = {
  name: "",
  isActive: true,
  description: "",
  accountId: "",
  accessKeyId: "",
  secretAccessKey: "",
  bucketName: "",
  endpoint: "",
  publicDomain: "",
  apiToken: "",
  quota: {
    maxSizeBytes: 10 * 1024 * 1024 * 1024,
    maxClassAOps: 1000000,
  },
};

export default function AccountsManager() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<AccountRequest>(defaultAccountForm);
  const [submitting, setSubmitting] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [clearingId, setClearingId] = useState<string | null>(null);

  const loadAccounts = async () => {
    setLoading(true);
    try {
      const data = await getAccounts();
      setAccounts(data || []);
    } catch (err) {
      console.error("加载账户失败:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    try {
      if (editingId) {
        await updateAccount(editingId, form);
      } else {
        await createAccount(form);
      }
      await loadAccounts();
      handleCancel();
    } catch (err) {
      alert(err instanceof Error ? err.message : "操作失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleActive = async (account: Account) => {
    setTogglingId(account.id);
    try {
      await updateAccount(account.id, {
        name: account.name,
        isActive: !account.isActive,
        description: account.description,
        accountId: account.accountId,
        accessKeyId: "",
        secretAccessKey: "",
        bucketName: account.bucketName,
        endpoint: account.endpoint,
        publicDomain: account.publicDomain,
        apiToken: "",
        quota: account.quota,
      });
      await loadAccounts();
    } catch (err) {
      alert(err instanceof Error ? err.message : "操作失败");
    } finally {
      setTogglingId(null);
    }
  };

  const handleEdit = (account: Account) => {
    setEditingId(account.id);
    setForm({
      name: account.name,
      isActive: account.isActive,
      description: account.description,
      accountId: account.accountId,
      accessKeyId: "",
      secretAccessKey: "",
      bucketName: account.bucketName,
      endpoint: account.endpoint,
      publicDomain: account.publicDomain,
      apiToken: "",
      quota: account.quota,
    });
    setShowForm(true);
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除此账户吗？")) return;
    try {
      await deleteAccount(id);
      await loadAccounts();
    } catch (err) {
      alert(err instanceof Error ? err.message : "删除失败");
    }
  };

  const handleClearBucket = async (id: string) => {
    setClearingId(id);
    try {
      await clearBucket(id);
      await loadAccounts();
    } catch (err) {
      alert(err instanceof Error ? err.message : "清空失败");
    } finally {
      setClearingId(null);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setEditingId(null);
    setForm(defaultAccountForm);
  };

  useEffect(() => {
    loadAccounts();
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <TooltipProvider>
      <div className="space-y-6">
        <div className="flex justify-between">
          <div className="text-sm text-muted-foreground">
            共 {accounts.length} 个账户
          </div>
          <Button onClick={() => setShowForm(true)}>
            <Plus className="mr-2 h-4 w-4" />
            添加账户
          </Button>
        </div>

      {showForm && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>{editingId ? "编辑账户" : "添加账户"}</CardTitle>
              <Button variant="ghost" size="icon" onClick={handleCancel}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label>账户名称 *</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="例如: 主账户"
                />
              </div>
              <div className="space-y-2">
                <Label>Cloudflare Account ID *</Label>
                <Input
                  value={form.accountId}
                  onChange={(e) => setForm({ ...form, accountId: e.target.value })}
                  placeholder="Cloudflare 账户 ID"
                />
              </div>
              <div className="space-y-2">
                <Label>Access Key ID *</Label>
                <Input
                  value={form.accessKeyId}
                  onChange={(e) => setForm({ ...form, accessKeyId: e.target.value })}
                  placeholder="R2 Access Key ID"
                />
              </div>
              <div className="space-y-2">
                <Label>Secret Access Key *</Label>
                <Input
                  type="password"
                  value={form.secretAccessKey}
                  onChange={(e) => setForm({ ...form, secretAccessKey: e.target.value })}
                  placeholder="R2 Secret Access Key"
                />
              </div>
              <div className="space-y-2">
                <Label>Bucket 名称 *</Label>
                <Input
                  value={form.bucketName}
                  onChange={(e) => setForm({ ...form, bucketName: e.target.value })}
                  placeholder="my-bucket"
                />
              </div>
              <div className="space-y-2">
                <Label>Endpoint *</Label>
                <Input
                  value={form.endpoint}
                  onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
                  placeholder="https://xxx.r2.cloudflarestorage.com"
                />
              </div>
              <div className="space-y-2">
                <Label>Public Domain *</Label>
                <Input
                  value={form.publicDomain}
                  onChange={(e) => setForm({ ...form, publicDomain: e.target.value })}
                  placeholder="cdn.example.com"
                />
              </div>
              <div className="space-y-2">
                <Label>API Token (用于查询用量)</Label>
                <Input
                  type="password"
                  value={form.apiToken}
                  onChange={(e) => setForm({ ...form, apiToken: e.target.value })}
                  placeholder="Cloudflare API Token"
                />
              </div>
              <div className="space-y-2">
                <Label>最大容量 (GB)</Label>
                <Input
                  type="number"
                  value={form.quota.maxSizeBytes / 1024 / 1024 / 1024}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      quota: {
                        ...form.quota,
                        maxSizeBytes: parseFloat(e.target.value) * 1024 * 1024 * 1024,
                      },
                    })
                  }
                />
              </div>
              <div className="space-y-2">
                <Label>最大写入操作数</Label>
                <Input
                  type="number"
                  value={form.quota.maxClassAOps}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      quota: {
                        ...form.quota,
                        maxClassAOps: parseInt(e.target.value),
                      },
                    })
                  }
                />
              </div>
            </div>
            <div className="flex gap-2">
              <Button onClick={handleSubmit} disabled={submitting}>
                {submitting ? "保存中..." : "保存"}
              </Button>
              <Button variant="outline" onClick={handleCancel}>
                取消
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="space-y-4">
        {accounts.map((account) => (
          <Card key={account.id} className={!account.isActive ? "opacity-50 border-muted" : ""}>
            <CardContent className="p-4 space-y-3">
              {/* 第一行：名称 + 操作按钮 */}
              <div className="flex items-center justify-between gap-4">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="font-medium truncate">{account.name}</span>
                  {!account.isActive && (
                    <Badge variant="destructive" className="text-xs shrink-0">已禁用</Badge>
                  )}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <div className="flex items-center">
                        <Switch
                          checked={account.isActive}
                          onCheckedChange={() => handleToggleActive(account)}
                          disabled={togglingId === account.id}
                        />
                      </div>
                    </TooltipTrigger>
                    <TooltipContent>
                      {account.isActive ? "停用账户" : "启用账户"}
                    </TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleEdit(account)}
                      >
                        <Pencil className="h-4 w-4" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>编辑账户</TooltipContent>
                  </Tooltip>
                  <AlertDialog>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <AlertDialogTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="text-destructive hover:text-destructive"
                            disabled={clearingId === account.id}
                          >
                            {clearingId === account.id ? (
                              <RefreshCw className="h-4 w-4 animate-spin" />
                            ) : (
                              <Eraser className="h-4 w-4" />
                            )}
                          </Button>
                        </AlertDialogTrigger>
                      </TooltipTrigger>
                      <TooltipContent>清空存储桶</TooltipContent>
                    </Tooltip>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>确认清空存储桶</AlertDialogTitle>
                        <AlertDialogDescription>
                          此操作将删除账户「{account.name}」存储桶中的所有文件，且无法恢复。确定要继续吗？
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>取消</AlertDialogCancel>
                        <AlertDialogAction
                          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                          onClick={() => handleClearBucket(account.id)}
                        >
                          确认清空
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive"
                        onClick={() => handleDelete(account.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>删除账户</TooltipContent>
                  </Tooltip>
                </div>
              </div>
              {/* 第二行：Bucket 和 Endpoint */}
              <div className="text-sm text-muted-foreground truncate">
                {account.bucketName} · {account.endpoint}
              </div>
              {/* 第三行：用量信息 */}
              <div className="text-sm text-muted-foreground">
                容量: {formatBytes(account.usage.sizeBytes)} /{" "}
                {formatBytes(account.quota.maxSizeBytes)} | 写入:{" "}
                {formatNumber(account.usage.classAOps)} /{" "}
                {formatNumber(account.quota.maxClassAOps)}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
      </div>
    </TooltipProvider>
  );
}
