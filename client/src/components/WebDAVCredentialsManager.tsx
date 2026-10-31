import { useEffect, useState, useMemo, useRef } from "react";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Pagination } from "@/components/ui/pagination";
import { Switch } from "@/components/ui/switch";
import {
  getWebDAVCredentials,
  createWebDAVCredential,
  updateWebDAVCredential,
  deleteWebDAVCredential,
  getAccounts,
  getUsers,
  rolePermissions,
  type WebDAVCredential,
  type WebDAVCredentialRequest,
  type Account,
  type UserProfile,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Plus, Trash2, RefreshCw, Copy, Check } from "lucide-react";
import { toast } from "sonner";
import ConfirmationDialog from "@/components/ConfirmationDialog";
import { useAuth } from "@/hooks/useAuth";

const PAGE_SIZE = 5;

const PERMISSIONS = [
  { value: "read", label: "读取", desc: "列表、下载" },
  { value: "write", label: "写入", desc: "上传文件" },
  { value: "delete", label: "删除", desc: "删除文件" },
];

/**
 * WebDAV 凭证管理组件
 */
export default function WebDAVCredentialsManager() {
  const { profile } = useAuth();
  const [credentials, setCredentials] = useState<WebDAVCredential[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [users, setUsers] = useState<UserProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<WebDAVCredentialRequest>({
    accountId: "",
    description: "",
    permissions: ["read", "write"],
    username: "",
    password: "",
    userId: profile?.id,
  });
  const [submitting, setSubmitting] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [loadError, setLoadError] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Pick<WebDAVCredential, "id" | "username"> | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const deleteInFlight = useRef(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);
  const availableUsers = users.filter((user) => user.isActive && (profile?.role === "owner" || user.id === profile?.id || user.role === "member" || user.role === "viewer"));
  const selectedUser = availableUsers.find((user) => user.id === (form.userId || profile?.id));
  const allowedPermissions = rolePermissions(selectedUser?.role || "viewer");

  // 分页计算
  const totalPages = Math.max(1, Math.ceil(credentials.length / PAGE_SIZE));
  const page = Math.min(currentPage, totalPages);
  const paginatedCredentials = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return credentials.slice(start, start + PAGE_SIZE);
  }, [credentials, page]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [credData, accData, memberData] = await Promise.all([
        getWebDAVCredentials(),
        getAccounts<Account>(),
        getUsers(),
      ]);
      setCredentials(credData || []);
      setAccounts(accData || []);
      setUsers(memberData);
      setLoadError(false);
    } catch (err) {
      console.error("加载数据失败:", err);
      setLoadError(true);
      toast.error("加载凭证失败");
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (submitting) return;
    if (!form.accountId) {
      toast.error("请选择关联账户");
      return;
    }
    if (form.permissions.length === 0) {
      toast.error("请选择至少一个权限");
      return;
    }
    if (!selectedUser) {
      toast.error("请选择有效的所属成员");
      return;
    }
    if (form.permissions.some((permission) => !allowedPermissions.some((allowed) => allowed === permission))) {
      toast.error("凭证权限不能超过所属成员的权限");
      return;
    }

    setSubmitting(true);
    try {
      await createWebDAVCredential({ ...form, userId: selectedUser.id });
      await loadData();
      setForm({
        accountId: "",
        description: "",
        permissions: ["read", "write"],
        username: "",
        password: "",
        userId: profile?.id,
      });
      setShowForm(false);
      toast.success("WebDAV 凭证创建成功");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleActive = async (cred: WebDAVCredential) => {
    if (togglingId) return;
    setTogglingId(cred.id);
    try {
      const member = users.find((user) => user.id === cred.userId);
      const allowed = member ? rolePermissions(member.role) : [];
      await updateWebDAVCredential(cred.id, {
        description: cred.description,
        permissions: cred.permissions.filter((permission) => allowed.some((item) => item === permission)),
        isActive: !cred.isActive,
      });
      await loadData();
      toast.success(cred.isActive ? "凭证已停用" : "凭证已启用");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "更新失败");
    } finally {
      setTogglingId(null);
    }
  };

  const handleDelete = (target: WebDAVCredential) => {
    if (loading || submitting || togglingId !== null || deletingId !== null) return;
    setDeleteTarget({ id: target.id, username: target.username });
    setDeleteError("");
  };

  const handleConfirmDelete = async () => {
    const target = deleteTarget;
    if (!target || deleteInFlight.current || loading || submitting || togglingId !== null || deletingId !== null) return;
    deleteInFlight.current = true;
    setDeletingId(target.id);
    setDeleteError("");
    try {
      await deleteWebDAVCredential(target.id);
      setDeleteTarget(null);
      setDeleteError("");
      toast.success("WebDAV 凭证已删除");
      await loadData();
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

  const handleCopy = async (value: string, field: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedField(field);
    toast.success("已复制到剪贴板");
    setTimeout(() => setCopiedField(null), 2000);
  };

  const togglePermission = (perm: string) => {
    if (form.permissions.includes(perm)) {
      setForm({
        ...form,
        permissions: form.permissions.filter((p) => p !== perm),
      });
    } else {
      setForm({
        ...form,
        permissions: [...form.permissions, perm],
      });
    }
  };

  const getAccountName = (accountId: string) => {
    const acc = accounts.find((a) => a.id === accountId);
    return acc ? acc.name : accountId;
  };

  const getBucketName = (accountId: string) => {
    const acc = accounts.find((a) => a.id === accountId);
    return acc ? acc.bucketName : "";
  };

  useEffect(() => {
    void Promise.resolve().then(loadData);
  }, []);

  if (loading && credentials.length === 0 && !showForm) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 工具栏 */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-muted-foreground">
          共 {credentials.length} 个凭证
        </div>
        <Button disabled={loading || loadError || submitting || deletingId !== null || togglingId !== null} onClick={() => setShowForm(true)}>
          <Plus className="mr-2 h-4 w-4" />
          创建凭证
        </Button>
      </div>

      {/* 创建表单 */}
      <Dialog open={showForm} onOpenChange={(open) => { if (!submitting) setShowForm(open); }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg" onEscapeKeyDown={(event) => { if (submitting) event.preventDefault(); }} onPointerDownOutside={(event) => { if (submitting) event.preventDefault(); }}>
          <DialogHeader><DialogTitle>创建 WebDAV 凭证</DialogTitle><DialogDescription>选择存储账户与所属成员，凭证权限受成员当前角色限制。</DialogDescription></DialogHeader>
          <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); void handleSubmit(); }}>
            <fieldset disabled={submitting} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="webdav-owner">所属成员 *</Label>
              <select id="webdav-owner" className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" required value={form.userId || profile?.id || ""} onChange={(event) => { const member = availableUsers.find((user) => user.id === event.target.value); const allowed = rolePermissions(member?.role || "viewer"); setForm({ ...form, userId: event.target.value, permissions: form.permissions.filter((permission) => allowed.some((item) => item === permission)) }); }}>
                <option value="" disabled>请选择成员</option>{availableUsers.map((user) => <option key={user.id} value={user.id}>{user.displayName || user.username}（{user.username}）</option>)}
              </select>
              <p className="text-xs text-muted-foreground">停用成员后，其 WebDAV 凭证立即停止访问。</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="webdavcredentialsmanager-field-1">关联账户 *</Label>
              <select id="webdavcredentialsmanager-field-1" required
                value={form.accountId}
                onChange={(e) => setForm({ ...form, accountId: e.target.value })}
                className="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              >
                <option value="">选择存储账户</option>
                {accounts
                  .filter((acc) => acc.permissions?.webdav)
                  .map((acc) => (
                    <option key={acc.id} value={acc.id}>
                      {acc.name} ({acc.bucketName})
                    </option>
                  ))}
              </select>
              <p className="text-xs text-muted-foreground">
                仅显示已启用 WebDAV 权限的账户
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="webdavcredentialsmanager-field-2">描述</Label>
              <Input id="webdavcredentialsmanager-field-2"
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="例如: Windows 文件资源管理器"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="webdavcredentialsmanager-field-3">用户名</Label>
              <Input id="webdavcredentialsmanager-field-3"
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                placeholder="留空自动生成（格式: FFLW_WebDAV_XXXXXXXX）"
              />
              <p className="text-xs text-muted-foreground">
                可自定义用户名，留空将自动生成
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="webdavcredentialsmanager-field-4">密码</Label>
              <Input id="webdavcredentialsmanager-field-4"
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder="留空自动生成 32 位随机密码"
              />
              <p className="text-xs text-muted-foreground">
                可自定义密码，留空将自动生成 32 位随机密码
              </p>
            </div>
            <div className="space-y-2">
              <Label>权限 *</Label>
              <div className="flex flex-wrap gap-2">
                {PERMISSIONS.map((perm) => (
                  <button
                    key={perm.value}
                    type="button"
                    disabled={!allowedPermissions.some((allowed) => allowed === perm.value)}
                    title={!allowedPermissions.some((allowed) => allowed === perm.value) ? "所属成员没有此权限" : undefined}
                    aria-pressed={form.permissions.includes(perm.value)}
                    onClick={() => togglePermission(perm.value)}
                    className={`px-3 py-1.5 rounded-md text-sm border transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${
                      form.permissions.includes(perm.value)
                        ? "bg-primary text-primary-foreground border-primary"
                        : "bg-background border-input hover:bg-accent"
                    }`}
                  >
                    {perm.label}
                  </button>
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                {PERMISSIONS.filter((p) => form.permissions.includes(p.value))
                  .map((p) => p.desc)
                  .join("、") || "无权限"}
              </p>
            </div>
            </fieldset>
            <DialogFooter><Button type="button" variant="outline" disabled={submitting} onClick={() => setShowForm(false)}>取消</Button><Button type="submit" disabled={submitting}>{submitting ? "创建中..." : "创建"}</Button></DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <div className="space-y-2">
        {loadError && <div role="alert" className="flex items-center justify-between gap-3 py-2 text-sm text-destructive"><span>WebDAV 凭证加载失败</span><Button variant="outline" size="sm" disabled={loading || submitting || deletingId !== null || togglingId !== null} onClick={loadData}>重试</Button></div>}
        <div className="overflow-x-auto border-y">
          <table className="w-full min-w-[900px] text-left text-sm">
            <caption className="sr-only">WebDAV 凭证列表</caption>
            <thead className="border-b bg-muted/40 text-muted-foreground"><tr><th scope="col" className="px-3 py-3">关联账户</th><th scope="col" className="px-3 py-3">所属成员</th><th scope="col" className="px-3 py-3">用户名 / 密码</th><th scope="col" className="px-3 py-3">权限</th><th scope="col" className="px-3 py-3">使用记录</th><th scope="col" className="px-3 py-3">启用</th><th scope="col" className="px-3 py-3 text-right">操作</th></tr></thead>
            <tbody className="divide-y">
              {paginatedCredentials.map((cred) => (
                <tr key={cred.id} className={cred.isActive ? "hover:bg-muted/30" : "bg-muted/30 text-muted-foreground"}>
                  <td className="max-w-56 px-3 py-4 align-top"><div className="font-medium">{getAccountName(cred.accountId)}</div><p className="mt-1 font-mono text-xs text-muted-foreground">{getBucketName(cred.accountId)}</p>{cred.description && <p className="mt-1 break-words text-xs text-muted-foreground">{cred.description}</p>}</td>
                  <td className="max-w-40 break-all px-3 py-4 align-top">{users.find((user) => user.id === cred.userId)?.displayName || users.find((user) => user.id === cred.userId)?.username || cred.userId}{users.find((user) => user.id === cred.userId)?.isActive === false && <p className="mt-1 text-xs text-muted-foreground">成员已停用</p>}</td>
                  <td className="px-3 py-4 align-top">
                    <div className="flex items-center gap-2"><span className="w-10 text-xs text-muted-foreground">用户</span><code className="max-w-56 truncate select-all font-mono text-xs" title={cred.username}>{cred.username}</code><Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" aria-label={`复制 ${cred.username} 的用户名`} onClick={() => handleCopy(cred.username, cred.id + "-user")}>{copiedField === cred.id + "-user" ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}</Button></div>
                    <div className="mt-1 flex items-center gap-2"><span className="w-10 text-xs text-muted-foreground">密码</span><code className="max-w-56 truncate select-all font-mono text-xs" title={cred.password}>{cred.password}</code><Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" aria-label={`复制 ${cred.username} 的密码`} onClick={() => handleCopy(cred.password, cred.id + "-pass")}>{copiedField === cred.id + "-pass" ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}</Button></div>
                  </td>
                  <td className="px-3 py-4 align-top"><div className="flex flex-wrap gap-1">{cred.permissions.map((permission) => <span key={permission} className="rounded bg-secondary px-2 py-0.5 text-xs">{PERMISSIONS.find((item) => item.value === permission)?.label || permission}</span>)}</div></td>
                  <td className="whitespace-nowrap px-3 py-4 align-top text-xs text-muted-foreground"><div>创建 {formatDate(cred.createdAt)}</div><div className="mt-1">{cred.lastUsedAt ? `使用 ${formatDate(cred.lastUsedAt)}` : "尚未使用"}</div></td>
                  <td className="px-3 py-4 align-top"><Switch aria-label={`启用凭证 ${cred.username}`} checked={cred.isActive} disabled={loading || togglingId !== null || deletingId !== null} onCheckedChange={() => handleToggleActive(cred)} /></td>
                  <td className="px-3 py-4 text-right align-top"><Button variant="ghost" size="icon" className="text-destructive" aria-label={`删除凭证 ${cred.username}`} title="删除凭证" disabled={loading || deletingId !== null || togglingId !== null} onClick={() => handleDelete(cred)}>{deletingId === cred.id ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}</Button></td>
                </tr>
              ))}
              {credentials.length === 0 && <tr><td colSpan={7} className="space-y-1 py-12 text-center text-muted-foreground"><p>暂无 WebDAV 凭证</p><p className="text-xs">创建凭证以使用 WebDAV 客户端访问</p></td></tr>}
            </tbody>
          </table>
        </div>
        <fieldset disabled={loading || submitting || deletingId !== null || togglingId !== null}><Pagination currentPage={page} totalPages={totalPages} onPageChange={setCurrentPage} totalItems={credentials.length} pageSize={PAGE_SIZE} /></fieldset>
      </div>
      <ConfirmationDialog
        open={deleteTarget !== null}
        onOpenChange={handleDeleteOpenChange}
        title="删除 WebDAV 凭证"
        description={`确定要删除 WebDAV 凭证「${deleteTarget?.username || ""}」吗？删除后使用此凭证的客户端将无法访问。`}
        busy={deletingId !== null}
        error={deleteError || undefined}
        onConfirm={handleConfirmDelete}
      />


    </div>
  );
}
