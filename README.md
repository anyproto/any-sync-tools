# Any-Sync Tools

## `any-sync-network`
Configuration builder for Any-Sync nodes.

### Installation
1. Clone `any-sync-tools` repository.
2. Navigate to the root directory of the repository and run `go install ./any-sync-network`.

### Usage
```
any-sync-network create
```
Use the interactive CLI to describe the parameters of basic nodes and create additional nodes if needed. 

Note that there are prerequisites for successful configuration:
1. `consensus-node` requires MongoDB.
2. `file-node` requires an S3 bucket and Redis.

You can use the generated `*.yml` files as your nodes' configurations.

### Example
![Interactive CLI demo](assets/any-sync-network-example.gif)

Configuring a network with three sync nodes and one file node.


## Contribution
Thank you for your desire to develop Anytype together. 

Currently, we're not ready to accept PRs, but we will in the nearest future.

Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT License](./LICENSE).