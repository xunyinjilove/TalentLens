const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const cache = path.join(root, '.e2e-data', 'go-cache');
fs.mkdirSync(cache, { recursive: true });
const env = { ...process.env, GOCACHE: process.env.GOCACHE || cache };
function run(command, args) {
  const result = spawnSync(command, args, { cwd: root, env, stdio: 'inherit', windowsHide: true });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status || 1);
}
if (!process.env.npm_execpath) throw new Error('Run this command through npm run test:e2e:prepare');
run(process.execPath, [process.env.npm_execpath, '--prefix', 'frontend', 'run', 'build']);
run('go', ['test', '-c', '-tags', 'dev,e2e', '-o', 'bin/talentlens-e2e.exe', '.']);
