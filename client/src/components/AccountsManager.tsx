import { useEffect, useState, useCallback, useRef } from "react";
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
import { Plus, Pencil, Trash2, RefreshCw, Eraser, Copy, Check, Calendar } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import ConfirmationDialog from "@/components/ConfirmationDialog";

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
    maxClassBOps: 0,
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
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Pick<AccountFull, "id" | "name"> | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const deleteInFlight = useRef(false);
  const [loadError, setLoadError] = useState(false);
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
  const latestLoadRequest = useRef(0);
  const busy = loading || submitting || togglingId !== null || clearingId !== null || deletingId !== null || deletingOldFiles;
  const invalidateLoads = useCallback(() => { latestLoadRequest.current++; }, []);

  const loadAccounts = useCallback(async (page: number) => {
    const request = ++latestLoadRequest.current;
    setLoading(true);
    try {
      let result = await getAccountsPaged(page, PAGE_SIZE);
      if (request !== latestLoadRequest.current) return;
      const lastPage = Math.max(1, result.totalPages);
      if (page > lastPage) result = await getAccountsPaged(lastPage, PAGE_SIZE);
      if (request !== latestLoadRequest.current) return;
      setAccounts(result.items || []);
      setTotalPages(Math.max(1, result.totalPages));
      setTotalItems(result.total);
      setCurrentPage(result.page);
      setLoadError(false);
    } catch {
      if (request !== latestLoadRequest.current) return;
      setLoadError(true);
      toast.error("加载账户失败");
    } finally {
      if (request === latestLoadRequest.current) setLoading(false);
    }
  }, []);

  const handlePageChange = (page: number) => {
    if (busy) return;
    setSelectedIds(new Set());
    void loadAccounts(page);
  };

  const handleSubmit = async () => {
    if (submitting) return;
    const requiredFields = [form.name, form.accountId, form.bucketName, form.endpoint, form.publicDomain];
    if (!editingId) requiredFields.push(form.accessKeyId, form.secretAccessKey);
    if (requiredFields.some((value) => !value.trim())) {
      toast.error("请填写所有必填字段");
      return;
    }
    if (!Number.isSafeInteger(form.quota.maxSizeBytes) || !Number.isSafeInteger(form.quota.maxClassAOps) || !Number.isSafeInteger(form.quota.maxClassBOps || 0) || form.quota.maxSizeBytes < 0 || form.quota.maxClassAOps < 0 || (form.quota.maxClassBOps || 0) < 0) {
      toast.error("容量和请求配额必须是有效的非负整数");
      return;
    }

    setSubmitting(true);
    try {
      if (editingId) await updateAccount(editingId, form);
      else await createAccount(form);
      toast.success(editingId ? "账户已更新" : "账户已创建");
      setShowForm(false);
      setEditingId(null);
      setForm(defaultAccountForm);
      await loadAccounts(currentPage);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleActive = async (account: AccountFull) => {
    if (togglingId) return;
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
      toast.error(err instanceof Error ? err.message : "操作失败");
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

  const handleDelete = (target: AccountFull) => {
    if (busy) return;
    setDeleteTarget({ id: target.id, name: target.name });
    setDeleteError("");
  };

  const handleConfirmDelete = async () => {
    const target = deleteTarget;
    if (!target || deleteInFlight.current || busy) return;
    deleteInFlight.current = true;
    setDeletingId(target.id);
    setDeleteError("");
    try {
      await deleteAccount(target.id);
      setSelectedIds((selected) => new Set([...selected].filter((selectedId) => selectedId !== target.id)));
      setDeleteTarget(null);
      setDeleteError("");
      toast.success("账户已删除");
      await loadAccounts(currentPage);
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : "删除失败");
    } finally {
      deleteInFlight.current = false;
      setDeletingId(null);
    }
  };

  const handleDeleteOpenChange = (open: boolean) => {
    if (!open && !deleteInFlight.current) {
      setDeleteTarget(null);
      setDeleteError("");
    }
  };

  const handleClearBucket = async (id: string) => {
    if (clearingId) return;
    setClearingId(id);
    try {
      await clearBucket(id);
      await loadAccounts(currentPage);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "清空失败");
    } finally {
      setClearingId(null);
    }
  };

  const handleCancel = () => {
    if (submitting) return;
    setShowForm(false);
    setEditingId(null);
    setForm(defaultAccountForm);
  };

  const handleCopyId = async (id: string) => {
    try {
      await navigator.clipboard.writeText(id);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      toast.error("复制失败，请手动复制账户 ID");
    }
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
      toast.error("请先选择至少一个账户");
      return;
    }
    setDeleteBeforeDate("");
    setDeleteResults(null);
    setShowDeleteOldFilesDialog(true);
  };

  const handleDeleteOldFiles = async () => {
    if (deletingOldFiles) return;
    if (!deleteBeforeDate) {
      toast.error("请选择日期");
      return;
    }

    setDeletingOldFiles(true);
    try {
      const response = await deleteOldFiles(Array.from(selectedIds), deleteBeforeDate);
      setDeleteResults(response.results);
      await loadAccounts(currentPage);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除失败");
    } finally {
      setDeletingOldFiles(false);
    }
  };

  const handleCloseDeleteOldFilesDialog = () => {
    if (deletingOldFiles) return;
    setShowDeleteOldFilesDialog(false);
    setDeleteResults(null);
    setDeleteBeforeDate("");
  };

  useEffect(() => {
    let active = true;
    void Promise.resolve().then(() => { if (active) return loadAccounts(1); });
    return () => {
      active = false;
      invalidateLoads();
    };
  }, [loadAccounts, invalidateLoads]);

  if (loading && accounts.length === 0 && !showForm) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <TooltipProvider>
      <div className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-4">
            <div className="text-sm text-muted-foreground">
              共 {totalItems} 个账户
            </div>
            {totalItems > 0 && (
              <div className="flex items-center gap-2">
                <Checkbox
                  id="select-all"
                  disabled={busy}
                  checked={accounts.length > 0 && accounts.every((account) => selectedIds.has(account.id)) ? true : accounts.some((account) => selectedIds.has(account.id)) ? "indeterminate" : false}
                  onCheckedChange={(checked) => handleSelectAll(checked === true)}
                />
                <Label htmlFor="select-all" className="text-sm cursor-pointer">
                  选择当前页
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
                      disabled={busy}
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
          <Button disabled={busy} onClick={() => { setEditingId(null); setForm(defaultAccountForm); setShowForm(true); }}>
            <Plus className="mr-2 h-4 w-4" />
            添加账户
          </Button>
        </div>

      <Dialog open={showForm} onOpenChange={(open) => { if (!submitting && !open) handleCancel(); }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl" onEscapeKeyDown={(event) => { if (submitting) event.preventDefault(); }} onPointerDownOutside={(event) => { if (submitting) event.preventDefault(); }}>
          <DialogHeader>
            <DialogTitle>{editingId ? "编辑账户" : "添加账户"}</DialogTitle>
            <DialogDescription>配置 R2 账户、访问凭证、存储配额及权限。</DialogDescription>
          </DialogHeader>
          <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); void handleSubmit(); }}>
            <fieldset disabled={submitting} className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="account-field-1">账户名称 *</Label>
                <Input id="account-field-1" required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder="例如: 主账户"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-2">Cloudflare Account ID *</Label>
                <Input id="account-field-2" required
                  value={form.accountId}
                  onChange={(e) => setForm({ ...form, accountId: e.target.value })}
                  placeholder="Cloudflare 账户 ID"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-3">Access Key ID{!editingId && " *"}</Label>
                <Input id="account-field-3" required={!editingId}
                  value={form.accessKeyId}
                  onChange={(e) => setForm({ ...form, accessKeyId: e.target.value })}
                  placeholder={editingId ? "留空保留原 Access Key ID" : "R2 Access Key ID"}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-4">Secret Access Key{!editingId && " *"}</Label>
                <Input id="account-field-4" required={!editingId}
                  type="password"
                  value={form.secretAccessKey}
                  onChange={(e) => setForm({ ...form, secretAccessKey: e.target.value })}
                  placeholder={editingId ? "留空保留原 Secret Access Key" : "R2 Secret Access Key"}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-5">Bucket 名称 *</Label>
                <Input id="account-field-5" required
                  value={form.bucketName}
                  onChange={(e) => setForm({ ...form, bucketName: e.target.value })}
                  placeholder="my-bucket"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-6">Endpoint *</Label>
                <Input id="account-field-6" required
                  value={form.endpoint}
                  onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
                  placeholder="https://xxx.r2.cloudflarestorage.com"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-7">Public Domain *</Label>
                <Input id="account-field-7" required
                  value={form.publicDomain}
                  onChange={(e) => setForm({ ...form, publicDomain: e.target.value })}
                  placeholder="cdn.example.com"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-8">API Token (用于查询用量)</Label>
                <Input id="account-field-8"
                  type="password"
                  value={form.apiToken}
                  onChange={(e) => setForm({ ...form, apiToken: e.target.value })}
                  placeholder="Cloudflare API Token"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-9">最大容量 (GiB)</Label>
                <Input id="account-field-9"
                  type="number"
                  value={form.quota.maxSizeBytes / 1024 / 1024 / 1024}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      quota: {
                        ...form.quota,
                        maxSizeBytes: Math.round((Number(e.target.value) || 0) * 1024 * 1024 * 1024),
                      },
                    })
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-field-10">Class A 请求上限</Label>
                <Input id="account-field-10"
                  type="number"
                  min={0}
                  step={1}
                  value={form.quota.maxClassAOps}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      quota: {
                        ...form.quota,
                        maxClassAOps: Number(e.target.value) || 0,
                      },
                    })
                  }
                />
                <p className="text-xs text-muted-foreground">包括上传、列目录等操作</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="account-class-b-quota">Class B 请求上限</Label>
                <Input id="account-class-b-quota" type="number" min={0} step={1} value={form.quota.maxClassBOps || 0} onChange={(event) => setForm({ ...form, quota: { ...form.quota, maxClassBOps: Number(event.target.value) || 0 } })} />
                <p className="text-xs text-muted-foreground">包括文件读取和检查；0 表示未设置此限制</p>
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="account-description">描述</Label>
              <Input id="account-description" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} placeholder="账户用途或备注" />
            </div>
            <div className="flex items-center justify-between">
              <Label htmlFor="account-active">启用账户</Label>
              <Switch id="account-active" checked={form.isActive} onCheckedChange={(checked) => setForm({ ...form, isActive: checked })} />
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
            </fieldset>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={handleCancel} disabled={submitting}>取消</Button>
              <Button type="submit" disabled={submitting}>{submitting ? "保存中..." : "保存账户"}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <div className="space-y-2">
        {loadError && <div role="alert" className="flex items-center justify-between gap-3 py-2 text-sm text-destructive"><span>账户列表加载失败</span><Button variant="outline" size="sm" disabled={busy} onClick={() => loadAccounts(currentPage)}>重试</Button></div>}
        <div className="overflow-x-auto border-y">
          <table className="w-full min-w-[960px] text-left text-sm">
            <caption className="sr-only">R2 账户列表</caption>
            <thead className="border-b bg-muted/40 text-muted-foreground">
              <tr><th scope="col" className="w-10 px-3 py-3">选择</th><th scope="col" className="px-3 py-3">账户</th><th scope="col" className="px-3 py-3">存储配置</th><th scope="col" className="px-3 py-3">用量 / 配额</th><th scope="col" className="px-3 py-3">权限</th><th scope="col" className="px-3 py-3">启用</th><th scope="col" className="px-3 py-3 text-right">操作</th></tr>
            </thead>
            <tbody className="divide-y">
              {accounts.map((account) => (
                <tr key={account.id} className={account.isActive ? "hover:bg-muted/30" : "bg-muted/30 text-muted-foreground"}>
                  <td className="px-3 py-4 align-top"><Checkbox aria-label={`选择账户 ${account.name}`} checked={selectedIds.has(account.id)} onCheckedChange={(checked) => handleSelectAccount(account.id, checked === true)} disabled={busy} /></td>
                  <td className="max-w-64 px-3 py-4 align-top">
                    <div className="font-medium">{account.name}</div>
                    {account.description && <p className="mt-1 max-w-56 break-words text-xs text-muted-foreground">{account.description}</p>}
                    <div className="mt-2 flex items-center gap-1 text-xs text-muted-foreground"><span className="max-w-44 truncate font-mono" title={account.id}>{account.id}</span><Button variant="ghost" size="icon" className="h-6 w-6 shrink-0" aria-label={`复制 ${account.name} 的账户 ID`} onClick={() => handleCopyId(account.id)}>{copiedId === account.id ? <Check className="h-3 w-3 text-green-600" /> : <Copy className="h-3 w-3" />}</Button></div>
                  </td>
                  <td className="max-w-64 px-3 py-4 align-top"><div className="font-mono">{account.bucketName}</div><div className="mt-1 max-w-60 truncate text-xs text-muted-foreground" title={account.endpoint}>{account.endpoint}</div><div className="mt-1 max-w-60 truncate text-xs text-muted-foreground" title={account.publicDomain}>{account.publicDomain}</div></td>
                  <td className="whitespace-nowrap px-3 py-4 align-top"><div>{formatBytes(account.usage.sizeBytes)} / {formatBytes(account.quota.maxSizeBytes)}</div><div className="mt-1 text-xs text-muted-foreground">Class A {formatNumber(account.usage.classAOps)} / {formatNumber(account.quota.maxClassAOps)}</div><div className="mt-1 text-xs text-muted-foreground">Class B {formatNumber(account.usage.classBOps)} / {account.quota.maxClassBOps ? formatNumber(account.quota.maxClassBOps) : "未限制"}</div></td>
                  <td className="px-3 py-4 align-top"><div className="flex max-w-48 flex-wrap gap-1">{account.permissions?.webdav && <Badge variant="secondary">WebDAV</Badge>}{account.permissions?.autoUpload && <Badge variant="secondary">自动上传</Badge>}{account.permissions?.apiUpload && <Badge variant="secondary">API 上传</Badge>}{account.permissions?.clientUpload && <Badge variant="secondary">前端上传</Badge>}{account.permissions && !Object.values(account.permissions).some(Boolean) && <span className="text-xs text-muted-foreground">无权限</span>}</div></td>
                  <td className="px-3 py-4 align-top"><Switch aria-label={`启用账户 ${account.name}`} checked={account.isActive} onCheckedChange={() => handleToggleActive(account)} disabled={busy} /></td>
                  <td className="px-3 py-4 align-top">
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="icon" aria-label={`编辑账户 ${account.name}`} title="编辑账户" onClick={() => handleEdit(account)} disabled={busy}><Pencil className="h-4 w-4" /></Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild><Button variant="ghost" size="icon" className="text-destructive hover:text-destructive" aria-label={`清空 ${account.name} 的存储桶`} title="清空存储桶" disabled={busy}>{clearingId === account.id ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Eraser className="h-4 w-4" />}</Button></AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader><AlertDialogTitle>确认清空存储桶</AlertDialogTitle><AlertDialogDescription>此操作将删除账户「{account.name}」存储桶中的所有文件，且无法恢复。确定要继续吗？</AlertDialogDescription></AlertDialogHeader>
                          <AlertDialogFooter><AlertDialogCancel>取消</AlertDialogCancel><AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={() => handleClearBucket(account.id)}>确认清空</AlertDialogAction></AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                      <Button variant="ghost" size="icon" className="text-destructive hover:text-destructive" aria-label={`删除账户 ${account.name}`} title="删除账户" onClick={() => handleDelete(account)} disabled={busy}>{deletingId === account.id ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}</Button>
                    </div>
                  </td>
                </tr>
              ))}
              {accounts.length === 0 && <tr><td colSpan={7} className="py-12 text-center text-muted-foreground">暂无 R2 账户，点击“添加账户”开始配置。</td></tr>}
            </tbody>
          </table>
        </div>
        <fieldset disabled={busy} aria-busy={loading}><Pagination currentPage={currentPage} totalPages={totalPages} onPageChange={handlePageChange} totalItems={totalItems} pageSize={PAGE_SIZE} /></fieldset>
      </div>

      <ConfirmationDialog
        open={deleteTarget !== null}
        onOpenChange={handleDeleteOpenChange}
        title="删除账户"
        description={`确定要删除账户「${deleteTarget?.name || ""}」吗？删除后该账户将从列表中移除。`}
        busy={deletingId !== null}
        error={deleteError || undefined}
        onConfirm={handleConfirmDelete}
      />

      {/* 删除旧文件对话框 */}
      <Dialog open={showDeleteOldFilesDialog} onOpenChange={(open) => { if (!deletingOldFiles) setShowDeleteOldFilesDialog(open); }}>
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
                <Button variant="outline" onClick={handleCloseDeleteOldFilesDialog} disabled={deletingOldFiles}>
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
