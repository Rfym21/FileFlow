import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

/**
 * API 文档页面
 */
export default function ApiDocs() {
  const baseUrl = window.location.origin;

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-3xl font-bold">开放 API 文档</h1>
        <p className="text-muted-foreground mt-2">
          FileFlow 提供 RESTful API 供外部应用调用，需要使用 API Token 进行认证
        </p>
      </div>

      {/* 认证方式 */}
      <Card>
        <CardHeader>
          <CardTitle>认证方式</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            在请求头中添加 <code className="bg-muted px-1.5 py-0.5 rounded">Authorization</code> 字段，使用 Bearer Token 格式：
          </p>
          <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`curl -X GET "${baseUrl}/api/files" \\
  -H "Authorization: Bearer your-api-token"`}
          </pre>
          <p className="text-xs text-muted-foreground">
            API Token 可在「设置 → API Token」页面创建和管理
          </p>
        </CardContent>
      </Card>

      {/* 权限说明 */}
      <Card>
        <CardHeader>
          <CardTitle>权限说明</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 pr-4">权限</th>
                  <th className="text-left py-2 pr-4">允许的操作</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                <tr>
                  <td className="py-2 pr-4"><Badge variant="secondary">read</Badge></td>
                  <td className="py-2 text-muted-foreground">获取文件列表、获取文件链接</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4"><Badge variant="secondary">write</Badge></td>
                  <td className="py-2 text-muted-foreground">上传文件</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4"><Badge variant="secondary">delete</Badge></td>
                  <td className="py-2 text-muted-foreground">删除文件</td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* API 列表 */}
      <div className="space-y-6">
        <h2 className="text-xl font-semibold">API 接口</h2>

        {/* 获取文件列表 */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Badge className="bg-green-600">GET</Badge>
              /api/files
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground">获取文件列表（懒加载+分页）</p>
            <div>
              <p className="text-sm font-medium mb-2">查询参数</p>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left py-2 pr-4">参数</th>
                      <th className="text-left py-2 pr-4">类型</th>
                      <th className="text-left py-2 pr-4">必填</th>
                      <th className="text-left py-2">说明</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    <tr>
                      <td className="py-2 pr-4"><code>idGroup</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string[]</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">指定账户 ID（可重复传递多个），不填则查询所有账户</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>prefix</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">目录前缀（如 images/）</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>cursor</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">分页游标（从响应的 nextCursor 获取）</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>limit</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">number</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">每页数量，默认 50，最大 100</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">请求示例</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`# 获取所有账户根目录
curl -X GET "${baseUrl}/api/files" \\
  -H "Authorization: Bearer your-api-token"

# 查询单个账户
curl -X GET "${baseUrl}/api/files?idGroup=xxx-xxx-xxx" \\
  -H "Authorization: Bearer your-api-token"

# 查询多个账户（逗号分隔）
curl -X GET "${baseUrl}/api/files?idGroup=id1,id2,id3" \\
  -H "Authorization: Bearer your-api-token"

# 指定目录前缀 + 分页
curl -X GET "${baseUrl}/api/files?idGroup=xxx&prefix=images/&limit=20" \\
  -H "Authorization: Bearer your-api-token"`}
              </pre>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">响应示例</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`[
  {
    "id": "acc-001",
    "accountName": "主账户",
    "files": [
      {
        "key": "images/",
        "name": "images",
        "isDir": true
      },
      {
        "key": "doc.pdf",
        "name": "doc.pdf",
        "size": 102400,
        "lastModified": "2024-01-01T00:00:00Z",
        "isDir": false
      }
    ],
    "sizeBytes": 1024000,
    "maxSize": 10737418240,
    "nextCursor": "abc123..."
  },
  {
    "id": "acc-002",
    "accountName": "备用账户",
    "files": [
      {
        "key": "backup/",
        "name": "backup",
        "isDir": true
      }
    ],
    "sizeBytes": 512000,
    "maxSize": 5368709120
  }
]`}
              </pre>
            </div>
          </CardContent>
        </Card>

        {/* 上传文件 */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Badge className="bg-blue-600">POST</Badge>
              /api/upload
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground">上传文件（可指定账户，不指定则智能选择）。支持两种方式：直接上传文件或从 URL 下载后上传。</p>

            <div className="rounded-lg bg-blue-50 dark:bg-blue-950 p-3 text-sm">
              <p className="font-medium text-blue-900 dark:text-blue-100 mb-1">💡 ImgBB 图床支持</p>
              <p className="text-blue-800 dark:text-blue-200">
                系统已集成 ImgBB 免费图床。当启用 ImgBB 优先模式且上传<strong>图片类型</strong>文件时，将自动使用 ImgBB（失败则回退到 R2）。
                仅支持图片格式（jpg, png, gif, webp, bmp, svg），URL 上传始终使用 R2。
                响应中 <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">provider: "imgbb"</code> 表示使用了 ImgBB。
              </p>
            </div>

            <div>
              <p className="text-sm font-medium mb-2">请求参数（multipart/form-data）</p>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left py-2 pr-4">参数</th>
                      <th className="text-left py-2 pr-4">类型</th>
                      <th className="text-left py-2 pr-4">必填</th>
                      <th className="text-left py-2">说明</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    <tr>
                      <td className="py-2 pr-4"><code>file</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">file</td>
                      <td className="py-2 pr-4 text-muted-foreground">二选一</td>
                      <td className="py-2 text-muted-foreground">要上传的文件（与 url 参数互斥）</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>url</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">二选一</td>
                      <td className="py-2 text-muted-foreground">远程文件 URL，从该地址下载后上传（与 file 参数互斥）</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>path</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">自定义存储路径（不含文件名）</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>idGroup</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">指定上传到的账户 ID，不填则智能选择</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>expirationDays</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">number</td>
                      <td className="py-2 pr-4 text-muted-foreground">否</td>
                      <td className="py-2 text-muted-foreground">文件有效期（天）。不填或 -1=使用系统默认，0=永久，&gt;0=指定天数</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">请求示例</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`# 方式一：直接上传文件
