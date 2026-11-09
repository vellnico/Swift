// Checks the gzip size of what a first visit downloads (HTML, JS, CSS; fonts
// are cached separately) against the budgets in SPEC §12, stage 11.
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const budgets = [
  { name: 'panel', dir: 'dist', limit: 150 * 1024 },
  { name: 'subscription page', dir: 'sub/dist', limit: 30 * 1024 },
];

const gz = (buf) => gzipSync(buf, { level: 9 }).length;
let failed = false;
for (const b of budgets) {
  const dir = join(root, b.dir);
  const html = readFileSync(join(dir, 'index.html'));
  const refs = [...html.toString().matchAll(/(?:src|href)="\.?\/?(assets\/[^"]+\.(?:js|css))"/g)].map((m) => m[1]);
  let total = gz(html);
  for (const ref of new Set(refs)) total += gz(readFileSync(join(dir, ref)));
  const ok = total <= b.limit;
  failed ||= !ok;
  console.log(`${ok ? 'ok  ' : 'FAIL'} ${b.name}: ${(total / 1024).toFixed(1)} KB gzip of ${b.limit / 1024} KB (${refs.length} assets)`);
}
if (failed) process.exit(1);
