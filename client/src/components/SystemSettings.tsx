import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { getSettings, updateSettings, type Settings } from "@/lib/api";
import { RefreshCw, Save } from "lucide-react";

export default function SystemSettings() {
  const [settings, setSettings] = useState<Settings>({
    syncInterval: 5,
    endpointProxy: false,
    endpointProxyUrl: "",
    defaultExpirationDays: 30,
    expirationCheckMinutes: 720,
  });
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadSettings = async () => {
    setLoading(true);
    try {
      setSettings(await getSettings());
      setLoadError(false);
    } catch {
      setLoadError(true);
      toast.error("加载设置失败");
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (saving) return;
    setSaving(true);
    try {
      setSettings(await updateSettings(settings));
      toast.success("设置已保存");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "保存设置失败");
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    void Promise.resolve().then(loadSettings);
  }, []);

  if (loading) {
    return <div className="flex h-64 items-center justify-center"><RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" /></div>;
  }

  if (loadError) {
    return <div className="space-y-3 py-12 text-center text-muted-foreground"><p>系统设置加载失败</p><Button variant="outline" onClick={loadSettings}>重新加载</Button></div>;
  }

  return (
    <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); void handleSave(); }}>
      <fieldset disabled={saving} className="divide-y border-y disabled:opacity-70">
        <div className="grid gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,1fr)] sm:items-center">
          <div className="space-y-1">
            <Label htmlFor="syncInterval">同步间隔（分钟）</Label>
            <p className="text-sm text-muted-foreground">配置账户使用量的自动同步间隔，建议设置为 5–60 分钟。</p>
          </div>
          <div className="space-y-2">
            <Input id="syncInterval" type="number" min={1} max={1440} required className="w-32" value={settings.syncInterval}
              onChange={(event) => setSettings({ ...settings, syncInterval: Number(event.target.value) })} />
            <p className="text-xs text-muted-foreground">每 {settings.syncInterval} 分钟自动同步一次，过于频繁可能影响 API 配额。</p>
          </div>
        </div>

        <div className="grid gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,1fr)] sm:items-center">
          <div className="space-y-1">
            <Label htmlFor="endpointProxy">启用端点代理</Label>
            <p className="text-sm text-muted-foreground">通过反向代理访问 R2 文件，解决跨域或访问限制问题。</p>
          </div>
          <Switch id="endpointProxy" checked={settings.endpointProxy} onCheckedChange={(checked) => setSettings({ ...settings, endpointProxy: checked })} />
        </div>

        {settings.endpointProxy && (
          <div className="grid gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,1fr)] sm:items-start">
            <div className="space-y-1">
              <Label htmlFor="endpointProxyUrl">代理 URL</Label>
              <p className="text-sm text-muted-foreground">填写外置代理域名，或程序内置代理地址。</p>
            </div>
            <div className="space-y-2">
              <Input id="endpointProxyUrl" type="url" placeholder="https://your-domain.com" value={settings.endpointProxyUrl}
                onChange={(event) => setSettings({ ...settings, endpointProxyUrl: event.target.value })} />
              <p className="break-words text-xs text-muted-foreground">外置示例：https://your-domain.com。内置示例：https://your-domain.com/p 或 http://localhost:port/p。外置部署方法见项目文档。</p>
            </div>
          </div>
        )}

        <div className="grid gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,1fr)] sm:items-center">
          <div className="space-y-1">
            <Label htmlFor="defaultExpirationDays">默认文件到期时间（天）</Label>
            <p className="text-sm text-muted-foreground">设为 0 则永久保存，上传时可单独设置每个文件的到期时间。</p>
          </div>
          <div className="space-y-2">
            <Input id="defaultExpirationDays" type="number" min={0} max={3650} required className="w-32" value={settings.defaultExpirationDays}
              onChange={(event) => setSettings({ ...settings, defaultExpirationDays: Number(event.target.value) })} />
            <p className="text-xs text-muted-foreground">{settings.defaultExpirationDays === 0 ? "文件永久保存" : `文件将在上传后 ${settings.defaultExpirationDays} 天自动删除`}</p>
          </div>
        </div>

        <div className="grid gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,1fr)] sm:items-center">
          <div className="space-y-1">
            <Label htmlFor="expirationCheckMinutes">到期检查间隔（分钟）</Label>
            <p className="text-sm text-muted-foreground">配置自动清理间隔，建议为 60–720 分钟（1–12 小时）。</p>
          </div>
          <div className="space-y-2">
            <Input id="expirationCheckMinutes" type="number" min={60} max={1440} required className="w-32" value={settings.expirationCheckMinutes}
              onChange={(event) => setSettings({ ...settings, expirationCheckMinutes: Number(event.target.value) })} />
            <p className="text-xs text-muted-foreground">每 {settings.expirationCheckMinutes} 分钟检查过期文件，约 {Math.round(settings.expirationCheckMinutes / 6) / 10} 小时。</p>
          </div>
        </div>
      </fieldset>
      <div className="flex justify-end">
        <Button type="submit" disabled={saving}>
          {saving ? <RefreshCw className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
          {saving ? "保存中..." : "保存设置"}
        </Button>
      </div>
    </form>
  );
}
