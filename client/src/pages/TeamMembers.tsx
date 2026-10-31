import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Plus, RefreshCw, Pencil, KeyRound, Users, History } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { createUser, getUsers, updateUser, getAuditLogs, TEAM_ROLES, type AuditEntry, type TeamRole, type UserProfile } from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Pagination } from "@/components/ui/pagination";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import ConfirmationDialog from "@/components/ConfirmationDialog";

const PAGE_SIZE = 10;
type MemberForm = { mode: "create" | "edit" | "password"; target?: UserProfile };

export default function TeamMembers() {
  const { profile } = useAuth();
  const [tab, setTab] = useState<"members" | "activity">("members");
  const [users, setUsers] = useState<UserProfile[]>([]);
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [page, setPage] = useState(1);
  const [form, setForm] = useState<MemberForm | null>(null);
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState<TeamRole>("member");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [formError, setFormError] = useState("");
  const [busy, setBusy] = useState(false);
  const [toggleTarget, setToggleTarget] = useState<UserProfile | null>(null);
  const [toggleError, setToggleError] = useState("");
  const requestSequence = useRef(0);
  const inFlight = useRef(false);

  const load = useCallback(async () => {
    const sequence = ++requestSequence.current;
    setLoading(true);
    setLoadError("");
    try {
      if (tab === "members") {
        const data = await getUsers();
        if (sequence === requestSequence.current) setUsers(data);
      } else {
        const data = await getAuditLogs();
        if (sequence === requestSequence.current) setEntries(data);
      }
    } catch (failure) {
      if (sequence === requestSequence.current) setLoadError(failure instanceof Error ? failure.message : "加载失败，请重试");
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  }, [tab]);

  useEffect(() => {
    void Promise.resolve().then(load);
    return () => { requestSequence.current += 1; };
  }, [load]);

  const canEdit = (target: UserProfile) => target.role === "owner"
    ? target.id === profile?.id
    : profile?.role === "owner" || target.role === "member" || target.role === "viewer";
  const openForm = (mode: MemberForm["mode"], target?: UserProfile) => {
    if (inFlight.current) return;
    setUsername(target?.username || "");
    setDisplayName(target?.displayName || "");
    setRole(target?.role || "member");
    setPassword("");
    setConfirmation("");
    setFormError("");
    setForm({ mode, target });
  };

  const closeForm = () => {
    if (inFlight.current) return;
    setForm(null);
    setPassword("");
    setConfirmation("");
    setFormError("");
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!form || inFlight.current) return;
    if (form.mode !== "edit") {
      const length = new TextEncoder().encode(password).length;
      if (length < 8 || length > 72) { setFormError("密码需为 8–72 字节，中文字符会占用多个字节"); return; }
      if (password !== confirmation) { setFormError("两次输入的密码不一致"); return; }
    }
    inFlight.current = true;
    setBusy(true);
    setFormError("");
    try {
      if (form.mode === "create") {
        if (role === "owner") throw new Error("无法创建新的所有者");
        await createUser({ username: username.trim(), displayName: displayName.trim(), password, role });
      } else if (form.target) {
        await updateUser(form.target.id, {
          displayName: form.mode === "password" ? form.target.displayName : displayName.trim(),
          role: form.mode === "password" ? form.target.role : role,
          isActive: form.target.isActive,
          ...(form.mode === "password" ? { password } : {}),
        });
      }
      toast.success(form.mode === "create" ? "成员已添加" : form.mode === "password" ? "密码已重置，该成员需要重新登录" : "成员信息已更新");
      setForm(null);
      setPassword("");
      setConfirmation("");
      await load();
    } catch (failure) {
      setFormError(failure instanceof Error ? failure.message : "保存失败，请重试");
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  };

  const toggle = async () => {
    if (!toggleTarget || inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setToggleError("");
    try {
      await updateUser(toggleTarget.id, {
        displayName: toggleTarget.displayName, role: toggleTarget.role, isActive: !toggleTarget.isActive,
      });
      toast.success(toggleTarget.isActive ? "成员已停用" : "成员已启用");
      setToggleTarget(null);
      await load();
    } catch (failure) {
      setToggleError(failure instanceof Error ? failure.message : "更新失败，请重试");
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  };

  const total = tab === "members" ? users.length : entries.length;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const visiblePage = Math.min(page, totalPages);
  const start = (visiblePage - 1) * PAGE_SIZE;
  const formTitle = form?.mode === "create" ? "添加成员" : form?.mode === "password" ? "重置成员密码" : "编辑成员";

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div><h1 className="text-2xl font-bold sm:text-3xl">团队成员</h1><p className="mt-1 text-sm text-muted-foreground">成员共享全部文件，角色决定读写和管理权限。</p></div>
        <div className="flex gap-2"><Button variant="outline" disabled={loading || busy} onClick={() => { void load(); }}><RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />刷新</Button>{tab === "members" && <Button disabled={loading || busy} onClick={() => openForm("create")}><Plus className="mr-2 h-4 w-4" />添加成员</Button>}</div>
      </div>
      <div className="flex gap-2 border-b pb-2" role="tablist" aria-label="团队管理">
        <Button role="tab" aria-selected={tab === "members"} variant={tab === "members" ? "default" : "ghost"} disabled={busy} onClick={() => { setTab("members"); setPage(1); }}><Users className="mr-2 h-4 w-4" />成员列表</Button>
        <Button role="tab" aria-selected={tab === "activity"} variant={tab === "activity" ? "default" : "ghost"} disabled={busy} onClick={() => { setTab("activity"); setPage(1); }}><History className="mr-2 h-4 w-4" />操作记录</Button>
      </div>
      {loadError && <div role="alert" className="flex items-center justify-between gap-3 text-sm text-destructive"><span>{loadError}</span><Button variant="outline" size="sm" disabled={loading} onClick={() => { void load(); }}>重试</Button></div>}
      <div className="overflow-x-auto border-y" aria-busy={loading}>
        {tab === "members" ? <table className="w-full min-w-[720px] text-left text-sm">
          <caption className="sr-only">团队成员列表</caption>
          <thead className="border-b bg-muted/40 text-muted-foreground"><tr><th scope="col" className="px-3 py-3">成员</th><th scope="col" className="px-3 py-3">角色 / 权限</th><th scope="col" className="px-3 py-3">状态</th><th scope="col" className="px-3 py-3">加入时间</th><th scope="col" className="px-3 py-3 text-right">操作</th></tr></thead>
          <tbody className="divide-y">{users.slice(start, start + PAGE_SIZE).map((member) => <tr key={member.id} className="hover:bg-muted/30">
            <td className="max-w-56 px-3 py-4"><p className="break-all font-medium">{member.displayName || member.username}{member.id === profile?.id && <span className="ml-2 text-xs text-muted-foreground">你</span>}</p><p className="mt-1 break-all text-xs text-muted-foreground">{member.username}</p></td>
            <td className="px-3 py-4"><span className="rounded bg-secondary px-2 py-1 text-xs">{TEAM_ROLES[member.role].label}</span><p className="mt-2 text-xs text-muted-foreground">{TEAM_ROLES[member.role].description}</p></td>
            <td className="px-3 py-4"><div className="flex items-center gap-2"><Switch aria-label={`${member.isActive ? "停用" : "启用"}成员 ${member.username}`} checked={member.isActive} disabled={busy || loading || member.role === "owner" || !canEdit(member)} onCheckedChange={() => { setToggleError(""); setToggleTarget(member); }} /><span className="text-xs">{member.isActive ? "已启用" : "已停用"}</span></div></td>
            <td className="whitespace-nowrap px-3 py-4 text-xs text-muted-foreground">{formatDate(member.createdAt)}</td>
            <td className="px-3 py-4 text-right">{canEdit(member) ? <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" title="编辑成员" aria-label={`编辑成员 ${member.username}`} disabled={busy || loading} onClick={() => openForm("edit", member)}><Pencil className="h-4 w-4" /></Button><Button variant="ghost" size="icon" title="重置密码" aria-label={`重置 ${member.username} 的密码`} disabled={busy || loading || member.id === profile?.id} onClick={() => openForm("password", member)}><KeyRound className="h-4 w-4" /></Button></div> : <span className="text-xs text-muted-foreground">{member.role === "owner" ? "所有者" : "由所有者管理"}</span>}</td>
          </tr>)}{users.length === 0 && <tr><td colSpan={5} className="py-12 text-center text-muted-foreground">{loading ? "正在加载成员…" : "暂无成员"}</td></tr>}</tbody>
        </table> : <table className="w-full min-w-[760px] text-left text-sm">
          <caption className="sr-only">团队操作记录</caption>
          <thead className="border-b bg-muted/40 text-muted-foreground"><tr>{["操作人", "操作", "对象", "结果", "时间"].map((label) => <th key={label} scope="col" className="px-3 py-3">{label}</th>)}</tr></thead>
          <tbody className="divide-y">{entries.slice(start, start + PAGE_SIZE).map((entry) => <tr key={entry.id} className="hover:bg-muted/30"><td className="max-w-40 break-all px-3 py-4">{entry.username || "系统"}</td><td className="px-3 py-4">{entry.action}</td><td className="max-w-80 break-all px-3 py-4 text-xs">{entry.target}</td><td className="px-3 py-4">{entry.status >= 200 && entry.status < 400 ? <span>成功</span> : <span className="text-destructive">失败（{entry.status}）</span>}</td><td className="whitespace-nowrap px-3 py-4 text-xs text-muted-foreground">{formatDate(entry.createdAt)}</td></tr>)}{entries.length === 0 && <tr><td colSpan={5} className="py-12 text-center text-muted-foreground">{loading ? "正在加载记录…" : "暂无操作记录"}</td></tr>}</tbody>
        </table>}
      </div>
      <fieldset disabled={busy || loading}><Pagination currentPage={visiblePage} totalPages={totalPages} onPageChange={setPage} totalItems={total} pageSize={PAGE_SIZE} /></fieldset>
      {tab === "activity" && <p className="text-xs text-muted-foreground">显示最近 100 条操作记录。</p>}
      <Dialog open={form !== null} onOpenChange={(open) => { if (!open) closeForm(); }}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg" onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); }} onPointerDownOutside={(event) => { if (busy) event.preventDefault(); }}>
          <DialogHeader><DialogTitle>{formTitle}</DialogTitle><DialogDescription>{form?.mode === "password" ? `为 ${form.target?.displayName || form.target?.username} 设置新密码。重置后该成员的所有登录会话会失效。` : "为成员设置独立登录账号和团队角色。"}</DialogDescription></DialogHeader>
          <form onSubmit={(event) => { void submit(event); }} className="space-y-4"><fieldset disabled={busy} className="space-y-4">
            {form?.mode !== "password" && <>
              <div className="space-y-2"><Label htmlFor="member-username">用户名</Label><Input id="member-username" autoComplete="off" required maxLength={64} disabled={form?.mode === "edit"} value={username} onChange={(event) => setUsername(event.target.value)} /></div>
              <div className="space-y-2"><Label htmlFor="member-display-name">显示名称</Label><Input id="member-display-name" maxLength={100} value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="留空时显示用户名" /></div>
              <div className="space-y-2"><Label htmlFor="member-role">角色</Label><select id="member-role" className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" value={role} disabled={form?.target?.role === "owner"} onChange={(event) => setRole(event.target.value as TeamRole)}>{role === "owner" && <option value="owner">所有者</option>}{profile?.role === "owner" && <option value="admin">管理员</option>}<option value="member">可读写成员</option><option value="viewer">只读成员</option></select><p className="text-xs text-muted-foreground">{TEAM_ROLES[role].description}</p></div>
            </>}
            {form?.mode !== "edit" && <><div className="space-y-2"><Label htmlFor="member-password">{form?.mode === "password" ? "新密码" : "初始密码"}</Label><Input id="member-password" autoComplete="new-password" type="password" required minLength={8} maxLength={72} value={password} onChange={(event) => setPassword(event.target.value)} /></div><div className="space-y-2"><Label htmlFor="member-confirm-password">确认密码</Label><Input id="member-confirm-password" autoComplete="new-password" type="password" required minLength={8} maxLength={72} value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></div></>}
          </fieldset>{formError && <p role="alert" className="text-sm text-destructive">{formError}</p>}<DialogFooter><Button type="button" variant="outline" disabled={busy} onClick={closeForm}>取消</Button><Button type="submit" disabled={busy}>{busy ? "保存中…" : "保存"}</Button></DialogFooter></form>
        </DialogContent>
      </Dialog>
      <ConfirmationDialog open={toggleTarget !== null} onOpenChange={(open) => { if (!open && !inFlight.current) { setToggleTarget(null); setToggleError(""); } }} title={toggleTarget?.isActive ? "停用成员？" : "启用成员？"} description={toggleTarget?.isActive ? `停用 ${toggleTarget.displayName || toggleTarget.username} 后，该成员将无法登录，其 API Token 和 WebDAV 凭证也将停止访问。` : `启用 ${toggleTarget?.displayName || toggleTarget?.username || ""} 后，该成员可以重新登录。`} confirmLabel={toggleTarget?.isActive ? "停用成员" : "启用成员"} destructive={!!toggleTarget?.isActive} busy={busy} error={toggleError || undefined} onConfirm={toggle} />
    </div>
  );
}
