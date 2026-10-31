import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

function Code({ text }: { text: string }) {
  return <pre className="overflow-x-auto rounded-lg bg-muted p-4 text-sm">{text}</pre>;
}
function Json({ value }: { value: unknown }) { return <Code text={JSON.stringify(value, null, 2)} />; }
const initFields = [
  ["filename", "string", "是", "原文件名，不含目录分隔符"],
  ["size", "integer", "是", "实际文件字节数；空文件允许，单次直传最大 5 GiB"],
  ["contentType", "string", "否", "默认为 application/octet-stream"],
  ["path", "string", "否", "存储目录，不含文件名；留空使用日期目录"],
  ["idGroup", "string", "否", "指定目标账户，留空自动选择满足剩余额度的账户"],
  ["expirationDays", "integer", "否", "-1 或留空使用默认设置，0 永久，1–36500 指定天数"],
  ["randomFilename", "boolean", "否", "默认 true；false 保留原文件名，同名文件拒绝上传"],
  ["requestId", "string", "否", "客户端生成的唯一标识；初始化响应丢失时以相同标识、相同参数重试"],
];

export default function DirectUploadDocs({ baseUrl }: { baseUrl: string }) {
  return <>
    <Card>
      <CardHeader><CardTitle className="text-base">文件搜索与扁平化浏览</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground">GET /api/files 保持目录浏览；增加 flat=true 列出 prefix 下所有子目录中的文件，不返回目录节点。增加 q 搜索文件名或完整路径，大小写不敏感，搜索配合 flat=true 使用。</p>
        <Code text={[
          "GET /api/files?idGroup=storage-account-id&flat=true&q=report&limit=50",
          "Authorization: Bearer your-api-token",
          "",
          "// 后续分页复用相同 idGroup、prefix、flat、q、limit",
          "GET /api/files?idGroup=storage-account-id&flat=true&q=report&limit=50&cursor=nextCursor"
        ].join("\n")} />
        <p className="text-sm text-muted-foreground">初次查询的 idGroup 可用逗号分隔最多 20 个账户 ID；续页须指定返回结果中的单个账户 ID，并复用对应的 nextCursor。limit 默认 50，最大 100。每个账户每次最多扫描一批 R2 列表；本批 files 为空但仍有 nextCursor 时，不能判定后续没有匹配文件。网页“选择已加载文件”只涵盖已经读取的结果。</p>
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle className="flex flex-wrap items-center gap-2 text-base"><Badge className="bg-blue-600">POST</Badge>/api/upload/init</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground">创建浏览器直传任务，需要 write 权限。服务器校验文件大小、账户上传权限、剩余容量及请求额度，为并发任务预留配额，并返回临时上传地址。文件正文随后直接发送到 R2。</p>
        <div className="overflow-x-auto"><table className="w-full text-sm"><caption className="mb-2 text-left font-medium">请求参数（application/json）</caption><thead><tr className="border-b">{["参数", "类型", "必填", "说明"].map((label) => <th className="py-2 pr-4 text-left" key={label}>{label}</th>)}</tr></thead><tbody className="divide-y">{initFields.map((row) => <tr key={row[0]}>{row.map((cell, index) => <td className="py-2 pr-4" key={index}>{index === 0 ? <code>{cell}</code> : cell}</td>)}</tr>)}</tbody></table></div>
        <p className="text-sm font-medium">请求示例</p>
        <Json value={{ filename: "report.pdf", size: 102400, contentType: "application/pdf", path: "team/reports", expirationDays: 30, randomFilename: false, requestId: "client-generated-unique-id" }} />
        <p className="text-sm font-medium">响应示例</p>
        <Json value={{
          uploadId: "upload-task-id", url: "https://account.r2.cloudflarestorage.com/bucket/team/reports/report.pdf?temporary-signature",
          method: "PUT", headers: { "Content-Type": "application/pdf", "X-Amz-Meta-Fileflow-Upload-Id": "upload-task-id", "If-None-Match": "*" },
          expiresAt: "2026-10-31T08:15:00Z", accountId: "storage-account-id", key: "team/reports/report.pdf", size: 102400,
        }} />
        <p className="text-sm text-muted-foreground">上传链接有效期 15 分钟，任务确认窗口为 1 小时。签名绑定文件大小、类型、任务信息及“不覆盖已有对象”条件，必须上传初始化时声明的文件。R2 条件上传遇到已有同名对象会返回 412，初始化或确认冲突会返回 409。</p>
        <p className="text-sm text-muted-foreground">网页原样发送响应中的 headers，并使用 File 或 Blob 作为请求正文。浏览器会自动设置实际 Content-Length，请勿手动设置 Content-Length 或 Host，也不要把 FileFlow 的 Authorization 发给 R2。</p>
        <p className="text-xs text-muted-foreground">零字节文件的响应 headers 还包含 Content-MD5，绑定空文件内容；也需原样发送。</p>
        <p className="text-sm font-medium">网页调用流程</p>
        <Code text={[
          "// file 是用户选择的 File；auth 是 FileFlow 的登录 JWT 或 API Token",
          "const taskResponse = await fetch(\"/api/upload/init\", {",
          "  method: \"POST\",",
          "  headers: { Authorization: \"Bearer \" + auth, \"Content-Type\": \"application/json\" },",
          "  body: JSON.stringify({ filename: file.name, size: file.size,",
          "    contentType: file.type || \"application/octet-stream\",",
          "    randomFilename: false, requestId: crypto.randomUUID() })",
          "});",
          "if (!taskResponse.ok) throw new Error(\"初始化上传失败\");",
          "const task = await taskResponse.json();",
          "",
          "const put = await fetch(task.url, {",
          "  method: task.method, headers: task.headers, body: file",
          "});",
          "if (!put.ok) throw new Error(\"R2 上传失败\");",
          "",
          "const complete = await fetch(\"/api/upload/complete\", {",
          "  method: \"POST\",",
          "  headers: { Authorization: \"Bearer \" + auth, \"Content-Type\": \"application/json\" },",
          "  body: JSON.stringify({ uploadId: task.uploadId })",
          "});",
          "if (!complete.ok) throw new Error(\"确认失败，请保留 uploadId 重试确认\");",
          "const result = await complete.json();",
        ].join("\n")} />
        <p className="text-sm font-medium">R2 CORS 示例</p>
        <Json value={[{ AllowedOrigins: [baseUrl], AllowedMethods: ["PUT", "GET", "HEAD"], AllowedHeaders: ["Content-Type", "Content-MD5", "X-Amz-Meta-Fileflow-Upload-Id", "If-None-Match"], ExposeHeaders: ["ETag"], MaxAgeSeconds: 3600 }]} />
        <p className="text-xs text-muted-foreground">使用真实部署的网页来源配置 CORS；协议、域名和端口必须匹配。跨域配置失败时，网页可以显式选择服务器中转上传。</p>
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle className="flex flex-wrap items-center gap-2 text-base"><Badge className="bg-blue-600">POST</Badge>/api/upload/complete</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <p className="text-muted-foreground">使用发起上传的身份确认任务，需要 write 权限。服务器检查 R2 对象的实际大小、类型及任务信息，确认后记录文件有效期、用量和操作记录，返回与 /api/upload 相同的 UploadResult。</p>
        <Json value={{ uploadId: "upload-task-id" }} />
        <Json value={{ id: "storage-account-id", accountName: "团队存储", key: "team/reports/report.pdf", size: 102400, url: "https://pub-example.r2.dev/team/reports/report.pdf" }} />
        <p className="text-sm text-muted-foreground">同一任务可幂等确认，重复成功确认不会重复添加到期记录或计算文件容量。PUT 已成功但确认响应丢失时，保留原 uploadId 并重试此接口，避免再次上传。PUT 响应不确定时先尝试确认；确实未上传且链接仍有效时才重试 PUT。</p>
        <p className="text-sm text-muted-foreground">任务绑定成员与凭证；成员停用、权限变化、目标账户停用、对象被篡改或任务过期时不能直接确认。网页显示“正在确认上传结果”后，收到服务器成功响应才视为上传完成。</p>
        <p className="text-sm text-muted-foreground">FileFlow 签发任务前预留容量和 Class A / Class B 请求额度，确认时再次验证，限制应用授权的并发任务。R2 签名链接在有效期内仍可重复请求，其他工具直接访问 R2 的调用也不经过 FileFlow；实际云端请求用量仍需同步核对。</p>
        <p className="text-sm text-muted-foreground">取消或移除队列项只会停止客户端传输，不能撤销已签发的 R2 链接。服务器保留预留额度，直到确认完成或过期任务经过清理校验；清理失败时也不会提前释放。切换中转上传可能仍受原任务预留额度或同名任务限制。</p>
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
        <p className="text-muted-foreground">设置或修改文件到期时间，需要 write 权限，最多 100 个路径。expiresAt 使用未来 ISO 8601 时间；传 null 设为永久。服务器验证全部文件后更新计划。</p>
        <Json value={{ accountId: "storage-account-id", keys: ["team/report.pdf", "docs/readme.txt"], expiresAt: "2027-01-01T00:00:00Z" }} />
        <p className="text-sm font-medium">永久保存示例</p>
        <Json value={{ accountId: "storage-account-id", keys: ["team/report.pdf"], expiresAt: null }} />
        <Json value={{ updatedCount: 2 }} />
      </CardContent>
    </Card>
  </>;
}
