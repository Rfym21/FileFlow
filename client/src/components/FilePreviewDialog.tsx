import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  ArrowLeft,
  ChevronRight,
  Download,
  ExternalLink,
  FileQuestion,
  File as FileIcon,
  FolderOpen,
  Home,
  RefreshCw,
} from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { getFilePreview, type FileNode, type FilePreview } from "@/lib/api";
import { cn, formatBytes, formatDate } from "@/lib/utils";
import { normalizeZipDirectoryEntries, getZipDirectoryItems } from "@/lib/zip-directory";

interface FilePreviewDialogProps {
  accountId: string;
  file: FileNode | null;
  onClose: () => void;
}

interface PreviewState {
  status: "loading" | "ready" | "error";
  data?: FilePreview;
  error?: string;
}

export default function FilePreviewDialog({
  accountId,
  file,
  onClose,
}: FilePreviewDialogProps) {
  return (
    <Dialog
      open={file !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="flex max-h-[calc(100dvh_-_2rem)] w-[calc(100vw_-_2rem)] max-w-5xl flex-col overflow-hidden p-4 sm:p-6">
        {file && (
          <PreviewContent
            key={`${accountId}:${file.key}`}
            accountId={accountId}
            file={file}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function PreviewContent({
  accountId,
  file,
}: {
  accountId: string;
  file: FileNode;
}) {
  const [state, setState] = useState<PreviewState>({ status: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [mediaError, setMediaError] = useState(false);
  const latestRequest = useRef(0);

  useEffect(() => {
    const controller = new AbortController();
    const requestId = ++latestRequest.current;
    void getFilePreview(accountId, file.key, controller.signal)
      .then((data) => {
        if (!controller.signal.aborted && requestId === latestRequest.current)
          setState({ status: "ready", data });
      })
      .catch((error) => {
        if (!controller.signal.aborted && requestId === latestRequest.current)
          setState((previous) => ({
            ...previous,
            status: "error",
            error: error instanceof Error ? error.message : "无法加载文件预览",
          }));
      });
    return () => controller.abort();
  }, [accountId, file.key, attempt]);

  const retry = () => {
    latestRequest.current += 1;
    setMediaError(false);
    setState((previous) => ({
      ...previous,
      status: "loading",
      error: undefined,
    }));
    setAttempt((previous) => previous + 1);
  };
  const preview = state.data;

  return (
    <>
      <DialogHeader className="shrink-0 pr-6 text-left">
        <DialogTitle className="max-h-24 overflow-y-auto break-all leading-snug">
          {file.name || file.key}
        </DialogTitle>
        <DialogDescription className="break-all">
          文件预览
          {file.size !== undefined ? ` · ${formatBytes(file.size)}` : ""}
          {preview?.contentType ? ` · ${preview.contentType}` : ""}
        </DialogDescription>
      </DialogHeader>
      <div
        className="min-h-0 overflow-auto"
        aria-busy={state.status === "loading"}
      >
        {state.status === "loading" ? (
          <div
            role="status"
            className="flex min-h-48 flex-col items-center justify-center gap-3 text-muted-foreground"
          >
            <RefreshCw className="h-8 w-8 animate-spin" aria-hidden="true" />
            <p>正在加载文件预览…</p>
          </div>
        ) : state.status === "error" || mediaError ? (
          <div
            role="alert"
            className="flex min-h-48 flex-col items-center justify-center gap-3 px-4 text-center"
          >
            <AlertTriangle
              className="h-8 w-8 text-destructive"
              aria-hidden="true"
            />
            <p className="break-all text-sm text-destructive">
              {state.error ||
                "预览加载失败，链接可能已过期或浏览器不支持此文件格式。"}
            </p>
            <Button variant="outline" onClick={retry}>
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              重试预览
            </Button>
            {preview?.url && (
              <p className="text-xs text-muted-foreground">
                也可以使用下方入口打开或下载原文件。
              </p>
            )}
          </div>
        ) : (
          preview && (
            <PreviewBody
              key={`${attempt}:${preview.url}`}
              preview={preview}
              name={file.name || file.key}
              onError={() => setMediaError(true)}
              onRetry={retry}
            />
          )
        )}
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t pt-4">
        <p className="text-xs text-muted-foreground">
          {preview?.url
            ? "临时链接有效期约 15 分钟，过期后请重新加载预览。"
            : "预览失败时可重试获取文件链接。"}
        </p>
        <div className="flex flex-wrap gap-2">
          {preview?.url && (
            <>
              <a
                href={preview.url}
                target="_blank"
                rel="noopener noreferrer"
                className={buttonVariants({ variant: "outline", size: "sm" })}
              >
                <ExternalLink className="mr-2 h-4 w-4" aria-hidden="true" />
                打开原文件
              </a>
              <a
                href={preview.downloadUrl || preview.url}
                target="_blank"
                rel="noopener noreferrer"
                download={file.name}
                className={buttonVariants({ variant: "outline", size: "sm" })}
              >
                <Download className="mr-2 h-4 w-4" aria-hidden="true" />
                下载文件
              </a>
            </>
          )}
          {state.status === "ready" && (
            <Button variant="ghost" size="sm" onClick={retry}>
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              重新加载
            </Button>
          )}
        </div>
      </div>
    </>
  );
}

function PreviewBody({
  preview,
  name,
  onError,
  onRetry,
}: {
  preview: FilePreview;
  name: string;
  onError: () => void;
  onRetry: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
  useEffect(() => {
    const video = videoRef.current;
    const audio = audioRef.current;
    return () => {
      video?.pause();
      audio?.pause();
    };
  }, []);

  switch (preview.kind) {
    case "image":
      return (
        <div className="flex min-h-48 items-center justify-center rounded-md bg-muted/30 p-2">
          <img
            src={preview.url}
            alt={name}
            className="max-h-[55dvh] w-full object-contain"
            onError={onError}
          />
        </div>
      );
    case "video":
      return (
        <div className="flex min-h-48 items-center justify-center rounded-md bg-black">
          <video
            ref={videoRef}
            src={preview.url}
            controls
            preload="none"
            className="max-h-[55dvh] w-full"
            aria-label={`视频预览：${name}`}
            onError={onError}
          >
            浏览器不支持播放此视频，请打开或下载原文件。
          </video>
        </div>
      );
    case "audio":
      return (
        <div className="flex min-h-48 flex-col items-center justify-center gap-4 px-2">
          <p className="text-sm text-muted-foreground">使用播放器试听音频</p>
          <audio
            ref={audioRef}
            src={preview.url}
            controls
            preload="none"
            className="w-full max-w-xl"
            aria-label={`音频预览：${name}`}
            onError={onError}
          >
            浏览器不支持播放此音频，请打开或下载原文件。
          </audio>
        </div>
      );
    case "pdf":
      return (
        <div className="space-y-2">
          <object
            data={preview.url}
            type="application/pdf"
            className="h-[55dvh] w-full rounded-md border"
            aria-label={`PDF 预览：${name}`}
            onError={onError}
          >
            <p className="p-4 text-sm text-muted-foreground">
              浏览器不支持内嵌 PDF。请使用下方“打开原文件”或“下载文件”。
            </p>
          </object>
          <p className="text-xs text-muted-foreground">
            如果 PDF 未能显示，请在新窗口打开原文件，或下载后查看。
          </p>
        </div>
      );
    case "text":
      return (
        <div className="space-y-2">
          {preview.truncated && (
            <p role="status" className="text-sm text-muted-foreground">
              文件较大，仅显示前 256 KiB 内容。请下载原文件查看完整内容。
            </p>
          )}
          <pre
            tabIndex={0}
            aria-label={`文本预览：${name}`}
            className={cn(
              "max-h-[55dvh] min-h-48 overflow-auto rounded-md border bg-muted/30 p-4 text-xs leading-relaxed",
              "whitespace-pre",
            )}
          >
            {preview.text ||
              (preview.truncated ? "（预览内容为空）" : "（空文件）")}
          </pre>
        </div>
      );
    case "zip":
      return <ZipDirectoryPreview preview={preview} onRetry={onRetry} />;
    default:
      return (
        <div className="flex min-h-48 flex-col items-center justify-center gap-3 px-4 text-center">
          <FileQuestion
            className="h-10 w-10 text-muted-foreground"
            aria-hidden="true"
          />
          <p className="font-medium">暂不支持预览此文件类型</p>
          <p className="text-sm text-muted-foreground">
            可以打开原文件或下载后使用对应应用查看。
          </p>
        </div>
      );
  }
}

function ZipDirectoryPreview({
  preview,
  onRetry,
}: {
  preview: FilePreview;
  onRetry: () => void;
}) {
  const [currentPage, setCurrentPage] = useState(1);
  const [currentPath, setCurrentPath] = useState<string[]>([]);
  const directoryListRef = useRef<HTMLDivElement>(null);
  const entries = preview.entries;
  const normalizedEntries = useMemo(() => normalizeZipDirectoryEntries(entries || []), [entries]);
  const directoryEntries = useMemo(() => getZipDirectoryItems(normalizedEntries, currentPath), [normalizedEntries, currentPath]);
  const totalCount = preview.entryCount ?? (entries?.length || 0);
  const isIncomplete = preview.truncated || totalCount > (entries?.length || 0);
  const pageSize = 20;
  const totalPages = Math.max(1, Math.ceil(directoryEntries.length / pageSize));
  const page = Math.min(currentPage, totalPages);
  const start = (page - 1) * pageSize;
  const visibleEntries = directoryEntries.slice(start, start + pageSize);

  const navigateToDirectory = (path: string[]) => {
    setCurrentPath([...path]);
    setCurrentPage(1);
    directoryListRef.current?.scrollTo({ top: 0, left: 0 });
  };

  if (preview.archiveError)
    return (
      <div
        role="alert"
        className="flex min-h-48 flex-col items-center justify-center gap-3 px-4 text-center"
      >
        <AlertTriangle
          className="h-8 w-8 text-destructive"
          aria-hidden="true"
        />
        <p className="font-medium">无法读取 ZIP 目录</p>
        <p className="break-all text-sm text-destructive">
          {preview.archiveError}
        </p>
        <p className="text-sm text-muted-foreground">
          可以下载原压缩包后查看，或重新加载目录预览。
        </p>
        <Button variant="outline" onClick={onRetry}>
          <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
          重新加载 ZIP 目录
        </Button>
      </div>
    );

  return (
    <div className="space-y-3">
      <div className="space-y-1">
        <p className="text-sm font-medium">ZIP 目录 · 共 {totalCount} 项</p>
        <p className="text-xs text-muted-foreground">
          点击目录逐级浏览压缩包。下载压缩包后可查看文件内容。
        </p>
        {isIncomplete && (
          <p role="status" className="text-sm text-muted-foreground">
            压缩包共 {totalCount} 项，目前仅读取 {entries?.length || 0} 项（最多 1000
            项），各级目录内容可能不完整。请下载压缩包查看完整目录。
          </p>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={currentPath.length === 0}
          onClick={() => navigateToDirectory(currentPath.slice(0, -1))}
        >
          <ArrowLeft className="mr-1 h-4 w-4" aria-hidden="true" />
          返回上一级
        </Button>
        <nav aria-label="ZIP 目录导航" className="flex min-w-0 flex-1 flex-wrap items-center gap-1 text-xs">
          <Button
            variant="ghost"
            size="sm"
            aria-current={currentPath.length === 0 ? "page" : undefined}
            onClick={() => navigateToDirectory([])}
          >
            <Home className="mr-1 h-4 w-4" aria-hidden="true" />
            根目录
          </Button>
          {currentPath.map((part, index) => (
            <span key={JSON.stringify(currentPath.slice(0, index + 1))} className="flex min-w-0 items-center gap-1">
              <ChevronRight className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden="true" />
              <Button
                variant="ghost"
                size="sm"
                className="h-auto max-w-full whitespace-normal break-all text-left"
                aria-current={index === currentPath.length - 1 ? "page" : undefined}
                onClick={() => navigateToDirectory(currentPath.slice(0, index + 1))}
              >
                {part}
              </Button>
            </span>
          ))}
        </nav>
      </div>
      <p className="text-xs text-muted-foreground" aria-live="polite">
        当前目录共 {directoryEntries.length} 项
      </p>
      {(entries?.length || 0) === 0 ? (
        <div className="flex min-h-48 items-center justify-center text-sm text-muted-foreground">
          {isIncomplete ? "当前已读取内容中没有文件或目录" : "这个 ZIP 压缩包没有文件或目录"}
        </div>
      ) : (
        <>
          <div
            ref={directoryListRef}
            role="region"
            aria-label="ZIP 目录列表，可横向滚动"
            tabIndex={0}
            className="max-h-[45dvh] overflow-auto rounded-md border focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <table className="w-full min-w-[700px] text-left text-xs">
              <caption className="sr-only">
                ZIP 当前目录中的名称、类型、大小和修改时间
              </caption>
              <thead className="sticky top-0 z-10 border-b bg-background text-muted-foreground">
                <tr>
                  <th scope="col" className="min-w-64 px-3 py-3 font-medium">
                    名称
                  </th>
                  <th scope="col" className="px-3 py-3 font-medium">
                    类型
                  </th>
                  <th scope="col" className="px-3 py-3 font-medium">
                    原始大小
                  </th>
                  <th scope="col" className="px-3 py-3 font-medium">
                    压缩大小
                  </th>
                  <th scope="col" className="px-3 py-3 font-medium">
                    修改时间
                  </th>
                </tr>
              </thead>
              <tbody>
                {visibleEntries.map((entry) => (
                  <tr
                    key={entry.id}
                    className="border-b last:border-0 hover:bg-muted/30"
                  >
                    <td className="max-w-sm px-3 py-3">
                      {entry.isDir ? (
                        <button
                          type="button"
                          className="flex w-full items-start gap-2 rounded text-left hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                          aria-label={`进入压缩包目录 ${entry.label}`}
                          title={entry.name}
                          onClick={() => navigateToDirectory(entry.path)}
                        >
                          <FolderOpen className="h-4 w-4 shrink-0 text-blue-500" aria-hidden="true" />
                          <span className="min-w-0 flex-1 break-all">{entry.label}</span>
                          <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                        </button>
                      ) : (
                        <div className="flex items-start gap-2" title={entry.name}>
                          <FileIcon
                            className="h-4 w-4 shrink-0 text-muted-foreground"
                            aria-hidden="true"
                          />
                          <span className="break-all">{entry.label}</span>
                        </div>
                      )}
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">
                      {entry.isDir ? "目录" : "文件"}
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">
                      {entry.isDir ? "—" : formatBytes(entry.size)}
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">
                      {entry.isDir ? "—" : formatBytes(entry.compressedSize)}
                    </td>
                    <td className="whitespace-nowrap px-3 py-3 text-muted-foreground">
                      {entry.modifiedAt &&
                      !Number.isNaN(new Date(entry.modifiedAt).getTime())
                        ? formatDate(entry.modifiedAt)
                        : "—"}
                    </td>
                  </tr>
                ))}
                {directoryEntries.length === 0 && <tr><td colSpan={5} className="px-3 py-12 text-center text-muted-foreground">{isIncomplete ? "当前已读取内容中没有此目录的条目" : "此目录为空"}</td></tr>}
              </tbody>
            </table>
          </div>
          <nav
            aria-label="ZIP 目录分页"
            className="flex flex-wrap items-center justify-between gap-3"
          >
            <p className="text-xs text-muted-foreground" aria-live="polite">
              显示 {directoryEntries.length > 0 ? start + 1 : 0}–{Math.min(start + pageSize, directoryEntries.length)}{" "}
              项，当前目录共 {directoryEntries.length} 项 · 第 {page}/{totalPages} 页
            </p>
            {totalPages > 1 && (
              <div className="flex flex-wrap gap-1">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 1}
                  onClick={() => setCurrentPage(1)}
                >
                  首页
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 1}
                  onClick={() => setCurrentPage(page - 1)}
                >
                  上一页
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === totalPages}
                  onClick={() => setCurrentPage(page + 1)}
                >
                  下一页
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === totalPages}
                  onClick={() => setCurrentPage(totalPages)}
                >
                  末页
                </Button>
              </div>
            )}
          </nav>
        </>
      )}
    </div>
  );
}
