import { useRef, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { useAuth } from "@/hooks/useAuth";
import { changePassword, TEAM_ROLES } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

export default function ProfileDialog({ open, onOpenChange }: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { profile, logout } = useAuth();
  const navigate = useNavigate();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const inFlight = useRef(false);

  const close = (nextOpen: boolean) => {
    if (inFlight.current) return;
    if (!nextOpen) {
      setCurrentPassword("");
      setNewPassword("");
      setConfirmation("");
      setError("");
    }
    onOpenChange(nextOpen);
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (inFlight.current) return;
    if (newPassword !== confirmation) {
      setError("两次输入的新密码不一致");
      return;
    }
    const length = new TextEncoder().encode(newPassword).length;
    if (length < 8 || length > 72) {
      setError("新密码需为 8–72 字节，中文字符会占用多个字节");
      return;
    }
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      await changePassword(currentPassword, newPassword);
      toast.success("密码已修改，请使用新密码重新登录");
      await logout().catch(() => undefined);
      onOpenChange(false);
      navigate("/login", { replace: true });
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "修改密码失败，请重试");
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-md"
        onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); }}
        onPointerDownOutside={(event) => { if (busy) event.preventDefault(); }}>
        <DialogHeader>
          <DialogTitle>个人信息</DialogTitle>
          <DialogDescription>查看当前成员身份，修改登录密码。</DialogDescription>
        </DialogHeader>
        {profile && <dl className="divide-y border-y text-sm">
          <div className="flex justify-between gap-4 py-3"><dt className="text-muted-foreground">用户名</dt><dd className="break-all text-right">{profile.username}</dd></div>
          <div className="flex justify-between gap-4 py-3"><dt className="text-muted-foreground">显示名称</dt><dd className="break-all text-right">{profile.displayName || profile.username}</dd></div>
          <div className="flex justify-between gap-4 py-3"><dt className="text-muted-foreground">角色</dt><dd>{TEAM_ROLES[profile.role].label}</dd></div>
        </dl>}
        <form onSubmit={(event) => { void submit(event); }} className="space-y-4">
          <fieldset disabled={busy} className="space-y-4">
            <div className="space-y-2"><Label htmlFor="profile-current-password">当前密码</Label><Input id="profile-current-password" type="password" autoComplete="current-password" required value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></div>
            <div className="space-y-2"><Label htmlFor="profile-new-password">新密码</Label><Input id="profile-new-password" type="password" autoComplete="new-password" required minLength={8} maxLength={72} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></div>
            <div className="space-y-2"><Label htmlFor="profile-confirm-password">确认新密码</Label><Input id="profile-confirm-password" type="password" autoComplete="new-password" required minLength={8} maxLength={72} value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></div>
            <p className="text-xs text-muted-foreground">修改密码后，此账号在所有设备上的登录会话都会退出。</p>
          </fieldset>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter><Button type="button" variant="outline" disabled={busy} onClick={() => close(false)}>关闭</Button><Button type="submit" disabled={busy}>{busy ? "保存中…" : "修改密码"}</Button></DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
