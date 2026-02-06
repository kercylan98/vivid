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
    stdio: 'ignore', // 不 pipe，避免子进程占用的管道导致脚本不退出
    detached: process.platform !== 'win32',
  });
  let ok = false;
  try {
    await waitForReady();
    const res = await fetch(apiUrl);
    if (!res.ok) throw new Error(`GET ${apiUrl} returned ${res.status}`);
    const data = await res.json();
    await mkdir(outDir, { recursive: true });
    await writeFile(outFile, JSON.stringify(data), 'utf8');
    console.log('Written', outFile);
    ok = true;
  } finally {
    try {
      if (process.platform !== 'win32' && child.pid) {
        process.kill(-child.pid, 'SIGKILL'); // 杀进程组，确保 next dev 子进程一起退出
      } else {
        child.kill('SIGKILL');
      }
    } catch (_) {}
    // 不等待子进程，直接退出，避免 GitHub Actions 卡住
    setTimeout(() => process.exit(ok ? 0 : 1), 800);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
