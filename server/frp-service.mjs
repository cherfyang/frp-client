import { constants as fsConstants } from 'node:fs';
import { access, open, readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { spawn, spawnSync } from 'node:child_process';
import { getCurrentConfigPath } from './config-store.mjs';

export const FRPC_BINARY_MISSING_CODE = 'frpc_binary_missing';
const FRPC_INSTALL_FAILED_CODE = 'frpc_install_failed';
const FRPC_MISSING_MESSAGE = '未找到 frpc 可执行文件。请先安装 frpc，或设置环境变量 FRPC_BIN 指向可执行文件。';

export class FrpcControlError extends Error {
  constructor(code, message) {
    super(message);
    this.name = 'FrpcControlError';
    this.code = code;
  }
}

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const getLocalFrpcBinaryPath = (rootDir) => {
  const platform = process.platform === 'win32' ? 'windows'
    : process.platform === 'darwin' ? 'darwin'
    : 'linux';
  const ext = process.platform === 'win32' ? '.exe' : '';
  return path.join(rootDir, '.tools', platform, `frpc${ext}`);
};

const getInstallerScriptPath = (rootDir) => path.join(rootDir, 'scripts', 'setup-frpc.sh');

const probeBinary = (binary) => {
  const result = spawnSync(binary, ['-v'], {
    stdio: 'ignore',
  });

  return !result.error;
};

const isProcessAlive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};

const readPidFile = async (pidFilePath) => {
  try {
    const content = await readFile(pidFilePath, 'utf8');
    const pid = Number(content.trim());
    return Number.isInteger(pid) && pid > 0 ? pid : null;
  } catch {
    return null;
  }
};

const stopManagedFrpc = async (pidFilePath) => {
  const pid = await readPidFile(pidFilePath);
  if (!pid) {
    await rm(pidFilePath, { force: true });
    return null;
  }

  if (!isProcessAlive(pid)) {
    await rm(pidFilePath, { force: true });
    return pid;
  }

  process.kill(pid, 'SIGTERM');

  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (!isProcessAlive(pid)) {
      await rm(pidFilePath, { force: true });
      return pid;
    }

    await wait(100);
  }

  process.kill(pid, 'SIGKILL');
  await rm(pidFilePath, { force: true });
  return pid;
};

const readRecentLogSnippet = async (logFilePath) => {
  try {
    const content = await readFile(logFilePath, 'utf8');
    return content
      .trim()
      .split(/\r?\n/)
      .filter(Boolean)
      .slice(-5)
      .join('\n');
  } catch {
    return '';
  }
};

export const resolveFrpcBinary = async (rootDir) => {
  const fromEnv = process.env.FRPC_BIN?.trim();
  if (fromEnv) {
    if (!probeBinary(fromEnv)) {
      throw new FrpcControlError(FRPC_BINARY_MISSING_CODE, FRPC_MISSING_MESSAGE);
    }

    return fromEnv;
  }

  const localBinaryPath = getLocalFrpcBinaryPath(rootDir);
  try {
    await access(localBinaryPath, fsConstants.X_OK);
    if (probeBinary(localBinaryPath)) {
      return localBinaryPath;
    }
  } catch {
    // Local project binary is optional.
  }

  if (probeBinary('frpc')) {
    return 'frpc';
  }

  throw new FrpcControlError(FRPC_BINARY_MISSING_CODE, FRPC_MISSING_MESSAGE);
};

const waitForStableStart = async (child, pidFilePath, logFilePath) =>
  new Promise((resolve, reject) => {
    let settled = false;

    const finish = (handler) => {
      if (settled) {
        return;
      }

      settled = true;
      clearTimeout(stableTimer);
      child.removeListener('error', onError);
      child.removeListener('exit', onExit);
      handler();
    };

    const onError = (error) => {
      finish(() => {
        reject(new Error(`frpc 启动失败：${error.message}`));
      });
    };

    const onExit = (code, signal) => {
      void (async () => {
        await rm(pidFilePath, { force: true });
        const logSnippet = await readRecentLogSnippet(logFilePath);
        const exitReason = code !== null
          ? `exit code ${code}`
          : signal
            ? `signal ${signal}`
            : '未知原因';
        const message = logSnippet
          ? `frpc 启动后立即退出（${exitReason}）。最近日志：${logSnippet}`
          : `frpc 启动后立即退出（${exitReason}）。`;

        finish(() => {
          reject(new Error(message));
        });
      })();
    };

    const stableTimer = setTimeout(() => {
      finish(resolve);
    }, 1000);

    child.once('error', onError);
    child.once('exit', onExit);
  });

const runInstallerScript = async (rootDir) =>
  new Promise((resolve, reject) => {
    const scriptPath = getInstallerScriptPath(rootDir);
    const isWindows = process.platform === 'win32';
    const child = isWindows
      ? spawn('powershell', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', scriptPath.replace('.sh', '.ps1')], {
          cwd: rootDir,
          env: process.env,
          stdio: ['ignore', 'pipe', 'pipe'],
        })
      : spawn('bash', [scriptPath], {
          cwd: rootDir,
          env: process.env,
          stdio: ['ignore', 'pipe', 'pipe'],
        });

    let stdout = '';
    let stderr = '';

    child.stdout.on('data', (chunk) => {
      stdout += chunk.toString();
    });

    child.stderr.on('data', (chunk) => {
      stderr += chunk.toString();
    });

    child.on('error', (error) => {
      reject(new FrpcControlError(FRPC_INSTALL_FAILED_CODE, `执行安装脚本失败：${error.message}`));
    });

    child.on('close', (code) => {
      if (code === 0) {
        resolve({ stdout, stderr });
        return;
      }

      const message = stderr.trim() || stdout.trim() || '安装 frpc 失败。';
      reject(new FrpcControlError(FRPC_INSTALL_FAILED_CODE, message));
    });
  });

export const installManagedFrpc = async (rootDir) => {
  const result = await runInstallerScript(rootDir);
  const binaryPath = await resolveFrpcBinary(rootDir);

  return {
    success: true,
    ...result,
    binaryPath,
  };
};

export const restartManagedFrpc = async (rootDir) => {
  const frpcBinary = await resolveFrpcBinary(rootDir);
  const configPath = process.env.FRPC_CONFIG || getCurrentConfigPath(rootDir);
  const pidFilePath = path.join(rootDir, '.frpc.pid');
  const logFilePath = path.join(rootDir, '.frpc.log');

  const stoppedPid = await stopManagedFrpc(pidFilePath);
  const logFile = await open(logFilePath, 'a');
  const child = spawn(frpcBinary, ['-c', configPath], {
    cwd: rootDir,
    detached: true,
    stdio: ['ignore', logFile.fd, logFile.fd],
  });

  child.unref();
  await logFile.close();

  if (!child.pid) {
    throw new Error('frpc 进程启动失败，未获取到 PID。');
  }

  await writeFile(pidFilePath, `${child.pid}\n`, 'utf8');
  await waitForStableStart(child, pidFilePath, logFilePath);

  return {
    success: true,
    pid: child.pid,
    stoppedPid,
    configPath,
    logFilePath,
    binaryPath: frpcBinary,
  };
};
