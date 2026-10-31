import { useEffect, useState, useMemo, useRef } from "react";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Pagination } from "@/components/ui/pagination";
import {
  getTokens,
  createToken,
  deleteToken,
  getUsers,
  rolePermissions,
  type UserProfile,
  type Token,
  type TokenRequest,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Plus, Trash2, RefreshCw, Copy, Check } from "lucide-react";
import { toast } from "sonner";
import ConfirmationDialog from "@/components/ConfirmationDialog";
import { useAuth } from "@/hooks/useAuth";
import { Link } from "react-router-dom";

const PAGE_SIZE = 5;

const PERMISSIONS = [
  { value: "read", label: "读取", desc: "列表、下载、获取链接" },
  { value: "write", label: "写入", desc: "上传文件" },
  { value: "delete", label: "删除", desc: "删除文件" },
  { value: "manage", label: "MCP 管理", desc: "通过 MCP 管理团队、账号、系统设置、凭证和操作记录" },
];

export default function TokensManager() {
  const { profile } = useAuth();
  const [tokens, setTokens] = useState<Token[]>([]);
  const [users, setUsers] = useState<UserProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<TokenRequest>({
    name: "",
    permissions: ["read"],
    userId: profile?.id,
  });
  const [submitting, setSubmitting] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [loadError, setLoadError] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Pick<Token, "id" | "name"> | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const deleteInFlight = useRef(false);
  const availableUsers = users.filter((user) => user.isActive && (profile?.role === "owner" || user.id === profile?.id || user.role === "member" || user.role === "viewer"));
  const selectedUser = availableUsers.find((user) => user.id === (form.userId || profile?.id));
  const allowedPermissions = rolePermissions(selectedUser?.role || "viewer");

  // 分页计算
  const totalPages = Math.max(1, Math.ceil(tokens.length / PAGE_SIZE));
  const page = Math.min(currentPage, totalPages);
  const paginatedTokens = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return tokens.slice(start, start + PAGE_SIZE);
  }, [tokens, page]);

  const loadTokens = async () => {
    setLoading(true);
    try {
      const [data, members] = await Promise.all([getTokens(), getUsers()]);
      setTokens(data || []);
      setUsers(members);
      setLoadError(false);
    } catch (err) {
      console.error("加载令牌失败:", err);
      setLoadError(true);
      toast.error("加载令牌失败");
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (submitting) return;
    if (!form.name.trim()) {
      toast.error("请输入令牌名称");
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
      toast.error("令牌权限不能超过所属成员的权限");
      return;
    }

    setSubmitting(true);
    try {
      await createToken({ ...form, userId: selectedUser.id });
      await loadTokens();
      setForm({ name: "", permissions: ["read"], userId: profile?.id });
      setShowForm(false);
      toast.success("令牌创建成功");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = (target: Token) => {
    if (loading || submitting || deletingId !== null) return;
    setDeleteTarget({ id: target.id, name: target.name });
    setDeleteError("");
  };

  const handleConfirmDelete = async () => {
    const target = deleteTarget;
    if (!target || deleteInFlight.current || loading || submitting || deletingId !== null) return;
    deleteInFlight.current = true;
    setDeletingId(target.id);
    setDeleteError("");
    try {
      await deleteToken(target.id);
      setDeleteTarget(null);
      setDeleteError("");
      toast.success("令牌已删除");
      await loadTokens();
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

  const handleCopy = async (tokenValue: string, tokenId: string) => {
    await navigator.clipboard.writeText(tokenValue);
    setCopiedId(tokenId);
    toast.success("已复制到剪贴板");
    setTimeout(() => setCopiedId(null), 2000);
  };

  const togglePermission = (perm: string) => {
    if (perm === "manage" && !form.permissions.includes("manage")) {
      setForm({ ...form, permissions: ["read", "write", "delete", "manage"] });
      return;
    }
    if (form.permissions.includes(perm)) {
      setForm({
        ...form,
        permissions: form.permissions.filter((p) => p !== perm && (perm === "manage" || p !== "manage")),
      });
    } else {
      setForm({
        ...form,
        permissions: [...form.permissions, perm],
      });
    }
  };

  useEffect(() => {
    void Promise.resolve().then(loadTokens);
  }, []);

  if (loading && tokens.length === 0 && !showForm) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <p className="text-sm text-muted-foreground">API 令牌也可用于 Agent 连接 MCP。<Link to="/mcp-docs" className="text-primary underline underline-offset-4">查看 MCP 使用说明</Link></p>
      {/* 工具栏 */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-muted-foreground">
          共 {tokens.length} 个令牌
        </div>
        <Button disabled={loading || loadError || submitting || deletingId !== null} onClick={() => setShowForm(true)}>
          <Plus className="mr-2 h-4 w-4" />
          创建令牌
        </Button>
      </div>

      {/* 表单 */}
      <Dialog open={showForm} onOpenChange={(open) => { if (!submitting) setShowForm(open); }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg" onEscapeKeyDown={(event) => { if (submitting) event.preventDefault(); }} onPointerDownOutside={(event) => { if (submitting) event.preventDefault(); }}>
          <DialogHeader><DialogTitle>创建 API 令牌</DialogTitle><DialogDescription>令牌属于指定成员，实际访问权限受成员当前角色限制。</DialogDescription></DialogHeader>
          <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); void handleSubmit(); }}>
            <fieldset disabled={submitting} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="token-owner">所属成员 *</Label>
              <select id="token-owner" className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" required value={form.userId || profile?.id || ""} onChange={(event) => { const member = availableUsers.find((user) => user.id === event.target.value); const allowed = rolePermissions(member?.role || "viewer"); setForm({ ...form, userId: event.target.value, permissions: form.permissions.filter((permission) => allowed.some((item) => item === permission)) }); }}>
                <option value="" disabled>请选择成员</option>{availableUsers.map((user) => <option key={user.id} value={user.id}>{user.displayName || user.username}（{user.username}）</option>)}
              </select>
              <p className="text-xs text-muted-foreground">停用成员后，其令牌立即停止访问。</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="tokensmanager-field-1">令牌名称 *</Label>
              <Input id="tokensmanager-field-1" required
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="例如: 图床上传"
              />
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
                {PERMISSIONS.filter((p) =>
                  form.permissions.includes(p.value)
                )
                  .map((p) => p.desc)
                  .join("、") || "无权限"}
              </p>
              <p className="text-xs text-muted-foreground">MCP 管理仅可授予所有者或管理员，只在 MCP 接口生效。选择后同时授予读取、写入和删除，允许 Agent 全量管理；取消任一文件权限也会取消 MCP 管理。</p>
            </div>
            </fieldset>
            <DialogFooter><Button type="button" variant="outline" disabled={submitting} onClick={() => setShowForm(false)}>取消</Button><Button type="submit" disabled={submitting}>{submitting ? "创建中..." : "创建"}</Button></DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <div className="space-y-2">
        {loadError && <div role="alert" className="flex items-center justify-between gap-3 py-2 text-sm text-destructive"><span>令牌列表加载失败</span><Button variant="outline" size="sm" disabled={loading || submitting || deletingId !== null} onClick={loadTokens}>重试</Button></div>}
        <div className="overflow-x-auto border-y">
          <table className="w-full min-w-[640px] text-left text-sm">
            <caption className="sr-only">API 令牌列表</caption>
            <thead className="border-b bg-muted/40 text-muted-foreground"><tr><th scope="col" className="px-3 py-3">名称</th><th scope="col" className="px-3 py-3">所属成员</th><th scope="col" className="px-3 py-3">权限</th><th scope="col" className="px-3 py-3">令牌</th><th scope="col" className="px-3 py-3">创建时间</th><th scope="col" className="px-3 py-3 text-right">操作</th></tr></thead>
            <tbody className="divide-y">
              {paginatedTokens.map((token) => (
                <tr key={token.id} className="hover:bg-muted/30">
                  <td className="max-w-56 break-words px-3 py-4 font-medium">{token.name}</td>
                  <td className="max-w-40 break-all px-3 py-4">{users.find((user) => user.id === token.userId)?.displayName || users.find((user) => user.id === token.userId)?.username || token.userId}{users.find((user) => user.id === token.userId)?.isActive === false && <p className="mt-1 text-xs text-muted-foreground">成员已停用</p>}</td>
                  <td className="px-3 py-4"><div className="flex flex-wrap gap-1">{token.permissions.map((permission) => <span key={permission} className="rounded bg-secondary px-2 py-0.5 text-xs">{PERMISSIONS.find((item) => item.value === permission)?.label || permission}</span>)}</div></td>
                  <td className="px-3 py-4">{token.token ? <div className="flex items-center gap-2"><code className="max-w-56 truncate select-all font-mono text-xs" title={token.token}>{token.token}</code><Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" aria-label={`复制令牌 ${token.name}`} onClick={() => handleCopy(token.token!, token.id)}>{copiedId === token.id ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}</Button></div> : <span className="text-xs text-muted-foreground">未返回令牌值</span>}</td>
                  <td className="whitespace-nowrap px-3 py-4 text-xs text-muted-foreground">{formatDate(token.createdAt)}</td>
                  <td className="px-3 py-4 text-right"><Button variant="ghost" size="icon" className="text-destructive" aria-label={`删除令牌 ${token.name}`} title="删除令牌" disabled={loading || deletingId !== null} onClick={() => handleDelete(token)}>{deletingId === token.id ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}</Button></td>
                </tr>
              ))}
              {tokens.length === 0 && <tr><td colSpan={6} className="space-y-1 py-12 text-center text-muted-foreground"><p>暂无 API 令牌</p><p className="text-xs">创建令牌以便外部应用访问</p></td></tr>}
            </tbody>
          </table>
        </div>
        <fieldset disabled={loading || submitting || deletingId !== null}><Pagination currentPage={page} totalPages={totalPages} onPageChange={setCurrentPage} totalItems={tokens.length} pageSize={PAGE_SIZE} /></fieldset>
      </div>
      <ConfirmationDialog
        open={deleteTarget !== null}
        onOpenChange={handleDeleteOpenChange}
        title="删除 API 令牌"
        description={`确定要删除 API 令牌「${deleteTarget?.name || ""}」吗？删除后使用此令牌的应用将无法访问。`}
        busy={deletingId !== null}
        error={deleteError || undefined}
        onConfirm={handleConfirmDelete}
      />


    </div>
  );
}
