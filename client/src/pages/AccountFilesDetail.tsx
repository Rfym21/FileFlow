import { useEffect, useState, useRef } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  getFiles,
  uploadFile,
  deleteFile,
  getFileLink,
  type FileNode,
} from "@/lib/api";
import { formatBytes, formatDate } from "@/lib/utils";
import {
  File,
  Trash2,
  Link,
  Upload,
  RefreshCw,
  Check,
  ChevronRight as ChevronRightIcon,
  ArrowLeft,
  FolderOpen,
  Home,
} from "lucide-react";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 50;

export default function AccountFilesDetail() {
  const { accountId } = useParams<{ accountId: string }>();
  const navigate = useNavigate();
  const [accountInfo, setAccountInfo] = useState<{ name: string; sizeBytes: number; maxSize: number } | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [currentPath, setCurrentPath] = useState<string>("");
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  /**
   * 加载当前目录的文件
   */
  const loadFiles = async (prefix: string = "", cursor?: string, append: boolean = false) => {
    if (!accountId) return;

    if (append) {
      setLoadingMore(true);
    } else {
      setLoading(true);
      setFiles([]);
      setNextCursor(undefined);
    }

    try {
      const data = await getFiles(prefix, cursor, PAGE_SIZE, accountId ? [accountId] : undefined);
      if (data && data.length > 0) {
        const accountData = data[0];
        setAccountInfo({
          name: accountData.accountName,
          sizeBytes: accountData.sizeBytes,
          maxSize: accountData.maxSize,
        });
        if (append) {
          setFiles(prev => [...prev, ...(accountData.files || [])]);
        } else {
          setFiles(accountData.files || []);
        }
        setNextCursor(accountData.nextCursor);
      }
    } catch (err) {
      console.error("加载文件失败:", err);
      toast.error("加载文件失败");
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  };

  /**
   * 加载更多
   */
  const handleLoadMore = () => {
    if (nextCursor) {
      loadFiles(currentPath, nextCursor, true);
    }
  };

  /**
   * 导航到指定路径
   */
  const handleNavigateToPath = (path: string) => {
    setCurrentPath(path);
    loadFiles(path);
  };

  /**
   * 进入子目录
   */
  const handleEnterDirectory = (dirKey: string) => {
    setCurrentPath(dirKey);
    loadFiles(dirKey);
  };

  /**
   * 上传文件
   */
  const handleUploadWithPath = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const fileList = e.target.files;
    if (!fileList || fileList.length === 0) return;

    setUploading(true);
    try {
      for (const file of fileList) {
        await uploadFile(file, currentPath || undefined);
      }
      toast.success("上传成功");
      loadFiles(currentPath);
    } catch (err) {
      console.error("上传失败:", err);
      toast.error(err instanceof Error ? err.message : "上传失败");
    } finally {
      setUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  /**
   * 删除文件
   */
  const handleDelete = async (key: string) => {
    if (!accountId || !confirm(`确定要删除 ${key} 吗？`)) return;

    try {
      await deleteFile(accountId, key);
      toast.success("删除成功");
      loadFiles(currentPath);
    } catch (err) {
      console.error("删除失败:", err);
      toast.error(err instanceof Error ? err.message : "删除失败");
    }
  };

  /**
   * 复制链接
   */
  const handleCopyLink = async (key: string) => {
    if (!accountId) return;

    try {
      const { url } = await getFileLink(accountId, key);
      await navigator.clipboard.writeText(url);
      toast.success("链接已复制到剪贴板");
    } catch (err) {
      console.error("获取链接失败:", err);
      toast.error("复制链接失败");
    }
  };

  /**
   * 获取面包屑
   */
  const getBreadcrumbs = () => {
    if (!currentPath) return [];
    return currentPath.split("/").filter(p => p);
  };

  useEffect(() => {
    loadFiles("");
  }, [accountId]);

  if (loading && files.length === 0) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!accountInfo) {
    return (
      <div className="space-y-6">
        <Button variant="outline" onClick={() => navigate("/files")}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          返回
        </Button>
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">账户不存在或无权访问</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  const breadcrumbs = getBreadcrumbs();

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Header */}
      <div className="space-y-4">
        <div className="flex items-center gap-2 sm:gap-4">
          <Button variant="outline" size="sm" onClick={() => navigate("/files")}>
            <ArrowLeft className="mr-1 sm:mr-2 h-4 w-4" />
            <span className="hidden sm:inline">返回</span>
          </Button>
          <div className="flex-1 min-w-0">
            <h1 className="text-xl sm:text-2xl lg:text-3xl font-bold truncate">
              {accountInfo.name}
            </h1>
            <p className="text-xs sm:text-sm text-muted-foreground">
              {formatBytes(accountInfo.sizeBytes)} / {formatBytes(accountInfo.maxSize)}
            </p>
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadFiles(currentPath)}
            disabled={loading}
            className="flex-1 sm:flex-none"
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            <span className="hidden sm:inline">刷新</span>
          </Button>
          <Button
            size="sm"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="flex-1 sm:flex-none"
          >
            <Upload className="mr-2 h-4 w-4" />
            {uploading ? "上传中..." : "上传"}
          </Button>
          <Input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={handleUploadWithPath}
          />
        </div>
      </div>

      {/* 面包屑导航 */}
      <Card>
        <CardContent className="p-3 sm:p-4">
          <div className="flex items-center gap-1 sm:gap-2 text-xs sm:text-sm overflow-x-auto pb-2 scrollbar-hide">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleNavigateToPath("")}
              className={cn("flex-shrink-0", !currentPath && "bg-accent")}
            >
              <Home className="h-3 w-3 sm:h-4 sm:w-4 mr-1" />
              <span className="hidden sm:inline">根目录</span>
            </Button>
            {breadcrumbs.map((crumb, index) => {
              const path = breadcrumbs.slice(0, index + 1).join("/") + "/";
              const isLast = index === breadcrumbs.length - 1;
              return (
                <div key={path} className="flex items-center gap-1 sm:gap-2 flex-shrink-0">
                  <span className="text-muted-foreground">/</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleNavigateToPath(path)}
                    className={cn("whitespace-nowrap", isLast && "bg-accent")}
                  >
                    {crumb}
                  </Button>
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>

      {/* 文件列表 */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base sm:text-lg">
            当前目录 ({files.length} 项{nextCursor ? "+" : ""})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {files.length === 0 ? (
            <p className="text-muted-foreground text-sm text-center py-8">空目录</p>
          ) : (
            <div className="space-y-1">
              {files.map((item) => (
                <FileManagerItem
                  key={item.key}
                  item={item}
                  onEnterDirectory={handleEnterDirectory}
                  onDelete={handleDelete}
                  onCopyLink={handleCopyLink}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 加载更多 */}
      {nextCursor && (
        <div className="flex justify-center">
          <Button
            variant="outline"
            onClick={handleLoadMore}
            disabled={loadingMore}
          >
            {loadingMore ? (
              <>
                <RefreshCw className="mr-2 h-4 w-4 animate-spin" />
                加载中...
              </>
            ) : (
              <>
                <ChevronRightIcon className="mr-2 h-4 w-4" />
                加载更多
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  );
}

interface FileManagerItemProps {
  item: FileNode;
  onEnterDirectory: (dirKey: string) => void;
  onDelete: (key: string) => void;
  onCopyLink: (key: string) => void;
}

function FileManagerItem({
  item,
  onEnterDirectory,
  onDelete,
  onCopyLink,
}: FileManagerItemProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await onCopyLink(item.key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (item.isDir) {
    return (
      <div
        className="flex items-center gap-2 py-3 px-3 sm:px-4 rounded hover:bg-accent cursor-pointer group transition-colors"
        onClick={() => onEnterDirectory(item.key)}
      >
        <FolderOpen className="h-4 w-4 sm:h-5 sm:w-5 flex-shrink-0 text-blue-500" />
        <span className="text-sm sm:text-base flex-1 truncate">{item.name}</span>
        <ChevronRightIcon className="h-4 w-4 text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 py-3 px-3 sm:px-4 rounded hover:bg-accent group transition-colors">
      <File className="h-4 w-4 sm:h-5 sm:w-5 flex-shrink-0 text-muted-foreground" />
      <div className="flex-1 min-w-0">
        <div className="text-sm sm:text-base truncate">{item.name}</div>
        <div className="flex gap-2 text-xs text-muted-foreground mt-0.5">
          {item.size !== undefined && <span>{formatBytes(item.size)}</span>}
          {item.lastModified && (
            <span className="hidden sm:inline">{formatDate(item.lastModified)}</span>
          )}
        </div>
      </div>
      <div className="flex gap-1 flex-shrink-0 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity">
        <Button variant="ghost" size="icon" className="h-8 w-8 sm:h-9 sm:w-9" onClick={handleCopy}>
          {copied ? <Check className="h-4 w-4 text-green-500" /> : <Link className="h-4 w-4" />}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 sm:h-9 sm:w-9 text-destructive"
          onClick={() => onDelete(item.key)}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
