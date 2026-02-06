/**
 * 构建前导出搜索索引到 public/search-index.json，供客户端静态加载（不走服务端 API）。
 * 通过临时启动 next dev 拉取 /api/search 的 staticGET 响应并写入 public。
 */
import { spawn } from 'node:child_process';
import { mkdir, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const basePath = process.env.BASE_PATH ?? '';
const port = Number(process.env.PORT) || 3010;
const apiUrl = `http://127.0.0.1:${port}${basePath}/api/search`;
const outDir = path.join(root, 'public');
const outFile = path.join(outDir, 'search-index.json');

async function waitForReady() {
  const maxAttempts = 60;
  for (let i = 0; i < maxAttempts; i++) {
    try {
      const res = await fetch(apiUrl);
      if (res.ok) return res;
    } catch (_) {}
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw new Error(`Timeout: ${apiUrl} did not become ready`);
}

async function main() {
  const child = spawn('pnpm', ['run', 'dev'], {
    cwd: root,
    env: { ...process.env, PORT: String(port) },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let stderr = '';
  child.stderr?.on('data', (chunk) => { stderr += chunk; });
  try {
    await waitForReady();
    const res = await fetch(apiUrl);
    if (!res.ok) throw new Error(`GET ${apiUrl} returned ${res.status}`);
    const data = await res.json();
    await mkdir(outDir, { recursive: true });
    await writeFile(outFile, JSON.stringify(data), 'utf8');
    console.log('Written', outFile);
  } finally {
    child.kill('SIGKILL');
    // 不 await 子进程退出，避免 CI 上 pnpm/next 子进程树未完全退出导致卡住
    await Promise.race([
      new Promise((resolve) => child.on('exit', resolve)),
      new Promise((r) => setTimeout(r, 3000)),
    ]);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
