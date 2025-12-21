import { useEffect, useState, useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  type WebDAVCredential,
  type WebDAVCredentialRequest,
  type AccountFull,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Plus, Trash2, RefreshCw, Copy, Check, FolderTree } from "lucide-react";
import { toast } from "sonner";

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
  const [credentials, setCredentials] = useState<WebDAVCredential[]>([]);
  const [accounts, setAccounts] = useState<AccountFull[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<WebDAVCredentialRequest>({
    accountId: "",
    description: "",
    permissions: ["read", "write"],
    username: "",
    password: "",
  });
  const [submitting, setSubmitting] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);

  // 分页计算
  const totalPages = Math.ceil(credentials.length / PAGE_SIZE);
  const paginatedCredentials = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return credentials.slice(start, start + PAGE_SIZE);
  }, [credentials, currentPage]);

  // 当数据变化时重置页码
  useEffect(() => {
    if (currentPage > totalPages && totalPages > 0) {
      setCurrentPage(totalPages);
    }
  }, [credentials.length, totalPages, currentPage]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [credData, accData] = await Promise.all([
        getWebDAVCredentials(),
        getAccounts(),
      ]);
      setCredentials(credData || []);
      setAccounts(accData || []);
    } catch (err) {
      console.error("加载数据失败:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (!form.accountId) {
      alert("请选择关联账户");
      return;
    }
    if (form.permissions.length === 0) {
      alert("请选择至少一个权限");
      return;
    }

    setSubmitting(true);
    try {
      await createWebDAVCredential(form);
      await loadData();
      setForm({
        accountId: "",
        description: "",
        permissions: ["read", "write"],
        username: "",
        password: "",
      });
      setShowForm(false);
      toast.success("WebDAV 凭证创建成功");
    } catch (err) {
      alert(err instanceof Error ? err.message : "创建失败");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleActive = async (cred: WebDAVCredential) => {
    try {
      await updateWebDAVCredential(cred.id, {
        description: cred.description,
        permissions: cred.permissions,
        isActive: !cred.isActive,
      });
      await loadData();
      toast.success(cred.isActive ? "凭证已停用" : "凭证已启用");
    } catch (err) {
      alert(err instanceof Error ? err.message : "更新失败");
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除此 WebDAV 凭证吗？删除后使用此凭证的客户端将无法访问。")) return;
    try {
      await deleteWebDAVCredential(id);
      await loadData();
      toast.success("WebDAV 凭证已删除");
    } catch (err) {
      alert(err instanceof Error ? err.message : "删除失败");
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
    loadData();
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
          共 {credentials.length} 个凭证
        </div>
        <Button onClick={() => setShowForm(!showForm)}>
          <Plus className="mr-2 h-4 w-4" />
          创建凭证
        </Button>
      </div>

      {/* 创建表单 */}
      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle>创建 WebDAV 凭证</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>关联账户 *</Label>
              <select
                value={form.accountId}
                onChange={(e) => setForm({ ...form, accountId: e.target.value })}
                className="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
              >
                <option value="">选择存储账户</option>
                {accounts.map((acc) => (
                  <option key={acc.id} value={acc.id}>
                    {acc.name} ({acc.bucketName})
                  </option>
                ))}
              </select>
              <p className="text-xs text-muted-foreground">
                凭证将绑定到此账户，访问路径为 /webdav/
              </p>
            </div>
            <div className="space-y-2">
              <Label>描述</Label>
              <Input
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="例如: Windows 文件资源管理器"
              />
            </div>
            <div className="space-y-2">
              <Label>用户名</Label>
              <Input
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                placeholder="留空自动生成（格式: FFLW_WebDAV_XXXXXXXX）"
              />
              <p className="text-xs text-muted-foreground">
                可自定义用户名，留空将自动生成
              </p>
            </div>
            <div className="space-y-2">
              <Label>密码</Label>
              <Input
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
                {PERMISSIONS.filter((p) => form.permissions.includes(p.value))
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

      {/* 凭证列表 */}
      <div className="space-y-4">
        {credentials.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <FolderTree className="h-12 w-12 mb-4 opacity-50" />
              <p>暂无 WebDAV 凭证</p>
              <p className="text-sm">创建凭证以使用 WebDAV 客户端访问</p>
            </CardContent>
          </Card>
        ) : (
          <>
            {paginatedCredentials.map((cred) => (
              <Card key={cred.id} className={!cred.isActive ? "opacity-60" : ""}>
                <CardContent className="p-4 space-y-3">
                  <div className="flex items-start justify-between">
                    <div className="space-y-1 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{getAccountName(cred.accountId)}</span>
                        <code className="text-xs bg-secondary px-2 py-0.5 rounded">
                          {getBucketName(cred.accountId)}
                        </code>
                        {!cred.isActive && (
                          <span className="text-xs text-destructive">(已停用)</span>
                        )}
                      </div>
                      {cred.description && (
                        <p className="text-sm text-muted-foreground">{cred.description}</p>
                      )}
                      <div className="flex gap-1 flex-wrap">
                        {cred.permissions.map((p) => (
                          <span
                            key={p}
                            className="text-xs bg-secondary px-2 py-0.5 rounded"
                          >
                            {PERMISSIONS.find((x) => x.value === p)?.label || p}
                          </span>
                        ))}
                      </div>
                      <div className="text-xs text-muted-foreground space-x-4">
                        <span>创建于 {formatDate(cred.createdAt)}</span>
                        {cred.lastUsedAt && (
                          <span>最后使用 {formatDate(cred.lastUsedAt)}</span>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Switch
                        checked={cred.isActive}
                        onCheckedChange={() => handleToggleActive(cred)}
                      />
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive"
                        onClick={() => handleDelete(cred.id)}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                  {/* Username */}
                  <div className="flex items-center gap-2">
                    <Label className="text-xs w-24 shrink-0">用户名:</Label>
                    <code className="flex-1 p-2 bg-secondary rounded text-xs font-mono break-all select-all">
                      {cred.username}
                    </code>
                    <Button
                      variant="outline"
                      size="icon"
                      className="shrink-0"
                      onClick={() => handleCopy(cred.username, cred.id + "-user")}
                    >
                      {copiedField === cred.id + "-user" ? (
                        <Check className="h-4 w-4" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </Button>
                  </div>
                  {/* Password */}
                  <div className="flex items-center gap-2">
                    <Label className="text-xs w-24 shrink-0">密码:</Label>
                    <code className="flex-1 p-2 bg-secondary rounded text-xs font-mono break-all select-all">
                      {cred.password}
                    </code>
                    <Button
                      variant="outline"
                      size="icon"
                      className="shrink-0"
                      onClick={() => handleCopy(cred.password, cred.id + "-pass")}
                    >
                      {copiedField === cred.id + "-pass" ? (
                        <Check className="h-4 w-4" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={credentials.length}
              pageSize={PAGE_SIZE}
            />
          </>
        )}
      </div>
    </div>
  );
}
