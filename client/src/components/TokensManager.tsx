import { useEffect, useState, useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Pagination } from "@/components/ui/pagination";
import {
  getTokens,
  createToken,
  deleteToken,
  type Token,
  type TokenRequest,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Plus, Trash2, RefreshCw, Copy, Check } from "lucide-react";
import { toast } from "sonner";

const PAGE_SIZE = 5;

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
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);

  // 分页计算
  const totalPages = Math.ceil(tokens.length / PAGE_SIZE);
  const paginatedTokens = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return tokens.slice(start, start + PAGE_SIZE);
  }, [tokens, currentPage]);

  // 当数据变化时重置页码
  useEffect(() => {
    if (currentPage > totalPages && totalPages > 0) {
      setCurrentPage(totalPages);
    }
  }, [tokens.length, totalPages, currentPage]);

  const loadTokens = async () => {
    setLoading(true);
    try {
      const data = await getTokens();
      setTokens(data || []);
    } catch (err) {
      console.error("加载令牌失败:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (!form.name.trim()) {
      alert("请输入令牌名称");
      return;
    }
    if (form.permissions.length === 0) {
      alert("请选择至少一个权限");
      return;
    }

    setSubmitting(true);
    try {
      await createToken(form);
      await loadTokens();
      setForm({ name: "", permissions: ["read"] });
      setShowForm(false);
      toast.success("令牌创建成功");
    } catch (err) {
      alert(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除此令牌吗？删除后使用此令牌的应用将无法访问。"))
      return;
    try {
      await deleteToken(id);
      await loadTokens();
      toast.success("令牌已删除");
    } catch (err) {
      alert(err instanceof Error ? err.message : "删除失败");
    }
  };

  const handleCopy = async (tokenValue: string, tokenId: string) => {
    await navigator.clipboard.writeText(tokenValue);
    setCopiedId(tokenId);
    toast.success("已复制到剪贴板");
    setTimeout(() => setCopiedId(null), 2000);
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
      {/* 工具栏 */}
      <div className="flex justify-between">
        <div className="text-sm text-muted-foreground">
          共 {tokens.length} 个令牌
        </div>
        <Button onClick={() => setShowForm(!showForm)}>
          <Plus className="mr-2 h-4 w-4" />
          创建令牌
        </Button>
      </div>

      {/* 表单 */}
      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle>创建 API 令牌</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>令牌名称 *</Label>
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

      {/* 令牌列表 */}
      <div className="space-y-4">
        {tokens.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <p>暂无 API 令牌</p>
              <p className="text-sm">创建令牌以便外部应用访问</p>
            </CardContent>
          </Card>
        ) : (
          <>
            {paginatedTokens.map((token) => (
              <Card key={token.id}>
                <CardContent className="p-4 space-y-3">
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
                  {/* 令牌值 */}
                  {token.token && (
                    <div className="flex items-center gap-2">
                      <code className="flex-1 p-2 bg-secondary rounded text-xs font-mono break-all select-all">
                        {token.token}
                      </code>
                      <Button
                        variant="outline"
                        size="icon"
                        className="shrink-0"
                        onClick={() => handleCopy(token.token!, token.id)}
                      >
                        {copiedId === token.id ? (
                          <Check className="h-4 w-4" />
                        ) : (
                          <Copy className="h-4 w-4" />
                        )}
                      </Button>
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={tokens.length}
              pageSize={PAGE_SIZE}
            />
          </>
        )}
      </div>
    </div>
  );
}