curl -X POST "${baseUrl}/api/upload" \\
  -H "Authorization: Bearer your-api-token" \\
  -F "file=@/path/to/image.jpg" \\
  -F "path=images/2024"

# 方式二：从 URL 下载后上传
curl -X POST "${baseUrl}/api/upload" \\
  -H "Authorization: Bearer your-api-token" \\
  -F "url=https://example.com/image.png" \\
  -F "path=images/2024"

# 指定账户上传 + 设置 7 天后过期
curl -X POST "${baseUrl}/api/upload" \\
  -H "Authorization: Bearer your-api-token" \\
  -F "file=@/path/to/image.jpg" \\
  -F "idGroup=xxx-xxx-xxx" \\
  -F "expirationDays=7"

# 上传永久文件（不过期）
curl -X POST "${baseUrl}/api/upload" \\
  -H "Authorization: Bearer your-api-token" \\
  -F "file=@/path/to/image.jpg" \\
  -F "expirationDays=0"`}
              </pre>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">响应示例</p>
              <p className="text-xs text-muted-foreground mb-2">R2 存储响应：</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto mb-4">
{`{
  "id": "xxx-xxx-xxx",
  "accountId": "xxx-xxx-xxx",
  "key": "images/2024/image.jpg",
  "size": 102400,
  "url": "https://pub-xxx.r2.dev/images/2024/image.jpg"
}`}
              </pre>
              <p className="text-xs text-muted-foreground mb-2">ImgBB 图床响应（图片文件且启用优先模式）：</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`{
  "id": "imgbb",
  "accountId": "imgbb",
  "key": "https://i.ibb.co/xxxxx/image.jpg",
  "url": "https://i.ibb.co/xxxxx/image.jpg",
  "deleteUrl": "https://ibb.co/xxxxx/delete-token",
  "size": 102400,
  "provider": "imgbb"
}`}
              </pre>
              <p className="text-xs text-muted-foreground mt-2">
                注意：ImgBB 响应中 <code className="bg-muted px-1 rounded">deleteUrl</code> 用于删除文件，
                <code className="bg-muted px-1 rounded">provider</code> 字段标识使用的存储服务。
              </p>
            </div>
          </CardContent>
        </Card>

        {/* 获取文件链接 */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Badge className="bg-green-600">GET</Badge>
              /api/link
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground">获取文件的公开访问链接</p>

            <div className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">
              <p className="font-medium mb-1">💡 ImgBB 文件支持</p>
              <p>
                对于 ImgBB 上传的文件（<code className="bg-background px-1 rounded">idGroup=imgbb</code>），
                <code className="bg-background px-1 rounded">key</code> 就是直接访问 URL，接口会原样返回。
              </p>
            </div>

            <div>
              <p className="text-sm font-medium mb-2">查询参数</p>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left py-2 pr-4">参数</th>
                      <th className="text-left py-2 pr-4">类型</th>
                      <th className="text-left py-2 pr-4">必填</th>
                      <th className="text-left py-2">说明</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    <tr>
                      <td className="py-2 pr-4"><code>idGroup</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">是</td>
                      <td className="py-2 text-muted-foreground">账户 ID</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>key</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">是</td>
                      <td className="py-2 text-muted-foreground">文件路径</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">请求示例</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`curl -X GET "${baseUrl}/api/link?idGroup=xxx&key=images/photo.jpg" \\
  -H "Authorization: Bearer your-api-token"`}
              </pre>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">响应示例</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`{
  "url": "https://pub-xxx.r2.dev/images/photo.jpg"
}`}
              </pre>
            </div>
          </CardContent>
        </Card>

        {/* 删除文件 */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Badge className="bg-red-600">DELETE</Badge>
              /api/file
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground">删除文件</p>

            <div className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">
              <p className="font-medium mb-1">💡 ImgBB 文件删除</p>
              <p>
                对于 ImgBB 上传的文件，需要使用上传时返回的 <code className="bg-background px-1 rounded">deleteUrl</code> 作为
                <code className="bg-background px-1 rounded">key</code> 参数，<code className="bg-background px-1 rounded">idGroup</code>
                设置为 <code className="bg-background px-1 rounded">imgbb</code>。
              </p>
            </div>

            <div>
              <p className="text-sm font-medium mb-2">查询参数</p>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left py-2 pr-4">参数</th>
                      <th className="text-left py-2 pr-4">类型</th>
                      <th className="text-left py-2 pr-4">必填</th>
                      <th className="text-left py-2">说明</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    <tr>
                      <td className="py-2 pr-4"><code>idGroup</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">是</td>
                      <td className="py-2 text-muted-foreground">账户 ID</td>
                    </tr>
                    <tr>
                      <td className="py-2 pr-4"><code>key</code></td>
                      <td className="py-2 pr-4 text-muted-foreground">string</td>
                      <td className="py-2 pr-4 text-muted-foreground">是</td>
                      <td className="py-2 text-muted-foreground">文件路径</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">请求示例</p>
              <p className="text-xs text-muted-foreground mb-2">删除 R2 文件：</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto mb-4">
{`curl -X DELETE "${baseUrl}/api/file?idGroup=xxx&key=images/photo.jpg" \\
  -H "Authorization: Bearer your-api-token"`}
              </pre>
              <p className="text-xs text-muted-foreground mb-2">删除 ImgBB 文件（使用上传时返回的 deleteUrl）：</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`curl -X DELETE "${baseUrl}/api/file?idGroup=imgbb&key=https://ibb.co/xxxxx/delete-token" \\
  -H "Authorization: Bearer your-api-token"`}
              </pre>
            </div>
            <div>
              <p className="text-sm font-medium mb-2">响应示例</p>
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`{
  "message": "删除成功"
}`}
              </pre>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* 错误响应 */}
      <Card>
        <CardHeader>
          <CardTitle>错误响应</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">当请求失败时，会返回以下格式的错误信息：</p>
          <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`{
  "error": "错误信息描述"
}`}
          </pre>
          <div>
            <p className="text-sm font-medium mb-2">常见错误码</p>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 pr-4">状态码</th>
                    <th className="text-left py-2">说明</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  <tr>
                    <td className="py-2 pr-4 font-medium">400</td>
                    <td className="py-2 text-muted-foreground">请求参数错误</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">401</td>
                    <td className="py-2 text-muted-foreground">未认证或 Token 无效</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">403</td>
                    <td className="py-2 text-muted-foreground">权限不足</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">404</td>
                    <td className="py-2 text-muted-foreground">资源不存在</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">500</td>
                    <td className="py-2 text-muted-foreground">服务器内部错误</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
