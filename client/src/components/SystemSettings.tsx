import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { getSettings, updateSettings, type Settings } from "@/lib/api";
import { RefreshCw, Save, Clock, Globe, Trash2 } from "lucide-react";

export default function SystemSettings() {
  const [settings, setSettings] = useState<Settings>({
    syncInterval: 5,
    endpointProxy: false,
    endpointProxyUrl: "",
    defaultExpirationDays: 30,
    expirationCheckMinutes: 720,
    s3VirtualHostedStyle: false,
    s3BaseDomain: "",
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

      {/* S3 虚拟主机风格设置 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Globe className="h-5 w-5" />
            S3 虚拟主机风格
          </CardTitle>
          <CardDescription>
            启用 S3 Virtual Hosted Style 访问模式（需配置通配符 DNS）
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="s3VirtualHostedStyle">启用虚拟主机风格</Label>
              <p className="text-xs text-muted-foreground">
                允许使用 bucket.s3.example.com/key 格式访问（同时保留 Path Style）
              </p>
            </div>
            <Switch
              id="s3VirtualHostedStyle"
              checked={settings.s3VirtualHostedStyle}
              onCheckedChange={(checked) =>
                setSettings({ ...settings, s3VirtualHostedStyle: checked })
              }
            />
          </div>

          {settings.s3VirtualHostedStyle && (
            <div className="space-y-2">
              <Label htmlFor="s3BaseDomain">S3 基础域名</Label>
              <Input
                id="s3BaseDomain"
                type="text"
                placeholder="s3.example.com"
                value={settings.s3BaseDomain}
                onChange={(e) =>
                  setSettings({ ...settings, s3BaseDomain: e.target.value })
                }
              />
              <div className="text-xs text-muted-foreground space-y-1">
                <p>• 基础域名（不含 bucket 名称），如：s3.example.com</p>
                <p>• 需配置通配符 DNS：*.s3.example.com -&gt; 服务器 IP</p>
                <p>• 访问示例：my-bucket.s3.example.com/photo.jpg</p>
                <p>• Path Style 仍然可用：example.com/s3/my-bucket/photo.jpg</p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 文件到期设置 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Trash2 className="h-5 w-5" />
            文件到期管理
          </CardTitle>
          <CardDescription>
            配置文件默认到期时间和自动清理间隔
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="defaultExpirationDays">默认文件到期时间（天）</Label>
            <div className="flex items-center gap-2">
              <Input
                id="defaultExpirationDays"
                type="number"
                min="0"
                max="3650"
                value={settings.defaultExpirationDays}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    defaultExpirationDays: parseInt(e.target.value) || 0,
                  })
                }
                className="w-32"
              />
              <span className="text-sm text-muted-foreground">
                {settings.defaultExpirationDays === 0
                  ? "文件永久保存"
                  : `文件将在上传后 ${settings.defaultExpirationDays} 天自动删除`}
              </span>
            </div>
            <p className="text-xs text-muted-foreground">
              设置为 0 表示文件永久保存，不会自动删除；上传时可单独设置每个文件的到期时间
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="expirationCheckMinutes">到期检查间隔（分钟）</Label>
            <div className="flex items-center gap-2">
              <Input
                id="expirationCheckMinutes"
                type="number"
                min="60"
                max="1440"
                value={settings.expirationCheckMinutes}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    expirationCheckMinutes: parseInt(e.target.value) || 720,
                  })
                }
                className="w-32"
              />
              <span className="text-sm text-muted-foreground">
                每 {settings.expirationCheckMinutes} 分钟检查并删除过期文件
                （约 {Math.round(settings.expirationCheckMinutes / 60 * 10) / 10} 小时）
              </span>
            </div>
            <p className="text-xs text-muted-foreground">
              建议设置为 60-720 分钟（1-12 小时），过于频繁可能影响系统性能
            </p>
          </div>
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
