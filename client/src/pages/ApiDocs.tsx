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
            <p className="text-muted-foreground">上传文件（可指定账户，不指定则智能选择）</p>
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
                      <td className="py-2 pr-4 text-muted-foreground">是</td>
                      <td className="py-2 text-muted-foreground">要上传的文件</td>
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
{`# 智能上传（自动选择账户，使用默认有效期）
curl -X POST "${baseUrl}/api/upload" \\
  -H "Authorization: Bearer your-api-token" \\
  -F "file=@/path/to/image.jpg" \\
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
              <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`{
  "id": "xxx-xxx-xxx",
  "accountName": "主账户",
  "key": "images/2024/image.jpg",
  "size": 102400,
  "url": "https://pub-xxx.r2.dev/images/2024/image.jpg"
}`}
              </pre>
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
{`curl -X DELETE "${baseUrl}/api/file?idGroup=xxx&key=images/photo.jpg" \\
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
