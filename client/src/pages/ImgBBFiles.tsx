import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import {
  getImgBBFiles,
  deleteFile,
  type ImgBBFile,
} from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import {
  ArrowLeft,
  RefreshCw,
  Trash2,
  ExternalLink,
  Copy,
  Image as ImageIcon,
} from "lucide-react";

const PAGE_SIZE = 12;

export default function ImgBBFiles() {
  const navigate = useNavigate();
  const [files, setFiles] = useState<ImgBBFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);

  const loadFiles = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getImgBBFiles();
      setFiles(data);
    } catch (err) {
      console.error("加载 ImgBB 文件失败:", err);
      toast.error("加载文件失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadFiles();
  }, [loadFiles]);

  const handleDelete = async (file: ImgBBFile) => {
    if (!confirm(`确定要删除 ${file.fileName} 吗？`)) {
      return;
    }

    try {
      await deleteFile("imgbb", file.deleteUrl);
      toast.success("删除成功");
      loadFiles();
    } catch (err) {
      console.error("删除失败:", err);
      toast.error("删除失败");
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success("已复制到剪贴板");
  };

  // 分页计算
  const totalPages = Math.ceil(files.length / PAGE_SIZE);
  const paginatedFiles = files.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE
  );

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  return (
    <div className="space-y-4 sm:space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="flex items-center gap-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate("/files")}
            className="h-9 w-9 p-0"
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-2xl sm:text-3xl font-bold flex items-center gap-2">
              <ImageIcon className="h-6 w-6 sm:h-7 sm:w-7 text-blue-600" />
              ImgBB 图床
            </h1>
            <p className="text-sm sm:text-base text-muted-foreground">
              共 {files.length} 个文件
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={loadFiles}
          disabled={loading}
        >
          <RefreshCw
            className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`}
          />
          刷新
        </Button>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : files.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <ImageIcon className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">暂无文件</p>
            <p className="text-sm text-muted-foreground mt-2">
              前往文件管理页面上传图片到 ImgBB
            </p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {paginatedFiles.map((file) => (
              <Card
                key={file.id}
                className="overflow-hidden hover:shadow-lg transition-shadow"
              >
                <div className="aspect-video relative bg-muted">
                  <img
                    src={file.url}
                    alt={file.fileName}
                    className="w-full h-full object-cover"
                    loading="lazy"
                  />
                </div>
                <CardContent className="p-3 space-y-2">
                  <p className="text-sm font-medium truncate" title={file.fileName}>
                    {file.fileName}
                  </p>
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>{formatBytes(file.size)}</span>
                    <span>{new Date(file.uploadedAt).toLocaleDateString()}</span>
                  </div>
                  <div className="flex gap-1 pt-1">
                    <Button
                      variant="outline"
                      size="sm"
                      className="flex-1 h-8"
                      onClick={() => copyToClipboard(file.url)}
                      title="复制链接"
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="flex-1 h-8"
                      onClick={() => window.open(file.url, "_blank")}
                      title="在新窗口打开"
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="flex-1 h-8 text-red-600 hover:text-red-700 hover:bg-red-50"
                      onClick={() => handleDelete(file)}
                      title="删除"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          {totalPages > 1 && (
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}
    </div>
  );
}
