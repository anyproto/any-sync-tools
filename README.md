# Any-Sync Tools

## `any-sync-network`

Configuration builder for Any-Sync nodes. 
See the tool [`README`](any-sync-network/README.md) for more details.

## `any-sync-netcheck`

Network issues debugger for Any-Sync networks.
See the tool [`README`](any-sync-netcheck/README.md) for more details.


## `any-sync-acl-cli`

ACL (Access Control List) inspector for Any-Sync spaces.
Connects to a coordinator node and prints the ACL log and state summary for a given space.

**Usage:**

```
any-sync-acl-cli -n <network-config.yml> -s <spaceId> [-l]
```

Flags:
- `-n` — path to network config YAML file
- `-s` — space ID to inspect
- `-l` — also print the full ACL record log (optional)

## `any-sync-signing`

CLI tool for deriving an Anytype Ed25519 identity from a BIP39 mnemonic (via SLIP-0010) and signing arbitrary data.

**Usage:**

```
# Derive identity and sign messages interactively
any-sync-signing [--index <n>] [--show-private]

# Verify a signature
any-sync-signing verify
```

The sign command prompts for a BIP39 mnemonic (hidden input), displays the derived account/peer IDs, then enters an interactive loop where you can type messages to sign. The verify command prompts for an account address, message, and base58 signature, then confirms validity.

## Contribution
Thank you for your desire to develop Anytype together!

❤️ This project and everyone involved in it is governed by the [Code of Conduct](https://github.com/anyproto/.github/blob/main/docs/CODE_OF_CONDUCT.md).

🧑‍💻 Check out our [contributing guide](https://github.com/anyproto/.github/blob/main/docs/CONTRIBUTING.md) to learn about asking questions, creating issues, or submitting pull requests.

🫢 For security findings, please email [security@anytype.io](mailto:security@anytype.io) and refer to our [security guide](https://github.com/anyproto/.github/blob/main/docs/SECURITY.md) for more information.

🤝 Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT License](./LICENSE).
