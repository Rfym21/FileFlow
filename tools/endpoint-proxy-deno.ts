/**
 * FileFlow R2 Endpoint Proxy - Deno 版本
 *
 * 用于反向代理 R2 公开文件，隐藏源站地址
 *
 * URL 格式：
 *   请求: https://your-domain.com/pub-xxx/path/to/file.png
 *   代理: https://pub-xxx.r2.dev/path/to/file.png
 *
 * 运行方式：
 *   deno run --allow-net endpoint-proxy-deno.ts
 *
 * 环境变量：
 *   PORT - 监听端口（默认 8787）
 *   ALLOWED_ORIGINS - 允许的 CORS 源（默认 *）
 */

const PORT = parseInt(Deno.env.get("PORT") || "8787");
const ALLOWED_ORIGINS = Deno.env.get("ALLOWED_ORIGINS") || "*";

/**
 * 解析请求路径，提取 subdomain 和文件路径
 */
function parsePath(pathname: string): { subdomain: string; filePath: string } | null {
  // 移除开头的斜杠
  const path = pathname.startsWith("/") ? pathname.slice(1) : pathname;
  if (!path) return null;

  // 第一段是 subdomain，剩余是文件路径
  const firstSlash = path.indexOf("/");
  if (firstSlash === -1) {
    return { subdomain: path, filePath: "" };
  }

  return {
    subdomain: path.slice(0, firstSlash),
    filePath: path.slice(firstSlash),
  };
}

/**
 * 处理 CORS 预检请求
 */
function handleCors(request: Request): Response | null {
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

/**
 * 处理代理请求
 */
async function handleProxy(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const parsed = parsePath(url.pathname);

  if (!parsed || !parsed.subdomain) {
    return new Response(JSON.stringify({ error: "Invalid path" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  // 构建目标 URL
  const targetUrl = `https://${parsed.subdomain}.r2.dev${parsed.filePath}${url.search}`;

  try {
    // 转发请求
    const proxyRequest = new Request(targetUrl, {
      method: request.method,
      headers: request.headers,
      redirect: "follow",
    });

    const response = await fetch(proxyRequest);

    // 创建新响应，添加 CORS 头
    const headers = new Headers(response.headers);
    headers.set("Access-Control-Allow-Origin", ALLOWED_ORIGINS);
    headers.set("X-Proxy-By", "FileFlow-Deno");

    return new Response(response.body, {
      status: response.status,
      statusText: response.statusText,
      headers,
    });
  } catch (error) {
    console.error("Proxy error:", error);
    return new Response(JSON.stringify({ error: "Proxy failed" }), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }
}

/**
 * 主请求处理器
 */
async function handler(request: Request): Promise<Response> {
  // 处理 CORS 预检
  const corsResponse = handleCors(request);
  if (corsResponse) return corsResponse;

  // 只允许 GET 和 HEAD 请求
  if (request.method !== "GET" && request.method !== "HEAD") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers: { "Content-Type": "application/json" },
    });
  }

  return await handleProxy(request);
}

// 启动服务器
console.log(`FileFlow R2 Proxy listening on http://localhost:${PORT}`);
Deno.serve({ port: PORT }, handler);
