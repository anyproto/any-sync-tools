package cmd

import (
	"fmt"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/keys"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/any-sync/util/peer"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

const (
	nodesPathFlag         = "n"
	typesFlag             = "t"
	outputNodesPathFlag   = "output"
	outputAccountPathFlag = "account"
	addressFlag           = "address"
)

var validOptions = []nodeconf.NodeType{nodeconf.NodeTypeTree, nodeconf.NodeTypeFile, nodeconf.NodeTypeConsensus, nodeconf.NodeTypeCoordinator}

type Nodes struct {
	Nodes []nodeconf.NodeConfig `yaml:"nodes"`
}

type PrivateConf struct {
	Account accountservice.Config `yaml:"account"`
	Nodes   []nodeconf.NodeConfig `yaml:"nodes"`
}

var addNode = &cobra.Command{
	Use:   "add-node",
	Short: "Add note to existing node list",
	Args:  cobra.RangeArgs(0, 10),
	Run: func(cmd *cobra.Command, args []string) {
		nodesConfigPath, err := cmd.Flags().GetString(nodesPathFlag)
		types, err := cmd.Flags().GetStringArray(typesFlag)
		outputNodesPath, err := cmd.Flags().GetString(outputNodesPathFlag)
		outputAccountPath, err := cmd.Flags().GetString(outputAccountPathFlag)
		address, err := cmd.Flags().GetString(addressFlag)

		if err != nil {
			panic(err)
		}

		if len(types) == 0 {
			panic("You should specify at least one node type")
		}

		nodesConfig := Nodes{}

		data, err := ioutil.ReadFile(nodesConfigPath)
		if err != nil {
			panic("Couldn't read file")
		}

		err = yaml.Unmarshal(data, &nodesConfig)
		if err != nil {
			panic("The file structure is wrong")
		}

		var addresses []string

		var nodeTypes []nodeconf.NodeType
		for _, nodeType := range types {
			nodeType := nodeconf.NodeType(nodeType)

			if !slices.Contains(validOptions, nodeType) {
				panic("Wrong node 'type' parameter")
			}

			nodeTypes = append(nodeTypes, nodeType)
		}

		if address != "" {
			addresses = append(addresses, address)
		}

		newConf, accountConf, err := genNodeConfig(addresses, nodeTypes)
		nodesConfig.Nodes = append(nodesConfig.Nodes, newConf)

		bytes, err := yaml.Marshal(nodesConfig)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(outputNodesPath, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}

		privateConf := PrivateConf{
			Account: accountConf,
			Nodes:   nodesConfig.Nodes,
		}

		bytes, err = yaml.Marshal(privateConf)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(outputAccountPath, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}
	},
}

func init() {
	addNode.Flags().String(nodesPathFlag, "old_nodes.yml", "Path to existing nodes yaml")

	addNode.Flags().StringArray(typesFlag, []string{}, "fill this flag with one of three options [tree, file, coordinator]")
	addNode.MarkFlagRequired(typesFlag)

	addNode.Flags().String(outputNodesPathFlag, "nodes.yml", "Path to output nodes yaml with a new node")

	addNode.Flags().String(addressFlag, "", "Address to node [optional]")

	addNode.Flags().String(outputAccountPathFlag, "account.yml", "Path to output account + nodes yaml")
}

func genNodeConfig(addresses []string, types []nodeconf.NodeType) (nodeconf.NodeConfig, accountservice.Config, error) {
	encKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		return nodeconf.NodeConfig{}, accountservice.Config{}, err
	}

	signKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return nodeconf.NodeConfig{}, accountservice.Config{}, err
	}

	encPubKey := encKey.GetPublic()
	encPubKeyString, err := keys.EncodeKeyToString(encPubKey)

	encEncKey, err := keys.EncodeKeyToString(encKey) // private key
	if err != nil {
		return nodeconf.NodeConfig{}, accountservice.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signKey) //encSignKey
	if err != nil {
		return nodeconf.NodeConfig{}, accountservice.Config{}, err
	}

	peerID, err := peer.IdFromSigningPubKey(signKey.GetPublic())

	if err != nil {
		return nodeconf.NodeConfig{}, accountservice.Config{}, err
	}

	nodeconfig := nodeconf.NodeConfig{
		PeerId:        peerID.String(),
		Addresses:     addresses,
		EncryptionKey: encPubKeyString,
		Types:         types,
	}

	accountConfig := accountservice.Config{
		PeerId:        peerID.String(),
		PeerKey:       encSignKey,
		SigningKey:    encSignKey,
		EncryptionKey: encEncKey,
	}

	return nodeconfig, accountConfig, nil
}
