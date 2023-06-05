package cmd

import (
	"fmt"
	"github.com/anyproto/any-sync-tools/anyconf/gen"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

var createNetwork = &cobra.Command{
	Use:   "create-network",
	Short: "Creates new network keys",
	Args:  cobra.RangeArgs(0, 10),
	Run: func(cmd *cobra.Command, args []string) {
		outputAccountPath, _ := cmd.Flags().GetString(outputAccountPathFlag)
		outputNodesPath, _ := cmd.Flags().GetString(outputNodesPathFlag)
		address, _ := cmd.Flags().GetString(addressFlag)
		types, _ := cmd.Flags().GetStringArray(typesFlag)

		var nodeTypes []nodeconf.NodeType
		for _, nodeType := range types {
			nodeType := nodeconf.NodeType(nodeType)

			if !slices.Contains(validTypesOptions, nodeType) {
				panic(fmt.Errorf("wrong node 'type' parameter: '%s'", nodeType))
			}

			nodeTypes = append(nodeTypes, nodeType)
		}
		if !slices.Contains(nodeTypes, nodeconf.NodeTypeCoordinator) {
			nodeTypes = append(nodeTypes, nodeconf.NodeTypeCoordinator)
		}
		netKey, _, _ := crypto.GenerateRandomEd25519KeyPair()

		var addresses []string
		if address != "" {
			addresses = append(addresses, address)
		}
		nc, ac, err := gen.GenNodeConfig(addresses, []nodeconf.NodeType{nodeconf.NodeTypeCoordinator}, netKey)
		if err != nil {
			panic(fmt.Errorf("can't generate configs: %v", err))
		}

		nodesConfig := nodeconf.Configuration{
			Id:           bson.NewObjectId().Hex(),
			NetworkId:    netKey.GetPublic().Network(),
			Nodes:        []nodeconf.Node{nc},
			CreationTime: time.Now(),
		}

		fmt.Println("Network created")
		fmt.Printf("NetworkId:\t%s\n", netKey.GetPublic().Network())
		fmt.Printf("ConfigurationId:\t%s\n", nodesConfig.Id)

		bytes, err := yaml.Marshal(nodesConfig)
		if err != nil {
			panic(fmt.Sprintf("could not marshal the keys: %v", err))
		}

		err = os.WriteFile(outputNodesPath, bytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write the config to file: %v", err))
		}

		privateConf := PrivateConf{
			Account: ac,
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
	createNetwork.Flags().StringArray(typesFlag, []string{}, "fill this flag with one of three options [tree, file, coordinator]")
	createNetwork.Flags().String(outputNodesPathFlag, "nodes.yml", "Path to output nodes yaml with a new node")
	createNetwork.Flags().String(addressFlag, "", "Address to node [optional]")
	createNetwork.Flags().String(outputAccountPathFlag, "account.yml", "Path to output account + nodes yaml")
}
