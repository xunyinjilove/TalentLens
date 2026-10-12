const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..');
const runs = path.join(root, '.e2e-data');
fs.mkdirSync(runs, { recursive: true });
const dataDir = fs.mkdtempSync(path.join(runs, 'run-'));
const child = spawn(path.join(root, 'bin', 'talentlens-e2e.exe'), ['-test.run=^TestE2EServer$', '-test.timeout=0'], {
  cwd: root, stdio: 'inherit', windowsHide: true,
  env: {
    ...process.env,
    TALENTLENS_E2E_DATA_DIR: dataDir,
    assetdir: path.join(root, 'frontend', 'dist'),
    devserver: '127.0.0.1:34116',
  },
});
console.log(`Isolated E2E data: ${dataDir}`);
const stop = () => child.kill();
process.on('SIGINT', stop);
process.on('SIGTERM', stop);
process.on('exit', stop);
child.on('error', error => { console.error(error); process.exit(1); });
child.on('exit', code => process.exit(code ?? 0));
