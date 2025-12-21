import { useState } from "react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Check, Copy, ExternalLink } from "lucide-react";
import { toast } from "sonner";

/**
 * 代码块组件
 */
function CodeBlock({ code, language }: { code: string; language: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    toast.success("已复制到剪贴板");
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative group">
      <pre className="bg-muted rounded-lg p-4 overflow-x-auto text-sm">
        <code className={`language-${language}`}>{code}</code>
      </pre>
      <Button
        variant="ghost"
        size="icon"
        className="absolute top-2 right-2 h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity"
        onClick={handleCopy}
      >
        {copied ? (
          <Check className="h-4 w-4 text-green-500" />
        ) : (
          <Copy className="h-4 w-4" />
        )}
      </Button>
    </div>
  );
}

const workerCode = `/**
 * FileFlow R2 Endpoint Proxy - Cloudflare Worker 版本
 */

const ALLOWED_ORIGINS = "*";

function parsePath(pathname) {
  const path = pathname.startsWith("/") ? pathname.slice(1) : pathname;
  if (!path) return null;

  const firstSlash = path.indexOf("/");
  if (firstSlash === -1) {
    return { subdomain: path, filePath: "" };
  }

  return {
    subdomain: path.slice(0, firstSlash),
    filePath: path.slice(firstSlash),
  };
}

function handleCors(request) {
  const origin = request.headers.get("Origin") || "*";
  const corsHeaders = {
    "Access-Control-Allow-Origin": ALLOWED_ORIGINS === "*" ? "*" : origin,
    "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type, Range",
    "Access-Control-Max-Age": "86400",
  };

  if (request.method === "OPTIONS") {
    return new Response(null, { status: 204, headers: corsHeaders });
  }
  return null;
}

async function handleProxy(request) {
  const url = new URL(request.url);
  const parsed = parsePath(url.pathname);

  if (!parsed || !parsed.subdomain) {
    return new Response(JSON.stringify({ error: "Invalid path" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  const targetUrl = \`https://\${parsed.subdomain}.r2.dev\${parsed.filePath}\${url.search}\`;

  try {
    const response = await fetch(targetUrl, {
      method: request.method,
      headers: request.headers,
      redirect: "follow",
    });

    const headers = new Headers(response.headers);
    headers.set("Access-Control-Allow-Origin", ALLOWED_ORIGINS);
    headers.set("X-Proxy-By", "FileFlow-Worker");

    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  } catch (error) {
    return new Response(JSON.stringify({ error: "Proxy failed" }), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }
}

export default {
  async fetch(request, env, ctx) {
    const corsResponse = handleCors(request);
    if (corsResponse) return corsResponse;

    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response(JSON.stringify({ error: "Method not allowed" }), {
        status: 405,
        headers: { "Content-Type": "application/json" },
      });
    }

    return await handleProxy(request);
  },
};`;

const denoCode = `/**
 * FileFlow R2 Endpoint Proxy - Deno 版本
 *
 * 运行方式：
 *   deno run --allow-net endpoint-proxy.ts
 *
 * 环境变量：
 *   PORT - 监听端口（默认 8787）
 */

const PORT = parseInt(Deno.env.get("PORT") || "8787");
const ALLOWED_ORIGINS = Deno.env.get("ALLOWED_ORIGINS") || "*";

function parsePath(pathname: string) {
  const path = pathname.startsWith("/") ? pathname.slice(1) : pathname;
  if (!path) return null;

  const firstSlash = path.indexOf("/");
  if (firstSlash === -1) {
    return { subdomain: path, filePath: "" };
  }

  return {
    subdomain: path.slice(0, firstSlash),
    filePath: path.slice(firstSlash),
  };
}

async function handler(request: Request): Promise<Response> {
  // CORS 预检
  if (request.method === "OPTIONS") {
    return new Response(null, {
      status: 204,
      headers: {
        "Access-Control-Allow-Origin": ALLOWED_ORIGINS,
        "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
        "Access-Control-Allow-Headers": "Content-Type, Range",
      },
    });
  }

  if (request.method !== "GET" && request.method !== "HEAD") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers: { "Content-Type": "application/json" },
    });
  }

  const url = new URL(request.url);
  const parsed = parsePath(url.pathname);

  if (!parsed || !parsed.subdomain) {
    return new Response(JSON.stringify({ error: "Invalid path" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  const targetUrl = \`https://\${parsed.subdomain}.r2.dev\${parsed.filePath}\${url.search}\`;

  try {
    const response = await fetch(targetUrl, {
      method: request.method,
      headers: request.headers,
    });

    const headers = new Headers(response.headers);
    headers.set("Access-Control-Allow-Origin", ALLOWED_ORIGINS);
    headers.set("X-Proxy-By", "FileFlow-Deno");

    return new Response(response.body, {
      status: response.status,
      headers,
    });
  } catch {
    return new Response(JSON.stringify({ error: "Proxy failed" }), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }
}

console.log(\`Proxy listening on http://localhost:\${PORT}\`);
Deno.serve({ port: PORT }, handler);`;

