/**
 * FileFlow R2 Endpoint Proxy - Cloudflare Worker 版本
 *
 * 用于反向代理 R2 公开文件，隐藏源站地址
 *
 * URL 格式：
 *   请求: https://your-worker.workers.dev/pub-xxx/path/to/file.png
 *   代理: https://pub-xxx.r2.dev/path/to/file.png
 *
 * 部署方式：
 *   1. 登录 Cloudflare Dashboard
 *   2. 进入 Workers & Pages
 *   3. 创建 Worker，粘贴此代码
 *   4. 部署
 */

const ALLOWED_ORIGINS = "*";

/**
 * 解析请求路径，提取 subdomain 和文件路径
 * @param {string} pathname
 * @returns {{ subdomain: string, filePath: string } | null}
 */
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

/**
 * 处理 CORS 预检请求
 * @param {Request} request
 * @returns {Response | null}
 */
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

/**
 * 处理代理请求
 * @param {Request} request
 * @returns {Promise<Response>}
 */
async function handleProxy(request) {
  const url = new URL(request.url);
  const parsed = parsePath(url.pathname);

  if (!parsed || !parsed.subdomain) {
    return new Response(JSON.stringify({ error: "Invalid path" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  const targetUrl = `https://${parsed.subdomain}.r2.dev${parsed.filePath}${url.search}`;

  try {
    const proxyRequest = new Request(targetUrl, {
      method: request.method,
      headers: request.headers,
      redirect: "follow",
    });

    const response = await fetch(proxyRequest);

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
  /**
   * @param {Request} request
   * @param {object} env
   * @param {object} ctx
   * @returns {Promise<Response>}
   */
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
};
