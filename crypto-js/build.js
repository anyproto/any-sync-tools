#!/usr/bin/env node
// build.js — Injects words.txt into slip10.js → slip10.compiled.js

const fs = require('fs');
const path = require('path');

const dir   = __dirname;
const words = fs.readFileSync(path.join(dir, 'words.txt'), 'utf8')
  .split('\n').map(w => w.trim()).filter(Boolean);

if (words.length !== 2048) {
  console.error(`Expected 2048 words, got ${words.length}`);
  process.exit(1);
}

let src = fs.readFileSync(path.join(dir, 'slip10.js'), 'utf8');
const placeholder = '/*__WORDLIST__*/[]';
if (!src.includes(placeholder)) {
  console.error('Placeholder not found in slip10.js');
  process.exit(1);
}

src = src.replace(placeholder, JSON.stringify(words));
fs.writeFileSync(path.join(dir, 'slip10.compiled.js'), src);
console.log(`Done — injected ${words.length} words → slip10.compiled.js`);
