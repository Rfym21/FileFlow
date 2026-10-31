import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

function Code({ text }: { text: string }) {
  return <pre className="overflow-x-auto rounded-lg bg-muted p-4 text-sm">{text}</pre>;
}

function Json({ value }: { value: unknown }) {
  return <Code text={JSON.stringify(value, null, 2)} />;
}

export default function FileOperationsDocs() {
  return <>
    <Card>
      <CardHeader><CardTitle className="text-base">文件搜索与扁平化浏览</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground">GET /api/files 保持目录浏览；增加 flat=true 列出 prefix 下所有子目录中的文件，不返回目录节点。增加 q 搜索文件名或完整路径，大小写不敏感，并自动启用扁平浏览。</p>
        <Code text={[
          "GET /api/files?idGroup=storage-account-id&flat=true&q=report&limit=50",
          "Authorization: Bearer your-api-token",
          "",
          "// 后续分页复用相同 idGroup、prefix、flat、q、limit",
          "GET /api/files?idGroup=storage-account-id&flat=true&q=report&limit=50&cursor=nextCursor",
        ].join("\n")} />
        <p className="text-sm text-muted-foreground">初次查询的 idGroup 可用逗号分隔最多 20 个账户 ID；续页须指定返回结果中的单个账户 ID，并复用对应的 nextCursor。limit 默认 50，最大 100。每个账户每次最多扫描一批 R2 列表；本批 files 为空但仍有 nextCursor 时，不能判定后续没有匹配文件。网页“选择已加载文件”只涵盖已经读取的结果。</p>
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle className="flex flex-wrap items-center gap-2 text-base"><Badge className="bg-blue-600">POST</Badge>/api/files/batch-delete</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground">批量删除扁平化列表中的文件，需要 delete 权限。最多 100 个文件路径，逐项返回结果；失败文件仍可重试，不能传目录。</p>
        <Json value={{ accountId: "storage-account-id", keys: ["team/report.pdf", "docs/readme.txt"] }} />
        <Json value={{ results: [{ key: "team/report.pdf", success: true }, { key: "docs/readme.txt", success: false, error: "删除失败" }], successCount: 1, failCount: 1 }} />
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle className="flex flex-wrap items-center gap-2 text-base"><Badge className="bg-blue-600">PATCH</Badge>/api/files/expirations</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground">设置或修改文件到期时间，需要 JWT 与 write 权限，最多 100 个路径。expiresAt 使用未来 RFC3339 时间；传 null 设为永久。服务器验证全部文件后更新计划。</p>
        <Json value={{ accountId: "storage-account-id", keys: ["team/report.pdf", "docs/readme.txt"], expiresAt: "2027-01-01T00:00:00Z" }} />
        <p className="text-sm font-medium">永久保存示例</p>
        <Json value={{ accountId: "storage-account-id", keys: ["team/report.pdf"], expiresAt: null }} />
        <Json value={{ updatedCount: 2 }} />
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle className="text-base">服务器上传与配额</CardTitle></CardHeader>
      <CardContent className="space-y-3 text-sm text-muted-foreground">
        <p>所有文件和公网 URL 上传均经过 FileFlow 服务，单文件最大 5 GiB。服务器校验实际字节数与账号权限、原子预留容量和 Class A / Class B 额度，在写入后核验对象，并保存用量和到期计划；保留文件名时拒绝覆盖同名对象。</p>
        <p>目录列表 / 搜索计 Class A，HEAD / GET 和预览计 Class B，DELETE 不产生 R2 请求费用；SDK 重试也计数。Class B 限额为 0 时表示不限读取。网页上传不需要配置 R2 上传 CORS。</p>
        <p>请求失败或取消后，服务器会核验并清理尚未完成的内部操作，清理未确认前保留相应容量预留。文件预览的签名下载链接、公开域名访问和其他客户端请求仍可能产生应用计数之外的 R2 用量。</p>
      </CardContent>
    </Card>
  </>;
}
