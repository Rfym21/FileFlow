import { useEffect, useState, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  getFiles,
  uploadFile,
  type AccountFiles,
} from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import {
  FolderOpen,
  Upload,
  RefreshCw,
  ChevronRight,
} from "lucide-react";

export default function Files() {
  const navigate = useNavigate();
  const [accountFiles, setAccountFiles] = useState<AccountFiles[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadFiles = async () => {
    setLoading(true);
    try {
      const data = await getFiles();
      setAccountFiles(data || []);
    } catch (err) {
      console.error("加载文件失败:", err);
      toast.error("加载文件失败");
    } finally {
      setLoading(false);
    }
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    setUploading(true);
    try {
      for (const file of files) {
        await uploadFile(file);
      }
      await loadFiles();
      toast.success("上传成功");
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


  useEffect(() => {
    loadFiles();
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
            onClick={loadFiles}
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
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="flex-1 sm:flex-none"
          >
            <Upload className="mr-2 h-4 w-4" />
            {uploading ? "上传中..." : <><span className="hidden sm:inline">上传文件</span><span className="sm:hidden">上传</span></>}
          </Button>
          <Input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={handleUpload}
          />
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center h-64">
          <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : accountFiles.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">暂无文件</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3 sm:gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {accountFiles.map((account) => (
            <Card
              key={account.id}
              className="cursor-pointer hover:shadow-lg transition-shadow"
              onClick={() => navigate(`/files/${account.id}`)}
            >
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base sm:text-lg">
                  <FolderOpen className="h-4 w-4 sm:h-5 sm:w-5 text-primary flex-shrink-0" />
                  <span className="truncate">{account.accountName}</span>
                  <ChevronRight className="h-4 w-4 ml-auto text-muted-foreground flex-shrink-0" />
                </CardTitle>
              </CardHeader>
              <CardContent className="pt-0">
                <div className="space-y-2 text-xs sm:text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">存储占用</span>
                    <span className="font-medium">{formatBytes(account.sizeBytes)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">配额</span>
                    <span className="font-medium">{formatBytes(account.maxSize)}</span>
                  </div>
                  <div className="w-full bg-secondary h-2 rounded-full overflow-hidden">
                    <div
                      className="bg-primary h-full transition-all"
                      style={{
                        width: `${Math.min(
                          (account.sizeBytes / account.maxSize) * 100,
                          100
                        )}%`,
                      }}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
