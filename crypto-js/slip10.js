// slip10.js — SLIP-0010 Ed25519 HD Key Derivation + BIP-39 Mnemonics
// Zero dependencies. Uses Web Crypto API + BigInt.
//
// All derivation methods are async (Web Crypto is async by design).
//
// Public API:
//   deriveForPath(path, seed)        → Promise<Node>
//   newMasterNode(seed)              → Promise<Node>
//   unmarshalNode(bytes)             → Node
//   isValidPath(path)                → boolean
//   mnemonicToSeed(mnemonic, pass?)  → Promise<Uint8Array>
//   generateMnemonic(strength?)      → Promise<string>
//   validateMnemonic(mnemonic)       → Promise<boolean>
//   verify(msg, sig, publicKey)      → Promise<boolean>
//   hexToBytes / bytesToHex          → conversion helpers
//   FIRST_HARDENED_INDEX             → 0x80000000

(function (root, factory) {
  if (typeof module !== 'undefined' && module.exports) module.exports = factory();
  else if (typeof define === 'function' && define.amd) define(factory);
  else root.slip10 = factory();
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  'use strict';

  // ── BIP-39 wordlist (injected by build.js) ───────────────────
  const WORDLIST = /*__WORDLIST__*/[];

  // ── Web Crypto backend ────────────────────────────────────────
  const _crypto = typeof globalThis !== 'undefined' && globalThis.crypto
    ? globalThis.crypto
    : (typeof crypto !== 'undefined' ? crypto : undefined);
  if (!_crypto || !_crypto.subtle) throw new Error('Web Crypto API not available');
  const subtle = _crypto.subtle;

  // ════════════════════════════════════════════════════════════════
  //  Ed25519 — pure BigInt implementation
  // ════════════════════════════════════════════════════════════════

  const ED_P = 2n ** 255n - 19n;
  const ED_L = 2n ** 252n + 27742317777372353535851937790883648493n;

  function mod(a, m) {
    if (m === undefined) m = ED_P;
    return ((a % m) + m) % m;
  }

  function modPow(base, exp, m) {
    if (m === undefined) m = ED_P;
    let r = 1n;
    base = mod(base, m);
    while (exp > 0n) {
      if (exp & 1n) r = mod(r * base, m);
      exp >>= 1n;
      base = mod(base * base, m);
    }
    return r;
  }

  function modInv(a) { return modPow(a, ED_P - 2n); }

  // Curve constant d = -121665/121666 mod p
  const CURVE_D = mod(-121665n * modInv(121666n));
  // sqrt(-1) mod p
  const SQRT_M1 = modPow(2n, (ED_P - 1n) / 4n);

  // Base point: y = 4/5 mod p, x = positive square root
  const BASE_Y = mod(4n * modInv(5n));
  const BASE_X = (function () {
    const y2 = mod(BASE_Y * BASE_Y);
    const x2 = mod((y2 - 1n) * modInv(mod(1n + CURVE_D * y2)));
    let x = modPow(x2, (ED_P + 3n) / 8n);
    if (mod(x * x) !== x2) x = mod(x * SQRT_M1);
    if (mod(x * x) !== x2) throw new Error('base point x: no sqrt');
    if (x & 1n) x = ED_P - x; // positive = even
    return x;
  })();

  // Extended coordinates: (X, Y, Z, T)  x=X/Z  y=Y/Z  xy=T/Z
  const ZERO_PT = [0n, 1n, 1n, 0n];
  const BASE_PT = [BASE_X, BASE_Y, 1n, mod(BASE_X * BASE_Y)];

  function ptAdd(a, b) {
    const [X1, Y1, Z1, T1] = a;
    const [X2, Y2, Z2, T2] = b;
    const A  = mod(X1 * X2);
    const B  = mod(Y1 * Y2);
    const C  = mod(CURVE_D * mod(T1 * T2));
    const ZZ = mod(Z1 * Z2);
    const E  = mod(mod((X1 + Y1) * (X2 + Y2)) - A - B);
    const F  = mod(ZZ - C);
    const G  = mod(ZZ + C);
    const H  = mod(B + A);
    return [mod(E * F), mod(G * H), mod(F * G), mod(E * H)];
  }

  function ptDouble(p) {
    const [X1, Y1, Z1] = p;
    const A  = mod(X1 * X1);
    const B  = mod(Y1 * Y1);
    const C  = mod(2n * mod(Z1 * Z1));
    const aA = mod(-A);
    const E  = mod(mod((X1 + Y1) * (X1 + Y1)) - A - B);
    const G  = mod(aA + B);
    const F  = mod(G - C);
    const H  = mod(aA - B);
    return [mod(E * F), mod(G * H), mod(F * G), mod(E * H)];
  }

  function scalarMult(s, pt) {
    let R = ZERO_PT;
    let Q = pt;
    while (s > 0n) {
      if (s & 1n) R = ptAdd(R, Q);
      Q = ptDouble(Q);
      s >>= 1n;
    }
    return R;
  }

  function ptEncode(pt) {
    const [X, Y, Z] = pt;
    const zi = modInv(Z);
    const x = mod(X * zi);
    const y = mod(Y * zi);
    const out = new Uint8Array(32);
    let v = y;
    for (let i = 0; i < 32; i++) { out[i] = Number(v & 0xffn); v >>= 8n; }
    if (x & 1n) out[31] |= 0x80;
    return out;
  }

  function bigIntToBytes32LE(n) {
    const b = new Uint8Array(32);
    for (let i = 0; i < 32; i++) { b[i] = Number(n & 0xffn); n >>= 8n; }
    return b;
  }

  function bytesToBigIntLE(b) {
    let v = 0n;
    for (let i = b.length - 1; i >= 0; i--) v = (v << 8n) | BigInt(b[i]);
    return v;
  }

  function concat() {
    let len = 0;
    for (let i = 0; i < arguments.length; i++) len += arguments[i].length;
    const r = new Uint8Array(len);
    let off = 0;
    for (let i = 0; i < arguments.length; i++) { r.set(arguments[i], off); off += arguments[i].length; }
    return r;
  }

  function ptDecode(bytes) {
    const b = new Uint8Array(bytes);
    const sign = (b[31] >> 7) & 1;
    b[31] &= 0x7f;
    const y = bytesToBigIntLE(b);
    if (y >= ED_P) throw new Error('invalid point: y >= p');
    const y2 = mod(y * y);
    const x2 = mod((y2 - 1n) * modInv(mod(1n + CURVE_D * y2)));
    if (x2 === 0n) {
      if (sign) throw new Error('invalid point: x=0 but sign set');
      return [0n, y, 1n, 0n];
    }
    let x = modPow(x2, (ED_P + 3n) / 8n);
    if (mod(x * x) !== x2) x = mod(x * SQRT_M1);
    if (mod(x * x) !== x2) throw new Error('invalid point: no sqrt');
    if ((Number(x & 1n)) !== sign) x = ED_P - x;
    return [x, y, 1n, mod(x * y)];
  }

  function ptEqual(a, b) {
    return mod(a[0] * b[2]) === mod(b[0] * a[2]) &&
           mod(a[1] * b[2]) === mod(b[1] * a[2]);
  }

  function clampScalar(h) {
    const a = new Uint8Array(h.slice(0, 32));
    a[0]  &= 248;
    a[31] &= 127;
    a[31] |= 64;
    return a;
  }

  // ── CRC-16-XMODEM ─────────────────────────────────────────────

  function crc16xmodem(data) {
    let crc = 0x0000;
    for (let i = 0; i < data.length; i++) {
      crc ^= data[i] << 8;
      for (let j = 0; j < 8; j++)
        crc = (crc & 0x8000) ? ((crc << 1) ^ 0x1021) & 0xffff : (crc << 1) & 0xffff;
    }
    return crc;
  }

  // ── Base58 (Bitcoin alphabet) ─────────────────────────────────

  const B58 = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';

  function base58Encode(bytes) {
    let zeros = 0;
    while (zeros < bytes.length && bytes[zeros] === 0) zeros++;
    let n = 0n;
    for (let i = 0; i < bytes.length; i++) n = n * 256n + BigInt(bytes[i]);
    let out = '';
    while (n > 0n) { out = B58[Number(n % 58n)] + out; n /= 58n; }
    return '1'.repeat(zeros) + out;
  }

  function base58Decode(str) {
    let zeros = 0;
    while (zeros < str.length && str[zeros] === '1') zeros++;
    let n = 0n;
    for (let i = 0; i < str.length; i++) {
      const c = B58.indexOf(str[i]);
      if (c === -1) throw new Error('invalid base58 character: ' + str[i]);
      n = n * 58n + BigInt(c);
    }
    const hex = n === 0n ? '' : n.toString(16);
    const padded = hex.length % 2 ? '0' + hex : hex;
    const dataBytes = new Uint8Array(padded.length / 2);
    for (let i = 0; i < dataBytes.length; i++) dataBytes[i] = parseInt(padded.substr(i * 2, 2), 16);
    const out = new Uint8Array(zeros + dataBytes.length);
    out.set(dataBytes, zeros);
    return out;
  }

  /** Decode an account address (Base58) → 32-byte public key. Verifies version + CRC16. */
  function decodeAccount(address) {
    const raw = base58Decode(address);
    if (raw.length !== 35) throw new Error('invalid address length: expected 35 bytes, got ' + raw.length);
    if (raw[0] !== 0x5b) throw new Error('invalid version byte: expected 0x5b, got 0x' + raw[0].toString(16));
    const payload = raw.subarray(0, 33);
    const expected = crc16xmodem(payload);
    const actual = raw[33] | (raw[34] << 8);
    if (expected !== actual) throw new Error('checksum mismatch');
    return raw.slice(1, 33);
  }

  // ── Hashing (Web Crypto) ──────────────────────────────────────

  async function sha512(data) {
    return new Uint8Array(await subtle.digest('SHA-512', data));
  }

  async function sha256(data) {
    return new Uint8Array(await subtle.digest('SHA-256', data));
  }

  async function hmacSHA512(key, data) {
    const kb = typeof key === 'string' ? new TextEncoder().encode(key) : key;
    const ck = await subtle.importKey('raw', kb, { name: 'HMAC', hash: 'SHA-512' }, false, ['sign']);
    return new Uint8Array(await subtle.sign('HMAC', ck, data));
  }

  // ── Ed25519 public key from 32-byte seed ──────────────────────

  async function ed25519PubFromSeed(seed) {
    const h = await sha512(seed);
    const scalar = bytesToBigIntLE(clampScalar(h));
    return ptEncode(scalarMult(scalar, BASE_PT));
  }

  // ════════════════════════════════════════════════════════════════
  //  SLIP-0010 Key Derivation
  // ════════════════════════════════════════════════════════════════

  const FIRST_HARDENED_INDEX = 0x80000000;
  const ACCOUNT_PATH         = "m/44'/2046'/0'/0'";
  const SEED_KEY             = 'ed25519 seed';
  const PATH_RE              = /^m(\/\d+')*$/;

  class Node {
    /** @private */
    constructor(key, chainCode) {
      this._key   = key;        // Uint8Array(32)
      this._cc    = chainCode;  // Uint8Array(32)
    }

    /** Derive child node at hardened index i (must be >= 0x80000000). */
    async derive(i) {
      if (i < FIRST_HARDENED_INDEX) throw new Error('ed25519 requires hardened derivation');
      const data = new Uint8Array(37);                              // 0x00 || key(32) || index(4 BE)
      data.set(this._key, 1);
      new DataView(data.buffer).setUint32(33, i, false);
      const I = await hmacSHA512(this._cc, data);
      return new Node(I.slice(0, 32), I.slice(32));
    }

    /** Ed25519 keypair: { publicKey(32), privateKey(64 = seed||pub) } */
    async keypair() {
      const pub  = await ed25519PubFromSeed(this._key);
      const priv = new Uint8Array(64);
      priv.set(this._key, 0);
      priv.set(pub, 32);
      return { publicKey: pub, privateKey: priv };
    }

    /** 0x00-prefixed public key (33 bytes), per SLIP-0010 test vectors. */
    async publicKeyWithPrefix() {
      const { publicKey } = await this.keypair();
      const r = new Uint8Array(33);
      r.set(publicKey, 1);
      return r;
    }

    /** 32-byte Ed25519 seed for this node (same as rawSeed). */
    privateKey() { return new Uint8Array(this._key); }

    /** 32-byte raw key material. Does NOT include the chain code. */
    rawSeed() { return new Uint8Array(this._key); }

    /**
     * Sign a message with this node's Ed25519 key (RFC 8032).
     * @param {string|Uint8Array} message
     * @returns {Promise<Uint8Array>} 64-byte signature
     */
    async sign(message) {
      const msg = typeof message === 'string' ? new TextEncoder().encode(message) : message;
      const h = await sha512(this._key);
      const scalar = bytesToBigIntLE(clampScalar(h));
      const prefix = h.slice(32, 64);
      const pub = await ed25519PubFromSeed(this._key);

      const rHash = await sha512(concat(prefix, msg));
      const r = mod(bytesToBigIntLE(rHash), ED_L);
      const R = ptEncode(scalarMult(r, BASE_PT));

      const hram = await sha512(concat(R, pub, msg));
      const S = mod(r + mod(bytesToBigIntLE(hram), ED_L) * scalar, ED_L);

      const sig = new Uint8Array(64);
      sig.set(R, 0);
      sig.set(bigIntToBytes32LE(S), 32);
      return sig;
    }

    /**
     * Account address: Base58( 0x5b || publicKey || crc16 ).
     * Produces an "A…"-prefixed string like the Go Encode() with AccountAddressVersionByte.
     * @returns {Promise<string>}
     */
    async account() {
      const pub = await ed25519PubFromSeed(this._key);
      const raw = new Uint8Array(1 + 32 + 2);
      raw[0] = 0x5b;                              // version byte
      raw.set(pub, 1);
      const crc = crc16xmodem(raw.subarray(0, 33));
      raw[33] = crc & 0xff;                       // checksum LE
      raw[34] = (crc >> 8) & 0xff;
      return base58Encode(raw);
    }

    /** Serialize to 64 bytes: key(32) || chainCode(32). */
    marshalBinary() {
      const b = new Uint8Array(64);
      b.set(this._key, 0);
      b.set(this._cc, 32);
      return b;
    }
  }

  /** Create master node from root seed via HMAC-SHA512("ed25519 seed", seed). */
  async function newMasterNode(seed) {
    const I = await hmacSHA512(SEED_KEY, seed);
    return new Node(I.slice(0, 32), I.slice(32));
  }

  /** Restore a node from a 64-byte blob produced by marshalBinary(). */
  function unmarshalNode(data) {
    if (data.length !== 64) throw new Error('invalid node blob length: ' + data.length);
    return new Node(new Uint8Array(data.slice(0, 32)), new Uint8Array(data.slice(32)));
  }

  /** Validate a BIP-32-style path (only hardened segments allowed for ed25519). */
  function isValidPath(path) {
    if (!PATH_RE.test(path)) return false;
    for (const seg of path.split('/').slice(1)) {
      const n = Number(seg.replace("'", ''));
      if (!Number.isInteger(n) || n < 0 || n > 0xFFFFFFFF) return false;
    }
    return true;
  }

  /** Derive a node at `path` (e.g. "m/44'/607'/0'") from a root seed. */
  async function deriveForPath(path, seed) {
    if (!isValidPath(path)) throw new Error('invalid derivation path');
    let node = await newMasterNode(seed);
    for (const seg of path.split('/').slice(1)) {
      const idx = (Number(seg.replace("'", '')) + FIRST_HARDENED_INDEX) >>> 0;
      node = await node.derive(idx);
    }
    return node;
  }

  /** Derive the account node at m/44'/2046' from a root seed. */
  async function deriveAccount(seed) {
    return deriveForPath(ACCOUNT_PATH, seed);
  }

  // ════════════════════════════════════════════════════════════════
  //  BIP-39 Mnemonics
  // ════════════════════════════════════════════════════════════════

  /** Convert mnemonic phrase → 64-byte seed via PBKDF2-SHA512 (2048 rounds). */
  async function mnemonicToSeed(mnemonic, passphrase) {
    if (passphrase === undefined) passphrase = '';
    const enc  = new TextEncoder();
    const pwd  = enc.encode(mnemonic.normalize('NFKD'));
    const salt = enc.encode(('mnemonic' + passphrase).normalize('NFKD'));
    const key  = await subtle.importKey('raw', pwd, 'PBKDF2', false, ['deriveBits']);
    const bits = await subtle.deriveBits(
      { name: 'PBKDF2', salt: salt, iterations: 2048, hash: 'SHA-512' }, key, 512
    );
    return new Uint8Array(bits);
  }

  /** Generate a random mnemonic (strength: 128/160/192/224/256 bits → 12-24 words). */
  async function generateMnemonic(strength) {
    if (strength === undefined) strength = 128;
    if (![128, 160, 192, 224, 256].includes(strength))
      throw new Error('strength must be 128 / 160 / 192 / 224 / 256');
    if (WORDLIST.length !== 2048) throw new Error('wordlist not loaded — run build.js first');

    const ent  = new Uint8Array(strength / 8);
    _crypto.getRandomValues(ent);
    const hash = await sha256(ent);
    const csBits = strength / 32;

    let bits = '';
    for (const b of ent) bits += b.toString(2).padStart(8, '0');
    for (let i = 0; i < csBits; i++)
      bits += ((hash[i >> 3] >> (7 - (i & 7))) & 1).toString();

    const words = [];
    for (let i = 0; i < bits.length; i += 11)
      words.push(WORDLIST[parseInt(bits.slice(i, i + 11), 2)]);
    return words.join(' ');
  }

  /** Validate a mnemonic (word membership + checksum). */
  async function validateMnemonic(mnemonic) {
    if (WORDLIST.length !== 2048) return false;
    const words = mnemonic.normalize('NFKD').trim().split(/\s+/);
    if (![12, 15, 18, 21, 24].includes(words.length)) return false;

    let bits = '';
    for (const w of words) {
      const idx = WORDLIST.indexOf(w);
      if (idx === -1) return false;
      bits += idx.toString(2).padStart(11, '0');
    }

    const entBits = (words.length * 11 * 32) / 33;
    const csBits  = entBits / 32;
    const ent     = new Uint8Array(entBits / 8);
    for (let i = 0; i < ent.length; i++)
      ent[i] = parseInt(bits.slice(i * 8, i * 8 + 8), 2);
    const hash = await sha256(ent);

    for (let i = 0; i < csBits; i++) {
      const want = (hash[i >> 3] >> (7 - (i & 7))) & 1;
      if (want !== Number(bits[entBits + i])) return false;
    }
    return true;
  }

  // ── Ed25519 Verify ─────────────────────────────────────────────

  /**
   * Verify an Ed25519 signature (RFC 8032).
   * @param {string|Uint8Array} message
   * @param {Uint8Array} signature  64 bytes (R || S)
   * @param {Uint8Array} publicKey  32 bytes
   * @returns {Promise<boolean>}
   */
  async function verify(message, signature, publicKey) {
    const msg = typeof message === 'string' ? new TextEncoder().encode(message) : message;
    if (signature.length !== 64) return false;
    if (publicKey.length !== 32) return false;

    const R_enc = signature.slice(0, 32);
    const S = bytesToBigIntLE(signature.slice(32, 64));
    if (S >= ED_L) return false;

    let A, R;
    try { A = ptDecode(publicKey); R = ptDecode(R_enc); }
    catch { return false; }

    const hram = await sha512(concat(R_enc, publicKey, msg));
    const k = mod(bytesToBigIntLE(hram), ED_L);

    // Check: [S]B == R + [k]A
    const lhs = scalarMult(S, BASE_PT);
    const rhs = ptAdd(R, scalarMult(k, A));
    return ptEqual(lhs, rhs);
  }

  // ── Hex helpers ───────────────────────────────────────────────

  function hexToBytes(hex) {
    const b = new Uint8Array(hex.length / 2);
    for (let i = 0; i < b.length; i++) b[i] = parseInt(hex.substr(i * 2, 2), 16);
    return b;
  }

  function bytesToHex(bytes) {
    return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  }

  // ── Public API ────────────────────────────────────────────────

  return {
    FIRST_HARDENED_INDEX: FIRST_HARDENED_INDEX,
    ACCOUNT_PATH:        ACCOUNT_PATH,
    deriveAccount:       deriveAccount,
    deriveForPath:       deriveForPath,
    newMasterNode:       newMasterNode,
    unmarshalNode:       unmarshalNode,
    isValidPath:         isValidPath,
    mnemonicToSeed:      mnemonicToSeed,
    generateMnemonic:    generateMnemonic,
    validateMnemonic:    validateMnemonic,
    verify:              verify,
    decodeAccount:       decodeAccount,
    hexToBytes:          hexToBytes,
    bytesToHex:          bytesToHex,
  };
});
