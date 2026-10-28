// Writes the page FleetWatch embeds: index.html, fleet.js, fleet.css and
// theme.css under internal/ui/dist. The output is committed and CI rebuilds it
// and fails on any difference, so the Go build needs no Node. Run it from
// anywhere: `node web/build.mjs`.
import { createRequire } from 'node:module';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, '..', 'internal', 'ui', 'dist');
const { fleetPageHTML, fleetClientScript } = require('./page.js');

// The routes FleetWatch serves; the page and its script are built against them.
const urls = { streamUrl: '/v1/events', snapshotUrl: '/v1/fleet', sessionUrl: '/v1/session' };

mkdirSync(out, { recursive: true });
writeFileSync(join(out, 'index.html'), fleetPageHTML('', { ...urls, assets: '/ui/' }) + '\n');
writeFileSync(join(out, 'fleet.js'), fleetClientScript(urls).trimStart());
for (const f of ['fleet.css', 'theme.css']) {
  writeFileSync(join(out, f), readFileSync(join(here, f)));
}
console.log('wrote ' + out);
