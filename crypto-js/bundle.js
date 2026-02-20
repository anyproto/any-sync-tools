#!/usr/bin/env node
// bundle.js — Embeds slip10.compiled.js into index.html → crypto.compiled.html

const fs = require('fs');
const path = require('path');

const dir = __dirname;
const js  = fs.readFileSync(path.join(dir, 'slip10.compiled.js'), 'utf8');
let html  = fs.readFileSync(path.join(dir, 'index.html'), 'utf8');

html = html.replace(
  '<script src="slip10.compiled.js"></script>',
  '<script>\n' + js + '\n</script>'
);

fs.writeFileSync(path.join(dir, 'crypto.compiled.html'), html);
console.log('Done → crypto.compiled.html');
