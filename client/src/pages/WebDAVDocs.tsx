import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Info } from "lucide-react";

/**
 * WebDAV 接口文档页面
 */
export default function WebDAVDocs() {
  const baseUrl = window.location.origin;
  const webdavEndpoint = `${baseUrl}/webdav`;

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-3xl font-bold">WebDAV 接口</h1>
        <p className="text-muted-foreground mt-2">
          FileFlow 提供标准 WebDAV 接口，支持使用各类 WebDAV 客户端（Windows 文件资源管理器、macOS Finder、Cyberduck 等）直接访问
        </p>
      </div>

      {/* 特性说明 */}
      <div className="flex items-start gap-3 p-4 rounded-lg border bg-blue-50 dark:bg-blue-950/20 text-blue-800 dark:text-blue-200">
        <Info className="h-5 w-5 mt-0.5 shrink-0" />
        <p className="text-sm">
          WebDAV 接口使用 HTTP Basic Auth 认证，需要在「设置 → WebDAV 凭证」页面创建专用的用户名和密码
        </p>
      </div>

      {/* 端点信息 */}
      <Card>
        <CardHeader>
          <CardTitle>端点信息</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4">
            <div>
              <p className="text-sm font-medium mb-1">WebDAV 端点 URL</p>
              <code className="bg-muted px-3 py-2 rounded block text-sm">{webdavEndpoint}</code>
            </div>
            <div>
              <p className="text-sm font-medium mb-1">认证方式</p>
              <code className="bg-muted px-3 py-2 rounded block text-sm">HTTP Basic Auth</code>
            </div>
            <div>
              <p className="text-sm font-medium mb-1">支持的方法</p>
              <div className="flex flex-wrap gap-2 mt-2">
                <Badge>PROPFIND</Badge>
                <Badge>GET</Badge>
                <Badge>PUT</Badge>
                <Badge>DELETE</Badge>
                <Badge>MKCOL</Badge>
                <Badge>COPY</Badge>
                <Badge>MOVE</Badge>
                <Badge>LOCK/UNLOCK</Badge>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 凭证创建 */}
      <Card>
        <CardHeader>
          <CardTitle>创建 WebDAV 凭证</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <ol className="list-decimal list-inside space-y-2 text-muted-foreground">
            <li>进入「设置」页面，切换到「WebDAV 凭证」选项卡</li>
            <li>点击「创建凭证」按钮</li>
            <li>选择要关联的存储账户</li>
            <li>设置权限（read/write/delete）</li>
            <li>创建后可随时查看和复制用户名和密码</li>
          </ol>
        </CardContent>
      </Card>

      {/* 客户端配置 */}
      <Card>
        <CardHeader>
          <CardTitle>客户端配置示例</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Windows */}
          <div>
            <p className="text-sm font-medium mb-2">Windows 文件资源管理器</p>
            <ol className="list-decimal list-inside space-y-2 text-sm text-muted-foreground">
              <li>右键点击「此电脑」或「计算机」</li>
              <li>选择「映射网络驱动器」</li>
              <li>选择一个驱动器号（如 Z:）</li>
              <li>在文件夹框中输入: <code className="bg-muted px-2 py-0.5 rounded text-xs">{webdavEndpoint}</code></li>
              <li>勾选「登录时重新连接」（可选）</li>
              <li>点击「完成」，输入 WebDAV 凭证的用户名和密码</li>
            </ol>
          </div>

          {/* macOS */}
          <div>
            <p className="text-sm font-medium mb-2">macOS Finder</p>
            <ol className="list-decimal list-inside space-y-2 text-sm text-muted-foreground">
              <li>打开 Finder</li>
              <li>按下 Command + K，或选择菜单「前往 → 连接服务器」</li>
              <li>在服务器地址框中输入: <code className="bg-muted px-2 py-0.5 rounded text-xs">{webdavEndpoint}</code></li>
              <li>点击「连接」</li>
              <li>选择「注册用户」，输入 WebDAV 凭证的用户名和密码</li>
              <li>勾选「在我的钥匙串中记住此密码」（可选）</li>
            </ol>
          </div>

          {/* Linux */}
          <div>
            <p className="text-sm font-medium mb-2">Linux (GNOME Files / Nautilus)</p>
            <ol className="list-decimal list-inside space-y-2 text-sm text-muted-foreground">
              <li>打开文件管理器（Files / Nautilus）</li>
              <li>点击「其他位置」或按下 Ctrl + L</li>
              <li>在服务器地址框中输入: <code className="bg-muted px-2 py-0.5 rounded text-xs">dav://{window.location.host}/webdav</code></li>
              <li>点击「连接」</li>
              <li>输入 WebDAV 凭证的用户名和密码</li>
            </ol>
          </div>

          {/* Cyberduck */}
          <div>
            <p className="text-sm font-medium mb-2">Cyberduck / Mountain Duck</p>
            <ol className="list-decimal list-inside space-y-2 text-sm text-muted-foreground">
              <li>点击「打开连接」</li>
              <li>在协议下拉菜单中选择「WebDAV (HTTP)」或「WebDAV (HTTPS)」</li>
              <li>服务器: <code className="bg-muted px-2 py-0.5 rounded text-xs">{window.location.host}</code></li>
              <li>路径: <code className="bg-muted px-2 py-0.5 rounded text-xs">/webdav</code></li>
              <li>用户名和密码: 填入 WebDAV 凭证信息</li>
              <li>点击「连接」</li>
            </ol>
          </div>

          {/* curl */}
          <div>
            <p className="text-sm font-medium mb-2">命令行 (curl)</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`# 列出文件
curl -u username:password "${webdavEndpoint}/" -X PROPFIND --data '<?xml version="1.0"?><propfind xmlns="DAV:"><allprop/></propfind>'

# 上传文件
curl -u username:password -T local-file.txt "${webdavEndpoint}/path/file.txt"

# 下载文件
curl -u username:password "${webdavEndpoint}/path/file.txt" -o local-file.txt

# 删除文件
curl -u username:password -X DELETE "${webdavEndpoint}/path/file.txt"

# 创建目录
curl -u username:password -X MKCOL "${webdavEndpoint}/new-folder/"`}
            </pre>
          </div>

          {/* Python */}
          <div>
            <p className="text-sm font-medium mb-2">Python (webdav3)</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`from webdav3.client import Client

options = {
    'webdav_hostname': '${webdavEndpoint}',
    'webdav_login': 'username',
    'webdav_password': 'password',
}
client = Client(options)

# 列出文件
files = client.list('/')
for f in files:
    print(f)

# 上传文件
client.upload_sync(remote_path='path/file.txt', local_path='local-file.txt')

# 下载文件
client.download_sync(remote_path='path/file.txt', local_path='local-file.txt')

# 删除文件
client.clean('path/file.txt')

# 创建目录
client.mkdir('new-folder')`}
            </pre>
          </div>

          {/* Node.js */}
          <div>
            <p className="text-sm font-medium mb-2">Node.js (webdav)</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`import { createClient } from 'webdav';

const client = createClient('${webdavEndpoint}', {
  username: 'username',
  password: 'password',
});

// 列出文件
const files = await client.getDirectoryContents('/');
console.log(files);

// 上传文件
const fileBuffer = fs.readFileSync('./local-file.txt');
await client.putFileContents('/path/file.txt', fileBuffer);

// 下载文件
const data = await client.getFileContents('/path/file.txt');
fs.writeFileSync('./local-file.txt', data);

// 删除文件
await client.deleteFile('/path/file.txt');

// 创建目录
await client.createDirectory('/new-folder');`}
            </pre>
          </div>
        </CardContent>
      </Card>

      {/* 权限说明 */}
      <Card>
        <CardHeader>
          <CardTitle>权限说明</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <div>
              <p className="text-sm font-medium">读取权限 (read)</p>
              <p className="text-xs text-muted-foreground">允许列出目录、下载文件（PROPFIND, GET, HEAD）</p>
            </div>
            <div>
              <p className="text-sm font-medium">写入权限 (write)</p>
              <p className="text-xs text-muted-foreground">允许上传文件、创建目录、复制移动文件（PUT, MKCOL, COPY, MOVE）</p>
            </div>
            <div>
              <p className="text-sm font-medium">删除权限 (delete)</p>
              <p className="text-xs text-muted-foreground">允许删除文件和目录（DELETE）</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 常见问题 */}
      <Card>
        <CardHeader>
          <CardTitle>常见问题</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <p className="text-sm font-medium">Windows 映射网络驱动器失败</p>
            <p className="text-xs text-muted-foreground">
              如果使用 HTTP（非 HTTPS），需要修改注册表允许 Basic Auth。运行 regedit，定位到:
              <br />
              <code className="bg-muted px-2 py-0.5 rounded text-xs break-all block mt-1">
                HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\WebClient\Parameters
              </code>
              <br />
              将 BasicAuthLevel 的值改为 2（允许 HTTP Basic Auth）
            </p>
          </div>
          <div>
            <p className="text-sm font-medium">macOS 无法连接</p>
            <p className="text-xs text-muted-foreground">
              确保在连接时使用完整的 URL（包括 /webdav 路径），并且用户名密码正确。如果仍然失败，可以尝试使用第三方客户端如 Cyberduck。
            </p>
          </div>
          <div>
            <p className="text-sm font-medium">上传大文件失败</p>
            <p className="text-xs text-muted-foreground">
              大文件上传受到反向代理（Nginx/Caddy）的限制。请检查 client_max_body_size 或相应的配置项。
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
