import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { getSettings, updateSettings, type Settings } from "@/lib/api";
import { RefreshCw, Save, Clock, Globe } from "lucide-react";

export default function SystemSettings() {
  const [settings, setSettings] = useState<Settings>({
    syncInterval: 5,
    endpointProxy: false,
    endpointProxyUrl: "",
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  /**
   * 加载设置
   */
  const loadSettings = async () => {
    setLoading(true);
    try {
      const data = await getSettings();
      setSettings(data);
    } catch (err) {
      console.error("加载设置失败:", err);
      toast.error("加载设置失败");
    } finally {
      setLoading(false);
    }
  };

  /**
   * 保存设置
   */
  const handleSave = async () => {
    setSaving(true);
    try {
      const updated = await updateSettings(settings);
      setSettings(updated);
      toast.success("设置已保存");
    } catch (err) {
      console.error("保存设置失败:", err);
      toast.error(err instanceof Error ? err.message : "保存设置失败");
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    loadSettings();
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 同步设置 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Clock className="h-5 w-5" />
            同步设置
          </CardTitle>
          <CardDescription>
            配置账户使用量的自动同步间隔
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="syncInterval">同步间隔（分钟）</Label>
            <div className="flex items-center gap-2">
              <Input
                id="syncInterval"
                type="number"
                min="1"
                max="1440"
                value={settings.syncInterval}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    syncInterval: parseInt(e.target.value) || 5,
                  })
                }
                className="w-32"
              />
              <span className="text-sm text-muted-foreground">
                每 {settings.syncInterval} 分钟自动同步一次账户使用量
              </span>
            </div>
            <p className="text-xs text-muted-foreground">
              建议设置为 5-60 分钟，过于频繁可能影响 API 配额
            </p>
          </div>
        </CardContent>
      </Card>

      {/* 代理设置 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Globe className="h-5 w-5" />
            端点代理
          </CardTitle>
          <CardDescription>
            通过反向代理访问 R2 文件，用于解决跨域或访问限制问题
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label>启用端点代理</Label>
              <p className="text-sm text-muted-foreground">
                开启后，文件 URL 将通过代理服务器访问
              </p>
            </div>
            <Switch
              checked={settings.endpointProxy}
              onCheckedChange={(checked) =>
                setSettings({ ...settings, endpointProxy: checked })
              }
            />
          </div>

          {settings.endpointProxy && (
            <div className="space-y-2">
              <Label htmlFor="endpointProxyUrl">代理 URL</Label>
              <Input
                id="endpointProxyUrl"
                type="url"
                placeholder="https://your-domain.com"
                value={settings.endpointProxyUrl}
                onChange={(e) =>
                  setSettings({ ...settings, endpointProxyUrl: e.target.value })
                }
              />
              <p className="text-xs text-muted-foreground">
                示例：https://your-domain.com, 文件将通过 https://your-domain.com/[subdomain]/[path] 访问, 如果你没有部署外置的 endpoint-proxy 可以填程序内置的, 即 https://your-domain.com/p 或 http://localhost:port/p , 外置部署可以看项目文档。
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 保存按钮 */}
      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? (
            <>
              <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
              保存中...
            </>
          ) : (
            <>
              <Save className="mr-2 h-4 w-4" />
              保存设置
            </>
          )}
        </Button>
      </div>
    </div>
  );
}
