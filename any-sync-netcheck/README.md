# Any-Sync Netcheck

A simple tool that checks the ability to connect to any-sync nodes. 
It tests network and TLS issues.

The tool opens a connection to coordinator nodes and performs libp2p and any-sync handshakes, attempting to request a network configuration.

## Installation
You can download the binary release here: https://github.com/anyproto/any-sync-tools/releases  

### Build from source   
```go install github.com/anyproto/any-sync-tools/any-sync-netcheck@latest```

## Usage
```any-sync-netcheck```

```any-sync-netcheck -v``` for a verbose output

```any-sync-netcheck -c <path_to_client.yml>``` read and check coordinators from the client.yml file

## Contribution
Thank you for your desire to develop Anytype together!

❤️ This project and everyone involved in it is governed by the [Code of Conduct](https://github.com/anyproto/.github/blob/main/docs/CODE_OF_CONDUCT.md).

🧑‍💻 Check out our [contributing guide](https://github.com/anyproto/.github/blob/main/docs/CONTRIBUTING.md) to learn about asking questions, creating issues, or submitting pull requests.

🫢 For security findings, please email [security@anytype.io](mailto:security@anytype.io) and refer to our [security guide](https://github.com/anyproto/.github/blob/main/docs/SECURITY.md) for more information.

🤝 Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT License](../LICENSE).
