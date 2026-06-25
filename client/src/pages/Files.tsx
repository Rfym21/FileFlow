import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import {
  getAccountsPaged,
  getImgBBFiles,
  type AccountFull,
  type ImgBBFile,
} from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import {
  FolderOpen,
  Upload,
  RefreshCw,
  ChevronRight,
  ChevronUp,
  ChevronDown,
  Image as ImageIcon,
} from "lucide-react";
import FileUploader from "@/components/FileUploader";

const PAGE_SIZE = 9;

export default function Files() {
  const navigate = useNavigate();
  const [accounts, setAccounts] = useState<AccountFull[]>([]);
  const [imgbbFiles, setImgbbFiles] = useState<ImgBBFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [showUploader, setShowUploader] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [totalItems, setTotalItems] = useState(0);

  const loadAccounts = useCallback(async (page: number = currentPage) => {
    setLoading(true);
    try {
      const data = await getAccountsPaged(page, PAGE_SIZE);
      setAccounts(data.items || []);
      setTotalPages(data.totalPages);
      setTotalItems(data.total);
      setCurrentPage(data.page);
    } catch (err) {
      console.error("加载账户失败:", err);
      toast.error("加载账户失败");
    } finally {
      setLoading(false);
    }
  }, [currentPage]);

  const loadImgBBFiles = useCallback(async () => {
    try {
      const files = await getImgBBFiles();
      setImgbbFiles(files);
    } catch (err) {
      console.error("加载 ImgBB 文件失败:", err);
    }
  }, []);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    loadAccounts(page);
  };

  useEffect(() => {
    loadAccounts(1);
    loadImgBBFiles();
  }, []);

  return (
    <div className="space-y-4 sm:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl sm:text-3xl font-bold">文件管理</h1>
          <p className="text-sm sm:text-base text-muted-foreground">浏览和管理 R2 存储中的文件</p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadAccounts(currentPage)}
            disabled={loading}
            className="flex-1 sm:flex-none"
          >
            <RefreshCw
              className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`}
            />
            <span className="hidden sm:inline">刷新</span>
          </Button>
          <Button
            size="sm"
            onClick={() => setShowUploader(!showUploader)}
            className="flex-1 sm:flex-none"
          >
            <Upload className="mr-2 h-4 w-4" />
            <span className="hidden sm:inline">上传文件</span>
            <span className="sm:hidden">上传</span>
            {showUploader ? (
              <ChevronUp className="ml-1 h-4 w-4" />
            ) : (
              <ChevronDown className="ml-1 h-4 w-4" />
            )}
          </Button>
        </div>
      </div>

      {/* 上传区域 */}
      {showUploader && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base sm:text-lg">上传文件</CardTitle>
          </CardHeader>
          <CardContent>
            <FileUploader onUploadComplete={() => {
              loadAccounts(currentPage);
              loadImgBBFiles();
            }} />
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <>
          <div className="grid gap-3 sm:gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
            {/* ImgBB 卡片 - 固定在第一位 */}
            <Card
              className="border-2 border-blue-500/50 bg-gradient-to-br from-blue-50 to-white dark:from-blue-950/20 dark:to-background cursor-pointer hover:shadow-lg transition-shadow"
              onClick={() => navigate("/files/imgbb")}
            >
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
                  <ImageIcon className="h-4 w-4 sm:h-5 sm:w-5 text-blue-600 flex-shrink-0" />
                  <span className="truncate">ImgBB 图床</span>
                  <span className="ml-auto text-xs text-muted-foreground">{imgbbFiles.length} 个文件</span>
                  <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                </CardTitle>
              </CardHeader>
              <CardContent className="pt-0">
                {imgbbFiles.length === 0 ? (
                  <p className="text-sm text-muted-foreground text-center py-4">
                    暂无文件
                  </p>
                ) : (
                  <div className="space-y-2">
                    {imgbbFiles.slice(0, 3).map((file) => (
                      <div
                        key={file.id}
                        className="flex items-center gap-2 p-2 rounded bg-background/80 border"
                      >
                        <img
                          src={file.url}
                          alt={file.fileName}
                          className="w-10 h-10 object-cover rounded"
                          loading="lazy"
                        />
                        <div className="flex-1 min-w-0">
                          <p className="text-xs font-medium truncate">{file.fileName}</p>
                          <p className="text-xs text-muted-foreground">{formatBytes(file.size)}</p>
                        </div>
                      </div>
                    ))}
                    {imgbbFiles.length > 3 && (
                      <p className="text-xs text-center text-muted-foreground pt-1">
                        点击查看全部 {imgbbFiles.length} 个文件
                      </p>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* R2 账户卡片 */}
            {accounts.length === 0 ? (
              <Card>
                <CardContent className="flex flex-col items-center justify-center py-12">
                  <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
                  <p className="text-muted-foreground">暂无 R2 账户</p>
                  <p className="text-sm text-muted-foreground mt-2">前往账户管理页面添加 R2 存储账户</p>
                </CardContent>
              </Card>
            ) : (
              accounts.map((account) => (
                <Card
                  key={account.id}
                  className="cursor-pointer hover:shadow-lg transition-shadow"
                  onClick={() => navigate(`/files/${account.id}`)}
                >
                  <CardHeader className="pb-3">
                    <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
                      <FolderOpen className="h-4 w-4 sm:h-5 sm:w-5 text-primary flex-shrink-0" />
                      <span className="truncate">{account.name}</span>
                      <ChevronRight className="h-4 w-4 ml-auto text-muted-foreground flex-shrink-0" />
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="pt-0">
                    <div className="space-y-2 text-xs sm:text-sm">
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">存储占用</span>
                        <span className="font-medium">{formatBytes(account.usage.sizeBytes)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">配额</span>
                        <span className="font-medium">{formatBytes(account.quota.maxSizeBytes)}</span>
                      </div>
                      <div className="w-full bg-secondary h-2 rounded-full overflow-hidden">
                        <div
                          className="bg-primary h-full transition-all"
                          style={{
                            width: `${Math.min(
                              (account.usage.sizeBytes / account.quota.maxSizeBytes) * 100,
                              100
                            )}%`,
                          }}
                        />
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))
            )}
          </div>
          {totalPages > 1 && (
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={handlePageChange}
              totalItems={totalItems}
              pageSize={PAGE_SIZE}
            />
          )}
        </>
      )}
    </div>
  );
}
