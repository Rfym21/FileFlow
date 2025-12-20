import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  getTokens,
  createToken,
  deleteToken,
  type Token,
  type TokenRequest,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Plus, Trash2, RefreshCw, X, Copy, Check } from "lucide-react";

const PERMISSIONS = [
  { value: "read", label: "读取", desc: "列表、下载、获取链接" },
  { value: "write", label: "写入", desc: "上传文件" },
  { value: "delete", label: "删除", desc: "删除文件" },
];

export default function TokensManager() {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<TokenRequest>({
    name: "",
    permissions: ["read"],
  });
  const [submitting, setSubmitting] = useState(false);
  const [newToken, setNewToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const loadTokens = async () => {
    setLoading(true);
    try {
      const data = await getTokens();
      setTokens(data || []);
    } catch (err) {
      console.error("加载 Token 失败:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      alert("请输入 Token 名称");
      return;
    }
    if (form.permissions.length === 0) {
      alert("请选择至少一个权限");
      return;
    }

    setSubmitting(true);
    try {
      const result = await createToken(form);
      setNewToken(result.token || null);
      await loadTokens();
      setForm({ name: "", permissions: ["read"] });
    } catch (err) {
      alert(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除此 Token 吗？删除后使用此 Token 的应用将无法访问。"))
      return;
    try {
      await deleteToken(id);
      await loadTokens();
    } catch (err) {
      alert(err instanceof Error ? err.message : "删除失败");
    }
  };

  const handleCopy = async () => {
    if (newToken) {
      await navigator.clipboard.writeText(newToken);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
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

  useEffect(() => {
    loadTokens();
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
      {/* 新创建的 Token 显示 */}
      {newToken && (
        <Card className="border-foreground/20 bg-secondary">
          <CardContent className="p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="font-medium">Token 创建成功</p>
                <p className="text-sm text-muted-foreground">
                  请立即复制，此 Token 仅显示一次
                </p>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setNewToken(null)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="mt-2 flex items-center gap-2">
              <code className="flex-1 p-2 bg-background rounded border text-sm font-mono break-all">
                {newToken}
              </code>
              <Button variant="outline" size="icon" onClick={handleCopy}>
                {copied ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* 工具栏 */}
      <div className="flex justify-between">
        <div className="text-sm text-muted-foreground">
          共 {tokens.length} 个 Token
        </div>
        <Button onClick={() => setShowForm(!showForm)}>
          <Plus className="mr-2 h-4 w-4" />
          创建 Token
        </Button>
      </div>

      {/* 表单 */}
      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle>创建 API Token</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>Token 名称 *</Label>
              <Input
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
                    onClick={() => togglePermission(perm.value)}
                    className={`px-3 py-1.5 rounded-md text-sm border transition-colors ${
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
            </div>
            <div className="flex gap-2">
              <Button onClick={handleSubmit} disabled={submitting}>
                {submitting ? "创建中..." : "创建"}
              </Button>
              <Button variant="outline" onClick={() => setShowForm(false)}>
                取消
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Token 列表 */}
      <div className="space-y-4">
        {tokens.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <p>暂无 API Token</p>
              <p className="text-sm">创建 Token 以便外部应用访问</p>
            </CardContent>
          </Card>
        ) : (
          tokens.map((token) => (
            <Card key={token.id}>
              <CardContent className="p-4">
                <div className="flex items-start justify-between">
                  <div className="space-y-1">
                    <div className="font-medium">{token.name}</div>
                    <div className="flex gap-1">
                      {token.permissions.map((p) => (
                        <span
                          key={p}
                          className="text-xs bg-secondary px-2 py-0.5 rounded"
                        >
                          {PERMISSIONS.find((x) => x.value === p)?.label || p}
                        </span>
                      ))}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      创建于 {formatDate(token.createdAt)}
                    </div>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-destructive"
                    onClick={() => handleDelete(token.id)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>
    </div>
  );
}
