import { useState, useRef, useCallback, useEffect } from "react";
import { toast } from "sonner";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import {
  uploadFile,
  getAccounts,
  type UploadResult,
  type Account,
} from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import {
  X,
  Copy,
  Check,
  FileImage,
  FileVideo,
  File as FileIcon,
  CloudUpload,
  ExternalLink,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface UploadItem {
  id: string;
  file: File;
  status: "pending" | "uploading" | "success" | "error";
  progress: number;
  result?: UploadResult;
  error?: string;
}

interface FileUploaderProps {
  onUploadComplete?: () => void;
  defaultAccountId?: string;
  defaultPath?: string;
}

/**
 * 判断文件是否为图片
 */
function isImageFile(file: File | string): boolean {
  const type = typeof file === "string" ? file : file.type;
  return type.startsWith("image/");
}

/**
 * 判断文件是否为视频
 */
function isVideoFile(file: File | string): boolean {
  const type = typeof file === "string" ? file : file.type;
  return type.startsWith("video/");
}

/**
 * 根据 URL 判断文件类型
 */
function getFileTypeFromUrl(url: string): "image" | "video" | "other" {
  const ext = url.split(".").pop()?.toLowerCase() || "";
  const imageExts = ["jpg", "jpeg", "png", "gif", "webp", "svg", "ico", "bmp"];
  const videoExts = ["mp4", "webm", "ogg", "mov", "avi", "mkv"];
  if (imageExts.includes(ext)) return "image";
  if (videoExts.includes(ext)) return "video";
  return "other";
}

export default function FileUploader({
  onUploadComplete,
  defaultAccountId,
  defaultPath,
}: FileUploaderProps) {
  const [isDragging, setIsDragging] = useState(false);
  const [uploads, setUploads] = useState<UploadItem[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccountId, setSelectedAccountId] = useState<string>(defaultAccountId || "");
  const [isLoadingAccounts, setIsLoadingAccounts] = useState(true);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dragCounter = useRef(0);

  // 加载账户列表
  useEffect(() => {
    const loadAccounts = async () => {
      try {
        const data = await getAccounts();
        setAccounts(data || []);
      } catch (err) {
        console.error("加载账户列表失败:", err);
      } finally {
        setIsLoadingAccounts(false);
      }
    };
    loadAccounts();
  }, []);

  // 更新默认账户
  useEffect(() => {
    if (defaultAccountId) {
      setSelectedAccountId(defaultAccountId);
    }
  }, [defaultAccountId]);

  /**
   * 处理拖拽进入
   */
  const handleDragEnter = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current++;
    if (e.dataTransfer.items && e.dataTransfer.items.length > 0) {
      setIsDragging(true);
    }
  }, []);

  /**
   * 处理拖拽离开
   */
  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    dragCounter.current--;
    if (dragCounter.current === 0) {
      setIsDragging(false);
    }
  }, []);

  /**
   * 处理拖拽悬停
   */
  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  }, []);

  /**
   * 处理文件放置
   */
  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    dragCounter.current = 0;

    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) {
      addFilesToUpload(files);
    }
  }, []);

  /**
   * 处理文件选择
   */
  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    if (files.length > 0) {
      addFilesToUpload(files);
    }
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  }, []);

  /**
   * 添加文件到上传队列
   */
  const addFilesToUpload = (files: File[]) => {
    const newUploads: UploadItem[] = files.map((file) => ({
      id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      file,
      status: "pending",
      progress: 0,
    }));
    setUploads((prev) => [...prev, ...newUploads]);

    // 自动开始上传
    newUploads.forEach((item) => {
      uploadSingleFile(item.id);
    });
  };

  /**
   * 上传单个文件
   */
  const uploadSingleFile = async (uploadId: string) => {
    setUploads((prev) =>
      prev.map((item) =>
        item.id === uploadId ? { ...item, status: "uploading", progress: 10 } : item
      )
    );

    const upload = uploads.find((u) => u.id === uploadId) ||
      (await new Promise<UploadItem | undefined>((resolve) => {
        setUploads((prev) => {
          const found = prev.find((u) => u.id === uploadId);
          resolve(found);
          return prev;
        });
      }));

    if (!upload) return;

    try {
      // 模拟进度
      const progressInterval = setInterval(() => {
        setUploads((prev) =>
          prev.map((item) =>
            item.id === uploadId && item.status === "uploading" && item.progress < 90
              ? { ...item, progress: item.progress + 10 }
              : item
          )
        );
      }, 200);

      const result = await uploadFile(
        upload.file,
        defaultPath || undefined,
        selectedAccountId || undefined
      );

      clearInterval(progressInterval);

      setUploads((prev) =>
        prev.map((item) =>
          item.id === uploadId
            ? { ...item, status: "success", progress: 100, result }
            : item
        )
      );

      onUploadComplete?.();
    } catch (err) {
      setUploads((prev) =>
        prev.map((item) =>
          item.id === uploadId
            ? {
                ...item,
                status: "error",
                progress: 0,
                error: err instanceof Error ? err.message : "上传失败",
              }
            : item
        )
      );
    }
  };

  /**
   * 移除上传项
   */
  const removeUpload = (uploadId: string) => {
    setUploads((prev) => prev.filter((item) => item.id !== uploadId));
  };

  /**
   * 清空已完成的上传
   */
  const clearCompleted = () => {
    setUploads((prev) => prev.filter((item) => item.status !== "success"));
  };

  const hasUploads = uploads.length > 0;
  const hasCompleted = uploads.some((u) => u.status === "success");
  const availableAccounts = accounts.filter((a) => a.isAvailable);

  return (
    <div className="space-y-4">
      {/* 账户选择 */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="flex-1">
          <label className="text-sm font-medium mb-1.5 block">目标账户</label>
          <select
            value={selectedAccountId}
            onChange={(e) => setSelectedAccountId(e.target.value)}
            disabled={isLoadingAccounts}
            className="w-full h-9 px-3 rounded-md border border-input bg-background text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option value="">自动选择（智能分配）</option>
            {availableAccounts.map((account) => (
              <option key={account.id} value={account.id}>
                {account.name} ({formatBytes(account.usage.sizeBytes)} / {formatBytes(account.quota.maxSizeBytes)})
              </option>
            ))}
          </select>
        </div>
        {hasCompleted && (
          <div className="flex items-end">
            <Button variant="outline" size="sm" onClick={clearCompleted}>
              清除已完成
            </Button>
          </div>
        )}
      </div>

      {/* 拖拽上传区域 */}
      <div
        onDragEnter={handleDragEnter}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
        className={cn(
          "relative border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-all",
          isDragging
            ? "border-primary bg-primary/5 scale-[1.02]"
            : "border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/50"
        )}
      >
        <input
          ref={fileInputRef}
          type="file"
          multiple
          className="hidden"
          onChange={handleFileSelect}
        />
        <div className="flex flex-col items-center gap-3">
          <div
            className={cn(
              "p-4 rounded-full transition-colors",
              isDragging ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"
            )}
          >
            <CloudUpload className="h-8 w-8" />
          </div>
          <div>
            <p className="text-base font-medium">
              {isDragging ? "释放以上传文件" : "拖拽文件到这里上传"}
            </p>
            <p className="text-sm text-muted-foreground mt-1">
              或点击选择文件，支持多文件上传
            </p>
          </div>
        </div>
      </div>

      {/* 上传列表 */}
      {hasUploads && (
        <div className="space-y-3">
          {uploads.map((upload) => (
            <UploadItemCard
              key={upload.id}
              upload={upload}
              onRemove={() => removeUpload(upload.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface UploadItemCardProps {
  upload: UploadItem;
  onRemove: () => void;
}

function UploadItemCard({ upload, onRemove }: UploadItemCardProps) {
  const [copied, setCopied] = useState(false);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  // 生成本地预览
  useEffect(() => {
    if (isImageFile(upload.file) || isVideoFile(upload.file)) {
      const url = URL.createObjectURL(upload.file);
      setPreviewUrl(url);
      return () => URL.revokeObjectURL(url);
    }
  }, [upload.file]);

  const handleCopy = async () => {
    if (upload.result?.url) {
      await navigator.clipboard.writeText(upload.result.url);
      setCopied(true);
      toast.success("链接已复制");
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleOpenUrl = () => {
    if (upload.result?.url) {
      window.open(upload.result.url, "_blank");
    }
  };

  const isImage = isImageFile(upload.file);
  const isVideo = isVideoFile(upload.file);
  const resultFileType = upload.result?.url ? getFileTypeFromUrl(upload.result.url) : "other";

  return (
    <Card className="overflow-hidden">
      <CardContent className="p-3 sm:p-4">
        <div className="flex gap-3">
          {/* 预览/图标 */}
          <div className="flex-shrink-0 w-16 h-16 sm:w-20 sm:h-20 rounded-lg overflow-hidden bg-muted flex items-center justify-center">
            {upload.status === "success" && resultFileType === "image" ? (
              <img
                src={upload.result?.url}
                alt={upload.file.name}
                className="w-full h-full object-cover"
              />
            ) : upload.status === "success" && resultFileType === "video" ? (
              <video
                src={upload.result?.url}
                className="w-full h-full object-cover"
                muted
              />
            ) : previewUrl && isImage ? (
              <img
                src={previewUrl}
                alt={upload.file.name}
                className="w-full h-full object-cover"
              />
            ) : previewUrl && isVideo ? (
              <video
                src={previewUrl}
                className="w-full h-full object-cover"
                muted
              />
            ) : isImage ? (
              <FileImage className="h-8 w-8 text-muted-foreground" />
            ) : isVideo ? (
              <FileVideo className="h-8 w-8 text-muted-foreground" />
            ) : (
              <FileIcon className="h-8 w-8 text-muted-foreground" />
            )}
          </div>

          {/* 信息 */}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <p className="font-medium text-sm truncate">{upload.file.name}</p>
                <p className="text-xs text-muted-foreground">
                  {formatBytes(upload.file.size)}
                  {upload.result && (
                    <span className="ml-2">
                      <Badge variant="outline" className="text-xs">
                        {upload.result.accountName}
                      </Badge>
                    </span>
                  )}
                </p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 flex-shrink-0"
                onClick={onRemove}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* 状态和进度 */}
            {upload.status === "uploading" && (
              <div className="mt-2">
                <Progress value={upload.progress} className="h-1.5" />
                <p className="text-xs text-muted-foreground mt-1">上传中 {upload.progress}%</p>
              </div>
            )}

            {upload.status === "error" && (
              <p className="text-xs text-destructive mt-2">{upload.error}</p>
            )}

            {/* 成功结果 */}
            {upload.status === "success" && upload.result && (
              <div className="mt-2 space-y-2">
                <div className="flex items-center gap-1 p-2 bg-muted rounded text-xs font-mono break-all">
                  <span className="flex-1 truncate">{upload.result.url}</span>
                </div>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={handleCopy}
                  >
                    {copied ? (
                      <Check className="h-3 w-3 mr-1 text-green-500" />
                    ) : (
                      <Copy className="h-3 w-3 mr-1" />
                    )}
                    复制链接
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    onClick={handleOpenUrl}
                  >
                    <ExternalLink className="h-3 w-3 mr-1" />
                    打开
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