const goCode = `/**
 * FileFlow R2 Endpoint Proxy - Go 版本
 *
 * 编译运行：
 *   go build -o proxy proxy.go && ./proxy
 *
 * 环境变量：
 *   PORT - 监听端口（默认 8787）
 */

package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var allowedOrigins = getEnv("ALLOWED_ORIGINS", "*")

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parsePath(path string) (subdomain, filePath string, ok bool) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", "", false
	}

	idx := strings.Index(path, "/")
	if idx == -1 {
		return path, "", true
	}

	return path[:idx], path[idx:], true
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, \`{"error":"Method not allowed"}\`, http.StatusMethodNotAllowed)
		return
	}

	subdomain, filePath, ok := parsePath(r.URL.Path)
	if !ok || subdomain == "" {
		http.Error(w, \`{"error":"Invalid path"}\`, http.StatusBadRequest)
		return
	}

	targetURL := "https://" + subdomain + ".r2.dev" + filePath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, _ := http.NewRequest(r.Method, targetURL, nil)
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		http.Error(w, \`{"error":"Proxy failed"}\`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Proxy-By", "FileFlow-Go")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	port := getEnv("PORT", "8787")
	http.HandleFunc("/", proxyHandler)
	log.Printf("Proxy listening on http://localhost:%s", port)
	http.ListenAndServe(":"+port, nil)
}`;

/**
 * 外置代理部署指南页面
 */
