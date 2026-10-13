/** Keep the external Node platform adapters beside the Wails executable. */
const fs = require('fs');
const path = require('path');

const source = __dirname;
const target = path.resolve(__dirname, '../build/bin/scripts');
fs.mkdirSync(target, { recursive: true });
for (const name of fs.readdirSync(source)) {
  if (!/\.(?:js|json)$/.test(name)) continue;
  fs.copyFileSync(path.join(source, name), path.join(target, name));
}
console.log('Synced platform adapters to build/bin/scripts');
