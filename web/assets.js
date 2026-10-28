// The two stylesheets, read from beside this file. fleet.css is structure and
// uses only --fl-* tokens; theme.css gives those tokens their values and maps
// model classes to colours. They are separate files so a host can replace the
// theme without touching the structure.

const fs = require('fs');
const path = require('path');

function read(name) {
  return fs.readFileSync(path.join(__dirname, name), 'utf8');
}

function fleetCss() { return read('fleet.css'); }
function themeCss() { return read('theme.css'); }

module.exports = { fleetCss, themeCss };