export default function ProxyGuide() {
  return (
    <div className="space-y-8 max-w-4xl mx-auto">
      <div>
        <h1 className="text-3xl font-bold">外置代理部署指南</h1>
        <p className="text-muted-foreground mt-2">
          部署独立的反向代理服务，隐藏 R2 源站地址，提升访问速度
        </p>
      </div>

      {/* 为什么使用外置代理 */}
      <Card>
        <CardHeader>
          <CardTitle>为什么使用外置代理？</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            FileFlow 内置了反向代理功能，但在某些场景下，使用独立的外置代理更有优势：
          </p>
          <ul className="list-disc list-inside space-y-2 text-sm">
            <li><strong>边缘加速</strong> - 使用 Cloudflare Workers 部署，利用全球边缘节点加速访问</li>
            <li><strong>减轻负载</strong> - 分离代理流量，避免影响 FileFlow 主服务性能</li>
            <li><strong>独立扩展</strong> - 可以单独扩展代理服务的资源和带宽</li>
            <li><strong>灵活部署</strong> - 支持多种运行环境：Workers、Deno、Go</li>
          </ul>
        </CardContent>
      </Card>

      {/* URL 格式说明 */}
      <Card>
        <CardHeader>
          <CardTitle>URL 格式</CardTitle>
          <CardDescription>代理服务的请求和转发格式</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 pr-4">类型</th>
                  <th className="text-left py-2">URL 格式</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                <tr>
                  <td className="py-2 pr-4 font-medium">代理请求</td>
                  <td className="py-2 text-muted-foreground font-mono text-xs">
                    https://your-proxy.com/<span className="text-primary">pub-xxx</span>/path/to/file.png
                  </td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">实际转发</td>
                  <td className="py-2 text-muted-foreground font-mono text-xs">
                    https://<span className="text-primary">pub-xxx</span>.r2.dev/path/to/file.png
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p className="text-xs text-muted-foreground">
            其中 <code className="bg-muted px-1 rounded">pub-xxx</code> 是 R2 存储桶的公开访问子域名
          </p>
        </CardContent>
      </Card>

      {/* 部署方式 */}
      <Card>
        <CardHeader>
          <CardTitle>选择部署方式</CardTitle>
          <CardDescription>根据你的需求选择合适的部署方案</CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="worker" className="w-full">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="worker">Cloudflare Workers</TabsTrigger>
              <TabsTrigger value="deno">Deno / Deno Deploy</TabsTrigger>
              <TabsTrigger value="go">Go 独立部署</TabsTrigger>
            </TabsList>

            {/* Cloudflare Workers */}
            <TabsContent value="worker" className="space-y-4 mt-4">
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="secondary">推荐</Badge>
                <Badge variant="outline">免费额度</Badge>
                <Badge variant="outline">全球边缘</Badge>
              </div>
              <div className="space-y-2">
                <h4 className="font-medium">部署步骤：</h4>
                <ol className="list-decimal list-inside space-y-1 text-sm text-muted-foreground">
                  <li>登录 <a href="https://dash.cloudflare.com" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline inline-flex items-center gap-1">Cloudflare Dashboard <ExternalLink className="h-3 w-3" /></a></li>
                  <li>进入 <strong>Workers & Pages</strong></li>
                  <li>点击 <strong>Create</strong> → <strong>Create Worker</strong></li>
                  <li>粘贴下方代码，点击 <strong>Deploy</strong></li>
                  <li>（可选）绑定自定义域名</li>
                </ol>
              </div>
              <CodeBlock code={workerCode} language="javascript" />
            </TabsContent>

            {/* Deno */}
            <TabsContent value="deno" className="space-y-4 mt-4">
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="outline">TypeScript</Badge>
                <Badge variant="outline">Deno Deploy 免费</Badge>
              </div>
              <div className="space-y-2">
                <h4 className="font-medium">本地运行：</h4>
                <CodeBlock code="deno run --allow-net --allow-env proxy.ts" language="bash" />
              </div>
              <div className="space-y-2">
                <h4 className="font-medium">Deno Deploy 部署：</h4>
                <ol className="list-decimal list-inside space-y-1 text-sm text-muted-foreground">
                  <li>访问 <a href="https://dash.deno.com" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline inline-flex items-center gap-1">Deno Deploy <ExternalLink className="h-3 w-3" /></a></li>
                  <li>创建新项目，连接 GitHub 仓库或直接粘贴代码</li>
                </ol>
              </div>
              <CodeBlock code={denoCode} language="typescript" />
            </TabsContent>

            {/* Go */}
            <TabsContent value="go" className="space-y-4 mt-4">
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant="outline">高性能</Badge>
                <Badge variant="outline">单二进制</Badge>
                <Badge variant="outline">Docker 友好</Badge>
              </div>
              <div className="space-y-2">
                <h4 className="font-medium">编译运行：</h4>
                <CodeBlock code={`go build -o proxy proxy.go\n./proxy`} language="bash" />
              </div>
              <div className="space-y-2">
                <h4 className="font-medium">Docker 运行：</h4>
                <CodeBlock code={`docker run -d -p 8787:8787 -e PORT=8787 your-proxy-image`} language="bash" />
              </div>
              <CodeBlock code={goCode} language="go" />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* 配置 FileFlow */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Badge className="h-6 w-6 rounded-full p-0 flex items-center justify-center">!</Badge>
            配置 FileFlow
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-muted-foreground">
            部署完成后，需要在 FileFlow 中配置代理 URL：
          </p>
          <ol className="list-decimal list-inside space-y-2 text-sm">
            <li>进入 <strong>设置</strong> → <strong>系统设置</strong></li>
            <li>启用 <strong>端点代理</strong></li>
            <li>填写 <strong>代理 URL</strong>，例如：
              <ul className="list-disc list-inside ml-4 mt-1 text-muted-foreground">
                <li>Workers: <code className="bg-muted px-1 rounded">https://your-worker.workers.dev</code></li>
                <li>自定义域名: <code className="bg-muted px-1 rounded">https://proxy.yourdomain.com</code></li>
              </ul>
            </li>
            <li>点击 <strong>保存设置</strong></li>
          </ol>
          <p className="text-xs text-muted-foreground">
            配置后，FileFlow 生成的文件链接将自动使用外置代理
          </p>
        </CardContent>
      </Card>

      {/* 对比表格 */}
      <Card>
        <CardHeader>
          <CardTitle>方案对比</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 pr-4">特性</th>
                  <th className="text-left py-2 pr-4">Workers</th>
                  <th className="text-left py-2 pr-4">Deno</th>
                  <th className="text-left py-2">Go</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                <tr>
                  <td className="py-2 pr-4 font-medium">部署难度</td>
                  <td className="py-2 pr-4 text-green-500">简单</td>
                  <td className="py-2 pr-4 text-green-500">简单</td>
                  <td className="py-2 text-yellow-500">中等</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">免费额度</td>
                  <td className="py-2 pr-4">10万次/天</td>
                  <td className="py-2 pr-4">100万次/月</td>
                  <td className="py-2 text-muted-foreground">自托管</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">边缘加速</td>
                  <td className="py-2 pr-4 text-green-500">✓ 全球边缘</td>
                  <td className="py-2 pr-4 text-green-500">✓ 全球边缘</td>
                  <td className="py-2 text-muted-foreground">单节点</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">冷启动</td>
                  <td className="py-2 pr-4 text-green-500">极快</td>
                  <td className="py-2 pr-4 text-green-500">极快</td>
                  <td className="py-2 text-green-500">无</td>
                </tr>
                <tr>
                  <td className="py-2 pr-4 font-medium">适用场景</td>
                  <td className="py-2 pr-4">通用</td>
                  <td className="py-2 pr-4">通用</td>
                  <td className="py-2">高流量/私有部署</td>
                </tr>
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
