import { useEffect, useState, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Checkbox } from "@/components/ui/checkbox";
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  getAccountsPaged,
  createAccount,
  updateAccount,
  deleteAccount,
  clearBucket,
  deleteOldFiles,
  type AccountRequest,
  type AccountFull,
  type DeleteOldFilesResult,
} from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/utils";
import { Plus, Pencil, Trash2, RefreshCw, X, Eraser, Copy, Check, Calendar } from "lucide-react";
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
  permissions: {
    webdav: true,
    autoUpload: true,
    apiUpload: true,
    clientUpload: true,
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
  const [totalPages, setTotalPages] = useState(1);
  const [totalItems, setTotalItems] = useState(0);

  // 多选相关状态
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [showDeleteOldFilesDialog, setShowDeleteOldFilesDialog] = useState(false);
  const [deleteBeforeDate, setDeleteBeforeDate] = useState("");
  const [deletingOldFiles, setDeletingOldFiles] = useState(false);
  const [deleteResults, setDeleteResults] = useState<DeleteOldFilesResult[] | null>(null);

  const loadAccounts = useCallback(async (page: number = currentPage) => {
    setLoading(true);
    try {
      const data = await getAccountsPaged(page, PAGE_SIZE);
      setAccounts(data.items || []);
      setTotalPages(data.totalPages);
      setTotalItems(data.total);
      setCurrentPage(data.page);
    } catch (err) {
      console.error("加载账户失败:", err);
    } finally {
      setLoading(false);
    }
  }, [currentPage]);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    loadAccounts(page);
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
        permissions: result.permissions,
      });

      await loadAccounts(currentPage);
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
        permissions: account.permissions,
      });
      await loadAccounts(currentPage);
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
      permissions: account.permissions,
    });
    setShowForm(true);
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除此账户吗？")) return;
    try {
      await deleteAccount(id);
      await loadAccounts(currentPage);
    } catch (err) {
      alert(err instanceof Error ? err.message : "删除失败");
    }
  };

  const handleClearBucket = async (id: string) => {
    setClearingId(id);
    try {
      await clearBucket(id);
      await loadAccounts(currentPage);
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

  // 多选相关处理函数
  const handleSelectAccount = (id: string, checked: boolean) => {
    const newSelected = new Set(selectedIds);
    if (checked) {
      newSelected.add(id);
    } else {
      newSelected.delete(id);
    }
    setSelectedIds(newSelected);
  };

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(accounts.map((a) => a.id)));
    } else {
      setSelectedIds(new Set());
    }
  };

  const handleOpenDeleteOldFilesDialog = () => {
    if (selectedIds.size === 0) {
      alert("请先选择至少一个账户");
      return;
    }
    setDeleteBeforeDate("");
    setDeleteResults(null);
    setShowDeleteOldFilesDialog(true);
  };

  const handleDeleteOldFiles = async () => {
    if (!deleteBeforeDate) {
      alert("请选择日期");
      return;
    }

    setDeletingOldFiles(true);
    try {
      const response = await deleteOldFiles(Array.from(selectedIds), deleteBeforeDate);
      setDeleteResults(response.results);
      await loadAccounts(currentPage);
    } catch (err) {
      alert(err instanceof Error ? err.message : "删除失败");
    } finally {
      setDeletingOldFiles(false);
    }
  };

  const handleCloseDeleteOldFilesDialog = () => {
    setShowDeleteOldFilesDialog(false);
    setDeleteResults(null);
    setDeleteBeforeDate("");
  };

  useEffect(() => {
    loadAccounts(1);
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
        <div className="flex justify-between items-center">
          <div className="flex items-center gap-4">
            <div className="text-sm text-muted-foreground">
              共 {totalItems} 个账户
            </div>
            {totalItems > 0 && (
              <div className="flex items-center gap-2">
                <Checkbox
                  id="select-all"
                  checked={selectedIds.size === accounts.length && accounts.length > 0}
                  onCheckedChange={(checked) => handleSelectAll(checked === true)}
                />
                <Label htmlFor="select-all" className="text-sm cursor-pointer">
                  全选
                </Label>
              </div>
            )}
            {selectedIds.size > 0 && (
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">
                  已选 {selectedIds.size} 个
                </span>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleOpenDeleteOldFilesDialog}
                    >
                      <Calendar className="mr-2 h-4 w-4" />
                      删除旧文件
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>删除选中账户中指定日期之前的文件</TooltipContent>
                </Tooltip>
              </div>
            )}
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
            {/* 权限设置 */}
            <div className="space-y-3">
              <Label>权限设置</Label>
              <div className="grid gap-3 md:grid-cols-4">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="perm-webdav"
                    checked={form.permissions?.webdav ?? true}
                    onCheckedChange={(checked) =>
                      setForm({
                        ...form,
                        permissions: { ...form.permissions, webdav: checked === true },
                      })
                    }
                  />
                  <Label htmlFor="perm-webdav" className="text-sm cursor-pointer">WebDAV</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="perm-auto-upload"
                    checked={form.permissions?.autoUpload ?? true}
                    onCheckedChange={(checked) =>
                      setForm({
                        ...form,
                        permissions: { ...form.permissions, autoUpload: checked === true },
                      })
                    }
                  />
                  <Label htmlFor="perm-auto-upload" className="text-sm cursor-pointer">自动上传</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="perm-api-upload"
                    checked={form.permissions?.apiUpload ?? true}
                    onCheckedChange={(checked) =>
                      setForm({
                        ...form,
                        permissions: { ...form.permissions, apiUpload: checked === true },
                      })
                    }
                  />
                  <Label htmlFor="perm-api-upload" className="text-sm cursor-pointer">API 上传</Label>
                </div>
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="perm-client-upload"
                    checked={form.permissions?.clientUpload ?? true}
                    onCheckedChange={(checked) =>
                      setForm({
                        ...form,
                        permissions: { ...form.permissions, clientUpload: checked === true },
                      })
                    }
                  />
                  <Label htmlFor="perm-client-upload" className="text-sm cursor-pointer">前端上传</Label>
                </div>
              </div>
              <p className="text-xs text-muted-foreground">
                控制此账户可被使用的方式。取消勾选将禁止相应的访问方式。
              </p>
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
              {/* 第一行：复选框 + 名称 + 操作按钮 */}
              <div className="flex items-center justify-between gap-4">
                <div className="flex items-center gap-3 min-w-0">
                  <Checkbox
                    checked={selectedIds.has(account.id)}
                    onCheckedChange={(checked) => handleSelectAccount(account.id, checked === true)}
                  />
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
              {/* 第五行：权限标签 */}
              <div className="flex flex-wrap gap-1.5">
                {account.permissions?.webdav && (
                  <Badge variant="secondary" className="text-xs">WebDAV</Badge>
                )}
                {account.permissions?.autoUpload && (
                  <Badge variant="secondary" className="text-xs">自动上传</Badge>
                )}
                {account.permissions?.apiUpload && (
                  <Badge variant="secondary" className="text-xs">API上传</Badge>
                )}
                {account.permissions?.clientUpload && (
                  <Badge variant="secondary" className="text-xs">前端上传</Badge>
                )}
                {account.permissions && !account.permissions.webdav &&
                 !account.permissions.autoUpload && !account.permissions.apiUpload && !account.permissions.clientUpload && (
                  <Badge variant="outline" className="text-xs text-muted-foreground">无权限</Badge>
                )}
              </div>
            </CardContent>
          </Card>
        ))}
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={handlePageChange}
          totalItems={totalItems}
          pageSize={PAGE_SIZE}
        />
      </div>

      {/* 删除旧文件对话框 */}
      <Dialog open={showDeleteOldFilesDialog} onOpenChange={setShowDeleteOldFilesDialog}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>删除旧文件</DialogTitle>
            <DialogDescription>
              删除选中账户中指定日期之前的所有文件，此操作不可恢复。
            </DialogDescription>
          </DialogHeader>

          {!deleteResults ? (
            <>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label>已选择 {selectedIds.size} 个账户</Label>
                  <div className="text-sm text-muted-foreground max-h-24 overflow-y-auto">
                    {accounts
                      .filter((a) => selectedIds.has(a.id))
                      .map((a) => a.name)
                      .join("、")}
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="before-date">删除此日期之前的文件</Label>
                  <Input
                    id="before-date"
                    type="date"
                    value={deleteBeforeDate}
                    onChange={(e) => setDeleteBeforeDate(e.target.value)}
                    max={new Date().toISOString().split("T")[0]}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={handleCloseDeleteOldFilesDialog}>
                  取消
                </Button>
                <Button
                  variant="destructive"
                  onClick={handleDeleteOldFiles}
                  disabled={deletingOldFiles || !deleteBeforeDate}
                >
                  {deletingOldFiles ? (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                      删除中...
                    </>
                  ) : (
                    "确认删除"
                  )}
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label>删除结果</Label>
                  <div className="space-y-2 max-h-64 overflow-y-auto">
                    {deleteResults.map((result) => (
                      <div
                        key={result.accountId}
                        className={`p-3 rounded-md text-sm ${
                          result.error
                            ? "bg-destructive/10 text-destructive"
                            : "bg-muted"
                        }`}
                      >
                        <div className="font-medium">{result.accountName}</div>
                        {result.error ? (
                          <div className="text-xs mt-1">错误: {result.error}</div>
                        ) : (
                          <div className="text-xs mt-1 text-muted-foreground">
                            已删除 {result.deletedCount} 个文件
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                  <div className="text-sm text-muted-foreground pt-2 border-t">
                    共删除{" "}
                    {deleteResults.reduce((sum, r) => sum + r.deletedCount, 0)}{" "}
                    个文件
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button onClick={handleCloseDeleteOldFilesDialog}>关闭</Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
      </div>
    </TooltipProvider>
  );
}
