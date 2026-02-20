#!/usr/bin/env node
const slip10 = require('./slip10.compiled.js');

const vectors = [
  // Vector 1: seed 000102030405060708090a0b0c0d0e0f
  {
    seed: '000102030405060708090a0b0c0d0e0f',
    tests: [
      { path: 'm',                              priv: '2b4be7f19ee27bbf30c667b642d5f4aa69fd169872f8fc3059c08ebae2eb19e7', pub: '00a4b2856bfec510abab89753fac1ac0e1112364e7d250545963f135f2a33188ed' },
      { path: "m/0'",                            priv: '68e0fe46dfb67e368c75379acec591dad19df3cde26e63b93a8e704f1dade7a3', pub: '008c8a13df77a28f3445213a0f432fde644acaa215fc72dcdf300d5efaa85d350c' },
      { path: "m/0'/1'",                         priv: 'b1d0bad404bf35da785a64ca1ac54b2617211d2777696fbffaf208f746ae84f2', pub: '001932a5270f335bed617d5b935c80aedb1a35bd9fc1e31acafd5372c30f5c1187' },
      { path: "m/0'/1'/2'",                      priv: '92a5b23c0b8a99e37d07df3fb9966917f5d06e02ddbd909c7e184371463e9fc9', pub: '00ae98736566d30ed0e9d2f4486a64bc95740d89c7db33f52121f8ea8f76ff0fc1' },
      { path: "m/0'/1'/2'/2'",                   priv: '30d1dc7e5fc04c31219ab25a27ae00b50f6fd66622f6e9c913253d6511d1e662', pub: '008abae2d66361c879b900d204ad2cc4984fa2aa344dd7ddc46007329ac76c429c' },
      { path: "m/0'/1'/2'/2'/1000000000'",       priv: '8f94d394a8e8fd6b1bc2f3f49f5c47e385281d5c17e65324b0f62483e37e8793', pub: '003c24da049451555d51a7014a37337aa4e12d41e485abccfa46b47dfb2af54b7a' },
    ],
  },
  // Vector 2: long seed
  {
    seed: 'fffcf9f6f3f0edeae7e4e1dedbd8d5d2cfccc9c6c3c0bdbab7b4b1aeaba8a5a29f9c999693908d8a8784817e7b7875726f6c696663605d5a5754514e4b484542',
    tests: [
      { path: 'm',                                             priv: '171cb88b1b3c1db25add599712e36245d75bc65a1a5c9e18d76f9f2b1eab4012', pub: '008fe9693f8fa62a4305a140b9764c5ee01e455963744fe18204b4fb948249308a' },
      { path: "m/0'",                                          priv: '1559eb2bbec5790b0c65d8693e4d0875b1747f4970ae8b650486ed7470845635', pub: '0086fab68dcb57aa196c77c5f264f215a112c22a912c10d123b0d03c3c28ef1037' },
      { path: "m/0'/2147483647'",                              priv: 'ea4f5bfe8694d8bb74b7b59404632fd5968b774ed545e810de9c32a4fb4192f4', pub: '005ba3b9ac6e90e83effcd25ac4e58a1365a9e35a3d3ae5eb07b9e4d90bcf7506d' },
      { path: "m/0'/2147483647'/1'",                           priv: '3757c7577170179c7868353ada796c839135b3d30554bbb74a4b1e4a5a58505c', pub: '002e66aa57069c86cc18249aecf5cb5a9cebbfd6fadeab056254763874a9352b45' },
      { path: "m/0'/2147483647'/1'/2147483646'",               priv: '5837736c89570de861ebc173b1086da4f505d4adb387c6a1b1342d5e4ac9ec72', pub: '00e33c0f7d81d843c572275f287498e8d408654fdf0d1e065b84e2e6f157aab09b' },
      { path: "m/0'/2147483647'/1'/2147483646'/2'",            priv: '551d333177df541ad876a60ea71f00447931c0a9da16f227c11ea080d7391b8d', pub: '0047150c75db263559a70d5778bf36abbab30fb061ad69f69ece61a72b0cfa4fc0' },
    ],
  },
];

