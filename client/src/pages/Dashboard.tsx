import { useEffect, useState, useCallback } from "react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Pagination } from "@/components/ui/pagination";
import {
  getAccountsPaged,
  getAccountsStats,
  syncAccounts,
  type AccountFull,
  type AccountsStats,
} from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/utils";
import {
  HardDrive,
  Upload,
  Download,
  RefreshCw,
  CheckCircle,
  XCircle,
  AlertTriangle,
} from "lucide-react";

const PAGE_SIZE = 6;

export default function Dashboard() {
  const [accounts, setAccounts] = useState<AccountFull[]>([]);
  const [stats, setStats] = useState<AccountsStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [syncingAccountId, setSyncingAccountId] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [totalItems, setTotalItems] = useState(0);

  const loadStats = useCallback(async () => {
    try {
      const data = await getAccountsStats();
      setStats(data);
    } catch (err) {
      console.error("加载统计信息失败:", err);
    }
  }, []);

  const loadAccounts = useCallback(async (page: number) => {
    try {
      const data = await getAccountsPaged(page, PAGE_SIZE);
      setAccounts(data.items || []);
      setTotalPages(data.totalPages);
      setTotalItems(data.total);
      setCurrentPage(data.page);
    } catch (err) {
      console.error("加载账户失败:", err);
    }
  }, []);

  const loadData = useCallback(async (page: number = currentPage) => {
    setLoading(true);
    try {
      await Promise.all([loadStats(), loadAccounts(page)]);
    } finally {
      setLoading(false);
    }
  }, [loadStats, loadAccounts, currentPage]);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    loadAccounts(page);
  };

  const handleSync = async () => {
    setSyncing(true);
    try {
      await syncAccounts();
      await loadData(currentPage);
      toast.success("同步成功");
    } catch (err) {
      console.error("同步失败:", err);
      toast.error(err instanceof Error ? err.message : "同步失败");
    } finally {
      setSyncing(false);
    }
  };

  const handleSyncAccount = async (accountId: string) => {
    setSyncingAccountId(accountId);
    try {
      await syncAccounts(accountId);
      await loadData(currentPage);
      toast.success("同步成功");
    } catch (err) {
      console.error("同步失败:", err);
      toast.error(err instanceof Error ? err.message : "同步失败");
    } finally {
      setSyncingAccountId(null);
    }
  };

  useEffect(() => {
    loadData(1);
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">仪表盘</h1>
          <p className="text-muted-foreground">R2 存储账户概览</p>
        </div>
        <Button onClick={handleSync} disabled={syncing}>
          <RefreshCw
            className={`mr-2 h-4 w-4 ${syncing ? "animate-spin" : ""}`}
          />
          {syncing ? "同步中..." : "立即同步"}
        </Button>
      </div>

      {/* 统计卡片 */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">总存储使用</CardTitle>
            <HardDrive className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatBytes(stats?.totalSizeBytes || 0)}</div>
            <p className="text-xs text-muted-foreground">
              总配额 {formatBytes(stats?.totalQuotaBytes || 0)}
            </p>
            <Progress
              value={stats?.totalSizeBytes || 0}
              max={stats?.totalQuotaBytes || 1}
              className="mt-2"
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">写入操作</CardTitle>
            <Upload className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatNumber(stats?.totalWriteOps || 0)}</div>
            <p className="text-xs text-muted-foreground">本月累计（Class A）</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">读取操作</CardTitle>
            <Download className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{formatNumber(stats?.totalReadOps || 0)}</div>
            <p className="text-xs text-muted-foreground">本月累计（Class B）</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">可用账户</CardTitle>
            <CheckCircle className="h-4 w-4 text-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.availableCount || 0} / {stats?.totalAccounts || 0}
            </div>
            <p className="text-xs text-muted-foreground">可正常上传的账户</p>
          </CardContent>
        </Card>
      </div>

      {/* 账户列表 */}
      <div>
        <h2 className="text-xl font-semibold mb-4">账户状态</h2>
        {totalItems === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <HardDrive className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-muted-foreground">暂无账户，请前往设置添加</p>
            </CardContent>
          </Card>
        ) : (
          <>
            <div className="grid gap-4 md:grid-cols-2">
              {accounts.map((account) => (
                <Card key={account.id} className={!account.isActive ? "opacity-50 border-muted" : ""}>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-lg">{account.name}</CardTitle>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7"
                          onClick={() => handleSyncAccount(account.id)}
                          disabled={syncingAccountId === account.id}
                        >
                          <RefreshCw
                            className={`h-4 w-4 ${
                              syncingAccountId === account.id ? "animate-spin" : ""
                            }`}
                          />
                        </Button>
                        <StatusBadge account={account} />
                      </div>
                    </div>
                    <CardDescription>{account.bucketName}</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {/* 容量使用 */}
                    <div>
                      <div className="flex justify-between text-sm mb-1">
                        <span>容量使用</span>
                        <span>
                          {formatBytes(account.usage.sizeBytes)} /{" "}
                          {formatBytes(account.quota.maxSizeBytes)}
                        </span>
                      </div>
                      <Progress
                        value={account.usage.sizeBytes}
                        max={account.quota.maxSizeBytes}
                      />
                    </div>

                    {/* 写入操作 */}
                    <div>
                      <div className="flex justify-between text-sm mb-1">
                        <span>写入操作</span>
                        <span>
                          {formatNumber(account.usage.classAOps)} /{" "}
                          {formatNumber(account.quota.maxClassAOps)}
                        </span>
                      </div>
                      <Progress
                        value={account.usage.classAOps}
                        max={account.quota.maxClassAOps}
                      />
                    </div>

                    {/* 读取操作 */}
                    <div>
                      <div className="flex justify-between text-sm mb-1">
                        <span>读取操作</span>
                        <span>{formatNumber(account.usage.classBOps || 0)}</span>
                      </div>
                    </div>

                    {/* 状态信息 */}
                    <div className="flex items-center gap-4 text-xs text-muted-foreground">
                      <span>
                        使用率: {account.usagePercent.toFixed(1)}%
                      </span>
                      {account.usage.lastSyncAt && (
                        <span>
                          上次同步:{" "}
                          {new Date(account.usage.lastSyncAt).toLocaleString(
                            "zh-CN"
                          )}
                        </span>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={handlePageChange}
              totalItems={totalItems}
              pageSize={PAGE_SIZE}
            />
          </>
        )}
      </div>
    </div>
  );
}

/**
 * 账户状态徽章
 */
function StatusBadge({ account }: { account: AccountFull }) {
  if (!account.isActive) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-destructive px-2 py-1 text-xs text-destructive-foreground">
        <XCircle className="h-3 w-3" />
        已禁用
      </span>
    );
  }

  if (account.isOverQuota) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-foreground px-2 py-1 text-xs text-background">
        <XCircle className="h-3 w-3" />
        容量超限
      </span>
    );
  }

  if (account.isOverOps) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-muted-foreground px-2 py-1 text-xs text-background">
        <AlertTriangle className="h-3 w-3" />
        操作超限
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-secondary px-2 py-1 text-xs text-secondary-foreground border">
      <CheckCircle className="h-3 w-3" />
      正常
    </span>
  );
}
