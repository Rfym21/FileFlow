import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { X } from "lucide-react";

/**
 * 图片灯箱组件
 */
function ImageLightbox({
  src,
  alt,
  isOpen,
  onClose,
}: {
  src: string;
  alt: string;
  isOpen: boolean;
  onClose: () => void;
}) {
  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      onClick={onClose}
    >
      <button
        className="absolute top-4 right-4 text-white hover:text-gray-300 transition-colors"
        onClick={onClose}
      >
        <X className="h-8 w-8" />
      </button>
      <img
        src={src}
        alt={alt}
        className="max-h-[90vh] max-w-[90vw] object-contain rounded-lg"
        onClick={(e) => e.stopPropagation()}
      />
    </div>
  );
}

/**
 * 可点击放大的图片组件
 */
function ClickableImage({ src, alt }: { src: string; alt: string }) {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <>
      <img
        src={src}
        alt={alt}
        className="rounded-lg border shadow-sm cursor-zoom-in hover:opacity-90 transition-opacity"
        onClick={() => setIsOpen(true)}
      />
      <ImageLightbox
        src={src}
        alt={alt}
        isOpen={isOpen}
        onClose={() => setIsOpen(false)}
      />
    </>
  );
}

/**
 * 参数获取指南页面
 */
export default function Guide() {
  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-3xl font-bold">参数获取指南</h1>
        <p className="text-muted-foreground mt-2">
          按照以下步骤从 Cloudflare 控制台获取 R2 账户所需的配置参数
        </p>
        <p className="text-xs text-muted-foreground mt-1">
          点击图片可放大查看
        </p>
      </div>

      {/* 步骤 1: 进入 R2 存储 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Badge className="h-6 w-6 rounded-full p-0 flex items-center justify-center">1</Badge>
            进入 R2 Object Storage
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            登录 Cloudflare 控制台，在左侧菜单依次点击：
          </p>
          <ol className="list-decimal list-inside space-y-1 text-sm">
            <li><strong>Storage & databases</strong></li>
            <li><strong>R2 object storage</strong></li>
            <li><strong>Overview</strong></li>
            <li>点击右上角 <strong>Create bucket</strong> 按钮创建存储桶</li>
          </ol>
          <ClickableImage src="/guide/step1.png" alt="进入 R2 存储" />
        </CardContent>
      </Card>

      {/* 步骤 2: 创建 Bucket */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Badge className="h-6 w-6 rounded-full p-0 flex items-center justify-center">2</Badge>
            创建 Bucket（获取 Bucket 名称）
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            在创建 Bucket 页面填写信息：
          </p>
          <ol className="list-decimal list-inside space-y-1 text-sm">
            <li><strong>Bucket name</strong> - 这就是账户所需的 <code className="bg-muted px-1 rounded">Bucket 名称</code> 参数</li>
            <li>Location 选择 <strong>Automatic</strong>（推荐亚太地区）</li>
            <li>Default Storage Class 选择 <strong>Standard</strong></li>
            <li>点击 <strong>Create bucket</strong></li>
          </ol>
          <ClickableImage src="/guide/step2.png" alt="创建 Bucket" />
        </CardContent>
      </Card>

      {/* 步骤 3: 获取 Public URL */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Badge className="h-6 w-6 rounded-full p-0 flex items-center justify-center">3</Badge>
            开启 Public URL（获取 Public Domain）
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            进入刚创建的 Bucket，切换到 Settings 标签页：
          </p>
          <ol className="list-decimal list-inside space-y-1 text-sm">
            <li>点击进入刚刚创建的 Bucket</li>
            <li>切换到 <strong>Settings</strong> 标签页</li>
            <li>找到 <strong>Public Development URL</strong>，点击启用</li>
            <li>复制生成的 URL，这就是账户所需的 <code className="bg-muted px-1 rounded">Public Domain</code> 参数</li>
          </ol>
          <p className="text-xs text-muted-foreground">
            提示：也可以在 Custom Domains 中绑定自定义域名
          </p>
          <ClickableImage src="/guide/step3.png" alt="获取 Public URL" />
        </CardContent>
      </Card>

      {/* 步骤 4: 创建 R2 API Token */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Badge className="h-6 w-6 rounded-full p-0 flex items-center justify-center">4</Badge>
            创建 R2 API Token（获取 Access Key）
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            回到 R2 Overview 页面，在右侧 Account Details 中点击 Manage：
          </p>
          <ol className="list-decimal list-inside space-y-1 text-sm">
            <li>在 Account Details 区域找到 API Tokens，点击 <strong>Manage</strong></li>
            <li>点击 <strong>Create Account API Token</strong></li>
            <li>填写 Token 名称（随意填写）</li>
            <li>Permissions 选择 <strong>Admin Read & Write</strong></li>
            <li>点击创建，获取以下参数：
              <ul className="list-disc list-inside ml-4 mt-1">
                <li><code className="bg-muted px-1 rounded">Access Key ID</code></li>
                <li><code className="bg-muted px-1 rounded">Secret Access Key</code></li>
                <li><code className="bg-muted px-1 rounded">Endpoint</code>（页面中的 S3 API 地址）</li>
              </ul>
            </li>
          </ol>
          <p className="text-xs text-destructive">
            注意：Secret Access Key 只显示一次，请立即保存！
          </p>
          <ClickableImage src="/guide/step4.png" alt="创建 R2 API Token" />
        </CardContent>
      </Card>

      {/* 步骤 5: 创建 User API Token */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Badge className="h-6 w-6 rounded-full p-0 flex items-center justify-center">5</Badge>
            创建 User API Token（用于查询用量统计）
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            创建 User API Token 用于查询账户用量统计（可选但推荐）：
          </p>
          <ol className="list-decimal list-inside space-y-1 text-sm">
            <li>点击右上角头像，选择 <strong>Profile</strong></li>
            <li>在左侧菜单中选择 <strong>API Tokens</strong></li>
            <li>点击 <strong>Create Token</strong></li>
            <li>选择 <strong>Custom token</strong>，点击 Get started</li>
            <li>添加权限：Account → <strong>Account Analytics</strong> → Read</li>
            <li>创建 Token，复制生成的值作为 <code className="bg-muted px-1 rounded">API Token</code> 参数</li>
          </ol>
          <p className="text-xs text-muted-foreground">
            此 Token 用于通过 Cloudflare GraphQL API 查询 R2 用量统计
          </p>
          <ClickableImage src="/guide/step5.png" alt="创建 User API Token" />
        </CardContent>
      </Card>

      {/* 参数汇总 */}
      <Card>
        <CardHeader>
          <CardTitle>参数汇总</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 pr-4">参数名称</th>
                  <th className="text-left py-2 pr-4">获取位置</th>
                  <th className="text-left py-2">说明</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                <tr>
                  <td className="py-2 pr-4 font-medium">Account ID</td>
                  <td className="py-2 pr-4 text-muted-foreground">R2 Overview → Account Details</td>
                  <td className="py-2 text-muted-foreground">Cloudflare 账户 ID</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">Bucket 名称</td>
                  <td className="py-2 pr-4 text-muted-foreground">创建 Bucket 时设置</td>
                  <td className="py-2 text-muted-foreground">存储桶名称</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">Access Key ID</td>
                  <td className="py-2 pr-4 text-muted-foreground">创建 R2 API Token 后显示</td>
                  <td className="py-2 text-muted-foreground">S3 兼容 API 访问密钥</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">Secret Access Key</td>
                  <td className="py-2 pr-4 text-muted-foreground">创建 R2 API Token 后显示</td>
                  <td className="py-2 text-muted-foreground">S3 兼容 API 密钥（仅显示一次）</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">Endpoint</td>
                  <td className="py-2 pr-4 text-muted-foreground">创建 R2 API Token 后显示</td>
                  <td className="py-2 text-muted-foreground">S3 API 端点地址</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">Public Domain</td>
                  <td className="py-2 pr-4 text-muted-foreground">Bucket Settings → Public URL</td>
                  <td className="py-2 text-muted-foreground">公开访问域名</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">API Token</td>
                  <td className="py-2 pr-4 text-muted-foreground">Profile → API Tokens</td>
                  <td className="py-2 text-muted-foreground">用于查询用量统计（可选）</td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
