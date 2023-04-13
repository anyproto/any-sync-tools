package cmd

import (
	"fmt"
	"github.com/anytypeio/any-sync-tools/anyconf/gen"
	"github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

const (
	nodesPathFlag         = "n"
	typesFlag             = "t"
	outputNodesPathFlag   = "output"
	outputAccountPathFlag = "account"
	addressFlag           = "address"
)

var validTypesOptions = []nodeconf.NodeType{nodeconf.NodeTypeTree, nodeconf.NodeTypeFile, nodeconf.NodeTypeConsensus, nodeconf.NodeTypeCoordinator}

type Nodes struct {
	Nodes []nodeconf.Node `yaml:"nodes"`
}

type PrivateConf struct {
	Account accountservice.Config `yaml:"account"`
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

		nodesConfig := nodeconf.Configuration{}

		data, err := os.ReadFile(nodesConfigPath)
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

			if !slices.Contains(validTypesOptions, nodeType) {
				panic(fmt.Errorf("wrong node 'type' parameter: '%s'", nodeType))
			}

			nodeTypes = append(nodeTypes, nodeType)
		}

		if address != "" {
			addresses = append(addresses, address)
		}

		newConf, accountConf, err := gen.GenNodeConfig(addresses, nodeTypes, nil)
		nodesConfig.Nodes = append(nodesConfig.Nodes, newConf)
		nodesConfig.Id = bson.NewObjectId().Hex()
		nodesConfig.CreationTime = time.Now()
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
		}

		fmt.Println("Node created")
		fmt.Printf("PeerId:\t%s\n", accountConf.PeerId)
		fmt.Printf("ConfigurationId:\t%s\n", nodesConfig.Id)

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