(async () => {
  let pass = 0, fail = 0;

  for (const v of vectors) {
    const seed = slip10.hexToBytes(v.seed);
    for (const t of v.tests) {
      const node = await slip10.deriveForPath(t.path, seed);
      const priv = slip10.bytesToHex(node.privateKey());
      const pub  = slip10.bytesToHex(await node.publicKeyWithPrefix());

      if (priv !== t.priv || pub !== t.pub) {
        console.log(`FAIL ${t.path}`);
        if (priv !== t.priv) console.log(`  priv: got ${priv}\n  want ${t.priv}`);
        if (pub  !== t.pub)  console.log(`  pub:  got ${pub}\n  want ${t.pub}`);
        fail++;
      } else {
        console.log(`OK   ${t.path}`);
        pass++;
      }
    }
  }

  // Test invalid path
  try { await slip10.deriveForPath('m/0', slip10.hexToBytes('00')); fail++; console.log('FAIL m/0 should reject'); }
  catch { pass++; console.log('OK   m/0 rejected (non-hardened)'); }

  // Test marshal/unmarshal roundtrip
  const seed = slip10.hexToBytes('000102030405060708090a0b0c0d0e0f');
  const orig = await slip10.deriveForPath("m/0'/1'", seed);
  const blob = orig.marshalBinary();
  const restored = slip10.unmarshalNode(blob);
  const origPriv = slip10.bytesToHex(orig.privateKey());
  const restPriv = slip10.bytesToHex(restored.privateKey());
  if (origPriv === restPriv) { pass++; console.log('OK   marshal/unmarshal roundtrip'); }
  else { fail++; console.log('FAIL marshal/unmarshal roundtrip'); }

  // Test mnemonic validation
  const mnemonic = await slip10.generateMnemonic(128);
  if (await slip10.validateMnemonic(mnemonic)) { pass++; console.log('OK   generateMnemonic + validate'); }
  else { fail++; console.log('FAIL generateMnemonic + validate'); }

  // Test mnemonicToSeed produces 64 bytes
  const mnSeed = await slip10.mnemonicToSeed(mnemonic);
  if (mnSeed.length === 64) { pass++; console.log('OK   mnemonicToSeed → 64 bytes'); }
  else { fail++; console.log('FAIL mnemonicToSeed length: ' + mnSeed.length); }

  // Test sign + verify
  {
    const node = await slip10.deriveForPath("m/0'/1'", slip10.hexToBytes('000102030405060708090a0b0c0d0e0f'));
    const { publicKey } = await node.keypair();
    const msg = 'hello world';
    const sig = await node.sign(msg);

    if (sig.length === 64) { pass++; console.log('OK   sign → 64-byte signature'); }
    else { fail++; console.log('FAIL sign length: ' + sig.length); }

    if (await slip10.verify(msg, sig, publicKey)) { pass++; console.log('OK   verify(correct message) → true'); }
    else { fail++; console.log('FAIL verify should be true'); }

    if (!(await slip10.verify('wrong', sig, publicKey))) { pass++; console.log('OK   verify(wrong message) → false'); }
    else { fail++; console.log('FAIL verify(wrong) should be false'); }

    // tampered signature
    const bad = new Uint8Array(sig);
    bad[0] ^= 0xff;
    if (!(await slip10.verify(msg, bad, publicKey))) { pass++; console.log('OK   verify(tampered sig) → false'); }
    else { fail++; console.log('FAIL verify(tampered sig) should be false'); }

    // binary message
    const binMsg = new Uint8Array([1, 2, 3, 4, 5]);
    const binSig = await node.sign(binMsg);
    if (await slip10.verify(binMsg, binSig, publicKey)) { pass++; console.log('OK   sign/verify binary message'); }
    else { fail++; console.log('FAIL sign/verify binary message'); }
  }

  // Test account() address via deriveAccount (m/44'/2046')
  {
    const seed = slip10.hexToBytes('000102030405060708090a0b0c0d0e0f');
    const node = await slip10.deriveAccount(seed);
    const addr = await node.account();
    if (typeof addr === 'string' && addr.startsWith('A')) { pass++; console.log('OK   account() → "A…" address: ' + addr); }
    else { fail++; console.log('FAIL account() expected A-prefix, got: ' + addr); }

    // Deterministic: same seed → same address
    const node2 = await slip10.deriveForPath(slip10.ACCOUNT_PATH, seed);
    const addr2 = await node2.account();
    if (addr === addr2) { pass++; console.log('OK   deriveAccount == deriveForPath(ACCOUNT_PATH)'); }
    else { fail++; console.log('FAIL deriveAccount mismatch'); }

    // decodeAccount roundtrip
    const kp = await node.keypair();
    const decoded = slip10.decodeAccount(addr);
    if (slip10.bytesToHex(decoded) === slip10.bytesToHex(kp.publicKey)) { pass++; console.log('OK   decodeAccount roundtrip'); }
    else { fail++; console.log('FAIL decodeAccount roundtrip'); }
  }

  console.log(`\n${pass} passed, ${fail} failed`);
  process.exit(fail > 0 ? 1 : 0);
})();
