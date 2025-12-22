import { useEffect, useState, useMemo } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import { Badge } from "@/components/ui/badge";
import {
  getFileExpirations,
  deleteFileExpiration,
  type FileExpiration,
} from "@/lib/api";
import { formatDate } from "@/lib/utils";
import { Trash2, RefreshCw, Clock, FileIcon, AlertTriangle } from "lucide-react";
import { toast } from "sonner";

const PAGE_SIZE = 10;

/**
 * 计算剩余时间
 */
function getTimeRemaining(expiresAt: string): { text: string; isExpired: boolean; isUrgent: boolean } {
  const now = new Date();
  const expires = new Date(expiresAt);
  const diff = expires.getTime() - now.getTime();

  if (diff <= 0) {
    return { text: "已过期", isExpired: true, isUrgent: false };
  }

  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));

  if (days > 0) {
    return { text: `${days} 天 ${hours} 小时`, isExpired: false, isUrgent: days <= 3 };
  }
  if (hours > 0) {
    return { text: `${hours} 小时`, isExpired: false, isUrgent: true };
  }
  const minutes = Math.floor(diff / (1000 * 60));
  return { text: `${minutes} 分钟`, isExpired: false, isUrgent: true };
}

/**
 * 计划删除列表管理组件
 */
export default function ScheduledDeletionsManager() {
  const [expirations, setExpirations] = useState<FileExpiration[]>([]);
  const [loading, setLoading] = useState(true);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);

  // 分页计算
  const totalPages = Math.ceil(expirations.length / PAGE_SIZE);
  const paginatedExpirations = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return expirations.slice(start, start + PAGE_SIZE);
  }, [expirations, currentPage]);

  // 当数据变化时重置页码
  useEffect(() => {
    if (currentPage > totalPages && totalPages > 0) {
      setCurrentPage(totalPages);
    }
  }, [expirations.length, totalPages, currentPage]);

  const loadExpirations = async () => {
    setLoading(true);
    try {
      const data = await getFileExpirations();
      // 按到期时间排序（最近到期的排前面）
      const sorted = (data.expirations || []).sort((a, b) =>
        new Date(a.expiresAt).getTime() - new Date(b.expiresAt).getTime()
      );
      setExpirations(sorted);
    } catch (err) {
      console.error("加载到期列表失败:", err);
      toast.error("加载到期列表失败");
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string, fileKey: string) => {
    if (!confirm(`确定要删除文件 "${fileKey}" 吗？文件将从存储中永久删除。`)) {
      return;
    }

    setDeleting(id);
    try {
      await deleteFileExpiration(id);
      await loadExpirations();
      toast.success("文件已删除");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "删除失败");
    } finally {
      setDeleting(null);
    }
  };

  useEffect(() => {
    loadExpirations();
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // 统计信息
  const expiredCount = expirations.filter(e => getTimeRemaining(e.expiresAt).isExpired).length;
  const urgentCount = expirations.filter(e => {
    const status = getTimeRemaining(e.expiresAt);
    return !status.isExpired && status.isUrgent;
  }).length;

  return (
    <div className="space-y-6">
      {/* 工具栏 */}
      <div className="flex justify-between items-center">
        <div className="text-sm text-muted-foreground space-x-4">
          <span>共 {expirations.length} 个计划删除文件</span>
          {expiredCount > 0 && (
            <Badge variant="destructive">{expiredCount} 个已过期</Badge>
          )}
          {urgentCount > 0 && (
            <Badge variant="outline" className="border-yellow-500 text-yellow-600">
              {urgentCount} 个即将删除
            </Badge>
          )}
        </div>
        <Button variant="outline" onClick={loadExpirations}>
          <RefreshCw className="mr-2 h-4 w-4" />
          刷新
        </Button>
      </div>

      {/* 列表 */}
      <div className="space-y-3">
        {expirations.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Clock className="h-12 w-12 mb-4 opacity-50" />
              <p>暂无计划删除的文件</p>
              <p className="text-sm">上传时设置有效期的文件会出现在这里</p>
            </CardContent>
          </Card>
        ) : (
          <>
            {paginatedExpirations.map((exp) => {
              const timeStatus = getTimeRemaining(exp.expiresAt);
              return (
                <Card key={exp.id} className={timeStatus.isExpired ? "border-destructive/50" : ""}>
                  <CardContent className="p-4">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex items-start gap-3 min-w-0 flex-1">
                        <div className="p-2 bg-muted rounded-lg shrink-0">
                          <FileIcon className="h-5 w-5 text-muted-foreground" />
                        </div>
                        <div className="min-w-0 flex-1 space-y-1">
                          <div className="font-medium text-sm truncate" title={exp.fileKey}>
                            {exp.fileKey}
                          </div>
                          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                            <Badge variant="secondary" className="font-normal">
                              {exp.accountName}
                            </Badge>
                            <span>创建于 {formatDate(exp.createdAt)}</span>
                          </div>
                        </div>
                      </div>

                      <div className="flex items-center gap-3 shrink-0">
                        {/* 到期状态 */}
                        <div className="text-right">
                          <div className={`text-sm font-medium flex items-center gap-1 ${
                            timeStatus.isExpired
                              ? "text-destructive"
                              : timeStatus.isUrgent
                                ? "text-yellow-600"
                                : "text-muted-foreground"
                          }`}>
                            {(timeStatus.isExpired || timeStatus.isUrgent) && (
                              <AlertTriangle className="h-4 w-4" />
                            )}
                            {timeStatus.text}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {formatDate(exp.expiresAt)}
                          </div>
                        </div>

                        {/* 删除按钮 */}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="text-destructive hover:text-destructive"
                          onClick={() => handleDelete(exp.id, exp.fileKey)}
                          disabled={deleting === exp.id}
                        >
                          {deleting === exp.id ? (
                            <RefreshCw className="h-4 w-4 animate-spin" />
                          ) : (
                            <Trash2 className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={setCurrentPage}
              totalItems={expirations.length}
              pageSize={PAGE_SIZE}
            />
          </>
        )}
      </div>
    </div>
  );
}
