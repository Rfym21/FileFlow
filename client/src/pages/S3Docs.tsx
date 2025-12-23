import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Info } from "lucide-react";

/**
 * S3 兼容 API 文档页面
 */
export default function S3Docs() {
  const baseUrl = window.location.origin;
  const s3Endpoint = `${baseUrl}/s3`;

  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-3xl font-bold">S3 兼容接口</h1>
        <p className="text-muted-foreground mt-2">
          FileFlow 提供 S3 兼容 API，支持使用标准 S3 客户端（aws-cli、boto3、s3cmd 等）直接连接操作
        </p>
      </div>

      {/* 特性说明 */}
      <div className="flex items-start gap-3 p-4 rounded-lg border bg-blue-50 dark:bg-blue-950/20 text-blue-800 dark:text-blue-200">
        <Info className="h-5 w-5 mt-0.5 shrink-0" />
        <p className="text-sm">
          S3 兼容接口使用 AWS Signature v4 签名验证，需要在「设置 → S3 凭证」页面创建专用的 Access Key
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
              <p className="text-sm font-medium mb-1">S3 端点 URL</p>
              <code className="bg-muted px-3 py-2 rounded block text-sm">{s3Endpoint}</code>
            </div>
            <div>
              <p className="text-sm font-medium mb-1">区域 (Region)</p>
              <code className="bg-muted px-3 py-2 rounded block text-sm">auto 或任意 AWS 区域</code>
              <p className="text-xs text-muted-foreground mt-1">
                推荐使用 <code className="px-1 py-0.5 bg-muted rounded">auto</code> 或 <code className="px-1 py-0.5 bg-muted rounded">us-east-1</code>
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                Region 参数仅用于签名计算，实际请求会路由到 FileFlow 自定义端点，可使用任意 AWS 标准区域名称
              </p>
            </div>
            <div>
              <p className="text-sm font-medium mb-1">访问模式</p>
              <div className="space-y-2">
                <div>
                  <code className="bg-muted px-3 py-2 rounded block text-sm">Path Style（路径风格）</code>
                  <p className="text-xs text-muted-foreground mt-1">
                    URL 格式: {s3Endpoint}/bucket-name/object-key
                  </p>
                </div>
                <div>
                  <code className="bg-muted px-3 py-2 rounded block text-sm">Virtual Hosted Style（虚拟主机风格）</code>
                  <p className="text-xs text-muted-foreground mt-1">
                    URL 格式: bucket-name.s3.example.com/object-key（需在设置中启用并配置 DNS）
                  </p>
                </div>
              </div>
              <p className="text-xs text-muted-foreground mt-2">
                💡 两种模式可同时使用，客户端会根据配置自动选择
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 凭证创建 */}
      <Card>
        <CardHeader>
          <CardTitle>创建 S3 凭证</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <ol className="list-decimal list-inside space-y-2 text-muted-foreground">
            <li>进入「设置」页面，切换到「S3 凭证」选项卡</li>
            <li>点击「创建凭证」按钮</li>
            <li>
              <strong>选择要关联的存储账户</strong> - 该账户的 Bucket 名称将作为 S3 Bucket 名称
              <div className="ml-6 mt-1 p-2 bg-yellow-50 dark:bg-yellow-950/20 rounded text-xs">
                ⚠️ 重要：配置 S3 客户端时，Bucket 名称必须使用该账户的 Bucket Name（可在「账户管理」或「S3 凭证」页面查看）
              </div>
            </li>
            <li>设置权限（read/write/delete）</li>
            <li>创建后可随时查看和复制 Access Key ID、Secret Access Key 和 Bucket Name</li>
          </ol>
        </CardContent>
      </Card>

      {/* 客户端配置 */}
      <Card>
        <CardHeader>
          <CardTitle>客户端配置示例</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* AWS CLI */}
          <div>
            <p className="text-sm font-medium mb-2">AWS CLI</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`# 配置凭证
aws configure set aws_access_key_id FFLWXXXXXXXXXXXX
aws configure set aws_secret_access_key xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# 列出文件
aws s3 ls s3://my-bucket --endpoint-url ${s3Endpoint}

# 上传文件
aws s3 cp ./local-file.txt s3://my-bucket/path/file.txt --endpoint-url ${s3Endpoint}

# 下载文件
aws s3 cp s3://my-bucket/path/file.txt ./local-file.txt --endpoint-url ${s3Endpoint}

# 删除文件
aws s3 rm s3://my-bucket/path/file.txt --endpoint-url ${s3Endpoint}

# 同步目录
aws s3 sync ./local-dir s3://my-bucket/remote-dir --endpoint-url ${s3Endpoint}`}
            </pre>
          </div>

          {/* Python boto3 */}
          <div>
            <p className="text-sm font-medium mb-2">Python (boto3)</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`import boto3

s3 = boto3.client('s3',
    endpoint_url='${s3Endpoint}',
    aws_access_key_id='FFLWXXXXXXXXXXXX',
    aws_secret_access_key='xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
    region_name='us-east-1'
)

# 列出对象
response = s3.list_objects_v2(Bucket='my-bucket', Prefix='images/')
for obj in response.get('Contents', []):
    print(obj['Key'], obj['Size'])

# 上传文件
s3.upload_file('./local-file.txt', 'my-bucket', 'path/file.txt')

# 下载文件
s3.download_file('my-bucket', 'path/file.txt', './local-file.txt')

# 删除文件
s3.delete_object(Bucket='my-bucket', Key='path/file.txt')`}
            </pre>
          </div>

          {/* Node.js */}
          <div>
            <p className="text-sm font-medium mb-2">Node.js (@aws-sdk/client-s3)</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`import { S3Client, ListObjectsV2Command, PutObjectCommand } from '@aws-sdk/client-s3';
import { readFileSync } from 'fs';

const s3 = new S3Client({
  endpoint: '${s3Endpoint}',
  region: 'us-east-1',
  credentials: {
    accessKeyId: 'FFLWXXXXXXXXXXXX',
    secretAccessKey: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
  },
  forcePathStyle: true,
});

// 列出对象
const listResult = await s3.send(new ListObjectsV2Command({
  Bucket: 'my-bucket',
  Prefix: 'images/',
}));
console.log(listResult.Contents);

// 上传文件
await s3.send(new PutObjectCommand({
  Bucket: 'my-bucket',
  Key: 'path/file.txt',
  Body: readFileSync('./local-file.txt'),
}));`}
            </pre>
          </div>

          {/* Go */}
          <div>
            <p className="text-sm font-medium mb-2">Go (aws-sdk-go-v2)</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`package main

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
    cfg := aws.Config{
        Region: "us-east-1",
        Credentials: credentials.NewStaticCredentialsProvider(
            "FFLWXXXXXXXXXXXX",
            "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
            "",
        ),
    }

    client := s3.NewFromConfig(cfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String("${s3Endpoint}")
        o.UsePathStyle = true
    })

    // 列出对象
    result, _ := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
        Bucket: aws.String("my-bucket"),
        Prefix: aws.String("images/"),
    })
    for _, obj := range result.Contents {
        fmt.Println(*obj.Key, *obj.Size)
    }
}`}
            </pre>
          </div>

          {/* s3cmd */}
          <div>
            <p className="text-sm font-medium mb-2">s3cmd</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`# 创建配置文件 ~/.s3cfg
[default]
access_key = FFLWXXXXXXXXXXXX
secret_key = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
host_base = ${baseUrl.replace('https://', '').replace('http://', '')}/s3
host_bucket = ${baseUrl.replace('https://', '').replace('http://', '')}/s3/%(bucket)
use_https = ${baseUrl.startsWith('https') ? 'True' : 'False'}

# 使用
s3cmd ls s3://my-bucket/
s3cmd put ./file.txt s3://my-bucket/path/
s3cmd get s3://my-bucket/path/file.txt ./`}
            </pre>
          </div>

          {/* rclone */}
          <div>
            <p className="text-sm font-medium mb-2">rclone</p>
            <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`# 配置 (~/.config/rclone/rclone.conf)
[fileflow]
type = s3
provider = Other
access_key_id = FFLWXXXXXXXXXXXX
secret_access_key = xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
endpoint = ${s3Endpoint}
acl = private

# 使用
rclone ls fileflow:my-bucket/
rclone copy ./local-dir fileflow:my-bucket/remote-dir/
rclone sync ./local-dir fileflow:my-bucket/remote-dir/`}
            </pre>
          </div>
        </CardContent>
      </Card>

      {/* 支持的操作 */}
      <Card>
        <CardHeader>
          <CardTitle>支持的 S3 操作</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 pr-4">操作</th>
                  <th className="text-left py-2 pr-4">描述</th>
                  <th className="text-left py-2">所需权限</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                <tr>
                  <td className="py-2 pr-4 font-medium">HeadBucket</td>
                  <td className="py-2 pr-4 text-muted-foreground">检查 Bucket 是否存在</td>
                  <td className="py-2"><Badge variant="secondary">read</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">ListObjectsV2</td>
                  <td className="py-2 pr-4 text-muted-foreground">列出对象</td>
                  <td className="py-2"><Badge variant="secondary">read</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">GetObject</td>
                  <td className="py-2 pr-4 text-muted-foreground">获取对象内容</td>
                  <td className="py-2"><Badge variant="secondary">read</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">HeadObject</td>
                  <td className="py-2 pr-4 text-muted-foreground">获取对象元数据</td>
                  <td className="py-2"><Badge variant="secondary">read</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">PutObject</td>
                  <td className="py-2 pr-4 text-muted-foreground">上传对象</td>
                  <td className="py-2"><Badge variant="secondary">write</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">CopyObject</td>
                  <td className="py-2 pr-4 text-muted-foreground">复制对象</td>
                  <td className="py-2"><Badge variant="secondary">read</Badge> <Badge variant="secondary">write</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">DeleteObject</td>
                  <td className="py-2 pr-4 text-muted-foreground">删除对象</td>
                  <td className="py-2"><Badge variant="secondary">delete</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">DeleteObjects</td>
                  <td className="py-2 pr-4 text-muted-foreground">批量删除对象</td>
                  <td className="py-2"><Badge variant="secondary">delete</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">CreateMultipartUpload</td>
                  <td className="py-2 pr-4 text-muted-foreground">初始化分片上传</td>
                  <td className="py-2"><Badge variant="secondary">write</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">UploadPart</td>
                  <td className="py-2 pr-4 text-muted-foreground">上传分片</td>
                  <td className="py-2"><Badge variant="secondary">write</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">CompleteMultipartUpload</td>
                  <td className="py-2 pr-4 text-muted-foreground">完成分片上传</td>
                  <td className="py-2"><Badge variant="secondary">write</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">AbortMultipartUpload</td>
                  <td className="py-2 pr-4 text-muted-foreground">取消分片上传</td>
                  <td className="py-2"><Badge variant="secondary">write</Badge></td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">ListParts</td>
                  <td className="py-2 pr-4 text-muted-foreground">列出已上传的分片</td>
                  <td className="py-2"><Badge variant="secondary">read</Badge></td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Bucket 映射 */}
      <Card>
        <CardHeader>
          <CardTitle>Bucket 名称映射</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            S3 接口的 Bucket 名称对应 FileFlow 中账户配置的「Bucket 名称」字段。
            每个 S3 凭证绑定到一个账户，只能访问该账户对应的 Bucket。
          </p>
          <div className="bg-muted p-4 rounded-lg text-sm">
            <p className="font-medium mb-2">示例</p>
            <p className="text-muted-foreground">
              如果在 FileFlow 中创建了一个账户，Bucket 名称为 <code>my-storage</code>，
              那么在 S3 客户端中就可以使用 <code>s3://my-storage/...</code> 来访问该账户的文件。
            </p>
          </div>
        </CardContent>
      </Card>

      {/* 错误响应 */}
      <Card>
        <CardHeader>
          <CardTitle>错误响应</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">S3 接口返回标准的 S3 XML 错误格式：</p>
          <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
{`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access Denied</Message>
  <Resource>/s3/my-bucket/file.txt</Resource>
  <RequestId>xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx</RequestId>
</Error>`}
          </pre>
          <div>
            <p className="text-sm font-medium mb-2">常见错误码</p>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 pr-4">错误码</th>
                    <th className="text-left py-2">说明</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  <tr>
                    <td className="py-2 pr-4 font-medium">AccessDenied</td>
                    <td className="py-2 text-muted-foreground">访问被拒绝（凭证无效或权限不足）</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">InvalidAccessKeyId</td>
                    <td className="py-2 text-muted-foreground">Access Key ID 不存在</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">SignatureDoesNotMatch</td>
                    <td className="py-2 text-muted-foreground">签名验证失败（Secret Key 错误）</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">NoSuchBucket</td>
                    <td className="py-2 text-muted-foreground">Bucket 不存在</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">NoSuchKey</td>
                    <td className="py-2 text-muted-foreground">对象不存在</td>
                  </tr>
                  <tr>
                    <td className="py-2 pr-4 font-medium">NoSuchUpload</td>
                    <td className="py-2 text-muted-foreground">分片上传会话不存在</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 注意事项 */}
      <Card>
        <CardHeader>
          <CardTitle>注意事项</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="list-disc list-inside space-y-2 text-muted-foreground">
            <li>S3 凭证与普通 API Token 是独立的，需要分别创建</li>
            <li>每个 S3 凭证只能访问其绑定账户对应的 Bucket</li>
            <li>不支持跨账户复制对象</li>
            <li>不支持创建/删除 Bucket 操作（Bucket 通过 FileFlow 管理界面管理）</li>
            <li>分片上传的分片大小建议 5MB - 100MB</li>
            <li>请确保客户端使用 Path Style（路径风格）URL，而非 Virtual Host Style</li>
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
