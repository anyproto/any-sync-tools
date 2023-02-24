package cmd

import (
	"fmt"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/keys"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

type oldNodes struct {
	Nodes []oldNodeConfig `yaml:"nodes"`
}

type oldNodeConfig struct {
	PeerId        string              `yaml:"peerId"`
	Addresses     []string            `yaml:"address"`
	SigningKey    string              `yaml:"signingKey,omitempty"`
	EncryptionKey string              `yaml:"encryptionKey,omitempty"`
	Types         []nodeconf.NodeType `yaml:"types,omitempty"`
}

var reconfigure = &cobra.Command{
	Use:   "reconfigure",
	Short: "Reconfigure old config",
	Args:  cobra.RangeArgs(0, 10),
	Run: func(cmd *cobra.Command, args []string) {
		nodesConfigPath, err := cmd.Flags().GetString(nodesPathFlag)
		outputNodesPath, err := cmd.Flags().GetString(outputNodesPathFlag)

		if err != nil {
			panic(err)
		}

		oldNodesConfig := oldNodes{}

		data, err := ioutil.ReadFile(nodesConfigPath)
		if err != nil {
			panic("Couldn't read file")
		}

		err = yaml.Unmarshal(data, &oldNodesConfig)
		if err != nil {
			panic("The file structure is wrong")
		}

		var mappedNodes []nodeconf.NodeConfig

		for _, oldNode := range oldNodesConfig.Nodes {

			decodedEncryptionKey, err := keys.DecodeKeyFromString(
				oldNode.EncryptionKey,
				encryptionkey.NewEncryptionRsaPrivKeyFromBytes,
				nil)

			if err != nil {
				newNodeConf := nodeconf.NodeConfig{
					PeerId:    oldNode.PeerId,
					Addresses: oldNode.Addresses,
					Types:     oldNode.Types,
				}

				mappedNodes = append(mappedNodes, newNodeConf)

				continue
			}

			publicKey := decodedEncryptionKey.GetPublic()

			publicKeyString, err := keys.EncodeKeyToString(publicKey)
			if err != nil {
				panic(err)
			}

			newNodeConf := nodeconf.NodeConfig{
				PeerId:        oldNode.PeerId,
				Addresses:     oldNode.Addresses,
				EncryptionKey: publicKeyString,
				Types:         oldNode.Types,
			}

			mappedNodes = append(mappedNodes, newNodeConf)
		}

		nodesConfig := Nodes{mappedNodes}

		bytes, err := yaml.Marshal(nodesConfig)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(outputNodesPath, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}
	},
}

func init() {
	reconfigure.Flags().String(nodesPathFlag, "nodes.yml", "Path to existing nodes yaml")
	reconfigure.MarkFlagRequired(nodesPathFlag)

	reconfigure.Flags().String(outputNodesPathFlag, "", "Path to output nodes yaml with a new node")
	reconfigure.MarkFlagRequired(outputNodesPathFlag)
}
