import { useEffect, useState, useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Pagination } from "@/components/ui/pagination";
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
  type AccountRequest,
  type AccountFull,
} from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/utils";
import { Plus, Pencil, Trash2, RefreshCw, X, Eraser, Copy, Check } from "lucide-react";
import { Badge } from "@/components/ui/badge";

const PAGE_SIZE = 5;

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
  const [accounts, setAccounts] = useState<AccountFull[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<AccountRequest>(defaultAccountForm);
  const [submitting, setSubmitting] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const [clearingId, setClearingId] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);

  // 分页计算
  const totalPages = Math.ceil(accounts.length / PAGE_SIZE);
  const paginatedAccounts = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return accounts.slice(start, start + PAGE_SIZE);
  }, [accounts, currentPage]);

  // 当账户数据变化时重置页码
  useEffect(() => {
    if (currentPage > totalPages && totalPages > 0) {
      setCurrentPage(totalPages);
    }
  }, [accounts.length, totalPages, currentPage]);

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
      let result: AccountFull;
      if (editingId) {
        result = await updateAccount(editingId, form);
      } else {
        result = await createAccount(form);
      }

      // 更新表单，填充返回的完整字段（包括敏感字段）
      setForm({
        name: result.name,
        isActive: result.isActive,
        description: result.description,
        accountId: result.accountId,
        accessKeyId: result.accessKeyId,
        secretAccessKey: result.secretAccessKey,
        bucketName: result.bucketName,
        endpoint: result.endpoint,
        publicDomain: result.publicDomain,
        apiToken: result.apiToken,
        quota: result.quota,
      });

      await loadAccounts();
      alert(editingId ? "账户更新成功！所有字段已刷新。" : "账户创建成功！请妥善保管显示的凭证信息。");
    } catch (err) {
      alert(err instanceof Error ? err.message : "操作失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleActive = async (account: AccountFull) => {
    setTogglingId(account.id);
    try {
      await updateAccount(account.id, {
        name: account.name,
        isActive: !account.isActive,
        description: account.description,
        accountId: account.accountId,
        accessKeyId: account.accessKeyId,
        secretAccessKey: account.secretAccessKey,
        bucketName: account.bucketName,
        endpoint: account.endpoint,
        publicDomain: account.publicDomain,
        apiToken: account.apiToken,
        quota: account.quota,
      });
      await loadAccounts();
    } catch (err) {
      alert(err instanceof Error ? err.message : "操作失败");
    } finally {
      setTogglingId(null);
    }
  };

  const handleEdit = (account: AccountFull) => {
    setEditingId(account.id);
    setForm({
      name: account.name,
      isActive: account.isActive,
      description: account.description,
      accountId: account.accountId,
      accessKeyId: account.accessKeyId,
      secretAccessKey: account.secretAccessKey,
      bucketName: account.bucketName,
      endpoint: account.endpoint,
      publicDomain: account.publicDomain,
      apiToken: account.apiToken,
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

  const handleCopyId = (id: string) => {
    navigator.clipboard.writeText(id);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
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
        {paginatedAccounts.map((account) => (
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
              {/* 第二行：ID */}
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span className="font-mono">ID: {account.id}</span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-4 w-4 shrink-0"
                  onClick={() => handleCopyId(account.id)}
                >
                  {copiedId === account.id ? (
                    <Check className="h-3 w-3 text-green-500" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </Button>
              </div>
              {/* 第三行：Bucket + Endpoint */}
              <div className="text-sm text-muted-foreground truncate">
                {account.bucketName} · {account.endpoint}
              </div>
              {/* 第四行：用量信息 */}
              <div className="text-sm text-muted-foreground">
                容量: {formatBytes(account.usage.sizeBytes)} /{" "}
                {formatBytes(account.quota.maxSizeBytes)} | 写入:{" "}
                {formatNumber(account.usage.classAOps)} /{" "}
                {formatNumber(account.quota.maxClassAOps)}
              </div>
            </CardContent>
          </Card>
        ))}
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={accounts.length}
          pageSize={PAGE_SIZE}
        />
      </div>
      </div>
    </TooltipProvider>
  );
}
