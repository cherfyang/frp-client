import { defineConfig, type Plugin } from 'vite';
import vue from '@vitejs/plugin-vue';
import {
  FrpcControlError,
  installManagedFrpc,
  readProjectFrpcConfig,
  restartManagedFrpc,
  saveProjectFrpcConfig,
} from './scripts/frpc-control';

const frpcControlPlugin = (): Plugin => ({
  name: 'frpc-control-plugin',
  configureServer(server) {
    const respondJson = (
      res: {
        statusCode: number;
        setHeader(name: string, value: string): void;
        end(body: string): void;
      },
      statusCode: number,
      payload: Record<string, unknown>,
    ) => {
      res.statusCode = statusCode;
      res.setHeader('Content-Type', 'application/json; charset=utf-8');
      res.end(JSON.stringify(payload));
    };

    const handlePost = (
      route: string,
      action: () => Promise<Record<string, unknown>>,
      fallbackMessage: string,
    ) => {
      server.middlewares.use(route, async (req, res, next) => {
        if (req.method !== 'POST') {
          next();
          return;
        }

        try {
          const result = await action();
          respondJson(res, 200, { success: true, ...result });
        } catch (error) {
          respondJson(res, 500, {
            success: false,
            error: error instanceof Error ? error.message : fallbackMessage,
            code: error instanceof FrpcControlError ? error.code : undefined,
          });
        }
      });
    };

    const handleGet = (
      route: string,
      action: () => Promise<Record<string, unknown>>,
      fallbackMessage: string,
    ) => {
      server.middlewares.use(route, async (req, res, next) => {
        if (req.method !== 'GET') {
          next();
          return;
        }

        try {
          const result = await action();
          respondJson(res, 200, result);
        } catch (error) {
          respondJson(res, 500, {
            error: error instanceof Error ? error.message : fallbackMessage,
            code: error instanceof FrpcControlError ? error.code : undefined,
          });
        }
      });
    };

    const readJsonBody = async (req: NodeJS.ReadableStream) =>
      new Promise<Record<string, unknown>>((resolve, reject) => {
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
            resolve(JSON.parse(raw) as Record<string, unknown>);
          } catch (error) {
            reject(new Error(error instanceof Error ? error.message : 'JSON 解析失败。'));
          }
        });

        req.on('error', (error) => {
          reject(error);
        });
      });

    const handleJsonPost = (
      route: string,
      action: (body: Record<string, unknown>) => Promise<Record<string, unknown>>,
      fallbackMessage: string,
    ) => {
      server.middlewares.use(route, async (req, res, next) => {
        if (req.method !== 'POST') {
          next();
          return;
        }

        try {
          const body = await readJsonBody(req);
          const result = await action(body);
          respondJson(res, 200, result);
        } catch (error) {
          respondJson(res, 500, {
            error: error instanceof Error ? error.message : fallbackMessage,
            code: error instanceof FrpcControlError ? error.code : undefined,
          });
        }
      });
    };

    handleGet('/api/frp/config', () => readProjectFrpcConfig(server.config.root), '读取 frpc.toml 失败。');
    handleJsonPost(
      '/api/frp/config/save',
      async (body) => {
        if (typeof body.content !== 'string') {
          throw new Error('缺少 content 文本内容。');
        }

        return saveProjectFrpcConfig(server.config.root, body.content);
      },
      '保存 frpc.toml 失败。',
    );
    handlePost('/api/frp/restart', () => restartManagedFrpc(server.config.root), '重启 frp 服务失败。');
    handlePost('/api/frp/install', () => installManagedFrpc(server.config.root), '安装 frpc 失败。');
  },
});

export default defineConfig({
  plugins: [vue(), frpcControlPlugin()],
});
