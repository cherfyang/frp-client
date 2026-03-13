import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  ensureConfigWorkspace,
  listStoredConfigs,
  readCurrentConfig,
  readStoredConfig,
  saveConfigToList,
  saveCurrentConfig,
  saveStoredConfig,
} from './config-store.mjs';
import {
  FrpcControlError,
  installManagedFrpc,
  restartManagedFrpc,
} from './frp-service.mjs';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, '..');
const distDir = path.join(rootDir, 'dist');
const host = process.env.HOST || '0.0.0.0';
const port = Number(process.env.PORT || 6633);

const mimeTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.txt': 'text/plain; charset=utf-8',
};

const sendJson = (res, statusCode, payload) => {
  res.writeHead(statusCode, {
    'Content-Type': 'application/json; charset=utf-8',
  });
  res.end(JSON.stringify(payload));
};

const sendText = (res, statusCode, body) => {
  res.writeHead(statusCode, {
    'Content-Type': 'text/plain; charset=utf-8',
  });
  res.end(body);
};

const readJsonBody = async (req) =>
  new Promise((resolve, reject) => {
    let raw = '';

    req.on('data', (chunk) => {
      raw += chunk.toString();
    });

    req.on('end', () => {
      if (!raw.trim()) {
        resolve({});
        return;
      }

      try {
        resolve(JSON.parse(raw));
      } catch (error) {
        reject(new Error(error instanceof Error ? error.message : 'JSON 解析失败。'));
      }
    });

    req.on('error', (error) => {
      reject(error);
    });
  });

const handleApiError = (res, error, fallbackMessage) => {
  sendJson(res, 500, {
    success: false,
    error: error instanceof Error ? error.message : fallbackMessage,
    code: error instanceof FrpcControlError ? error.code : undefined,
  });
};

const serveStaticFile = async (req, res, pathname) => {
  const targetPath = pathname === '/'
    ? path.join(distDir, 'index.html')
    : path.join(distDir, pathname.replace(/^\/+/, ''));

  const normalizedPath = path.normalize(targetPath);
  if (!normalizedPath.startsWith(distDir)) {
    sendText(res, 403, 'Forbidden');
    return;
  }

  try {
    const fileStat = await stat(normalizedPath);
    const finalPath = fileStat.isDirectory() ? path.join(normalizedPath, 'index.html') : normalizedPath;
    const body = await readFile(finalPath);
    const extension = path.extname(finalPath).toLowerCase();

    res.writeHead(200, {
      'Content-Type': mimeTypes[extension] || 'application/octet-stream',
      'Cache-Control': pathname.startsWith('/assets/') ? 'public, max-age=31536000, immutable' : 'no-cache',
    });
    res.end(body);
  } catch {
    try {
      const indexHtml = await readFile(path.join(distDir, 'index.html'));
      res.writeHead(200, {
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'no-cache',
      });
      res.end(indexHtml);
    } catch {
      sendText(res, 500, 'dist 目录不存在，请先执行 pnpm build。');
    }
  }
};

const server = createServer(async (req, res) => {
  if (!req.url || !req.method) {
    sendText(res, 400, 'Bad Request');
    return;
  }

  const url = new URL(req.url, `http://${req.headers.host || `${host}:${port}`}`);
  const pathname = url.pathname;

  if (req.method === 'GET' && pathname === '/api/config/current') {
    try {
      sendJson(res, 200, await readCurrentConfig(rootDir));
    } catch (error) {
      handleApiError(res, error, '读取当前配置失败。');
    }
    return;
  }

  if (req.method === 'POST' && pathname === '/api/config/current/save') {
    try {
      const body = await readJsonBody(req);
      if (typeof body.content !== 'string') {
        throw new Error('缺少 content 文本内容。');
      }

      sendJson(res, 200, await saveCurrentConfig(rootDir, body.content));
    } catch (error) {
      handleApiError(res, error, '保存当前配置失败。');
    }
    return;
  }

  if (req.method === 'GET' && pathname === '/api/config/list') {
    try {
      sendJson(res, 200, await listStoredConfigs(rootDir));
    } catch (error) {
      handleApiError(res, error, '读取配置列表失败。');
    }
    return;
  }

  const storedMatch = pathname.match(/^\/api\/config\/list\/(\d+)$/);
  if (req.method === 'GET' && storedMatch) {
    try {
      sendJson(res, 200, await readStoredConfig(rootDir, Number(storedMatch[1])));
    } catch (error) {
      handleApiError(res, error, '读取已保存配置失败。');
    }
    return;
  }

  const storedSaveMatch = pathname.match(/^\/api\/config\/list\/(\d+)\/save$/);
  if (req.method === 'POST' && storedSaveMatch) {
    try {
      const body = await readJsonBody(req);
      if (typeof body.content !== 'string') {
        throw new Error('缺少 content 文本内容。');
      }

      sendJson(res, 200, await saveStoredConfig(rootDir, Number(storedSaveMatch[1]), body.content));
    } catch (error) {
      handleApiError(res, error, '保存配置槽位失败。');
    }
    return;
  }

  if (req.method === 'POST' && pathname === '/api/config/list/save') {
    try {
      const body = await readJsonBody(req);
      if (typeof body.content !== 'string') {
        throw new Error('缺少 content 文本内容。');
      }

      sendJson(res, 200, await saveConfigToList(rootDir, body));
    } catch (error) {
      handleApiError(res, error, '保存到配置列表失败。');
    }
    return;
  }

  if (req.method === 'POST' && pathname === '/api/frp/install') {
    try {
      sendJson(res, 200, await installManagedFrpc(rootDir));
    } catch (error) {
      handleApiError(res, error, '安装 frpc 失败。');
    }
    return;
  }

  if (req.method === 'POST' && pathname === '/api/frp/restart') {
    try {
      sendJson(res, 200, await restartManagedFrpc(rootDir));
    } catch (error) {
      handleApiError(res, error, '重启 frp 服务失败。');
    }
    return;
  }

  await serveStaticFile(req, res, pathname);
});

await ensureConfigWorkspace(rootDir);

server.listen(port, host, () => {
  console.log(`frpc client editor backend ready: http://${host}:${port}`);
});
