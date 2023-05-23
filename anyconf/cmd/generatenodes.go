package cmd

import (
	"fmt"
	"github.com/anyproto/any-sync-tools/anyconf/gen"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"gopkg.in/mgo.v2/bson"
	"os"
	"time"
)

const (
	addressesFlag    = "addresses"
	debugAddressFlag = "d"
	nodesYaml        = "nodes.yml"
)

var generateNodes = &cobra.Command{
	Use:   "generate-nodes",
	Short: "Generate nodes",
	Args:  cobra.RangeArgs(0, 10),
	Run: func(cmd *cobra.Command, args []string) {
		addresses, err := cmd.Flags().GetStringArray(addressesFlag)
		types, err := cmd.Flags().GetStringArray(typesFlag)
		debugAddresses, err := cmd.Flags().GetStringArray(debugAddressFlag)

		var nodesParams []gen.NodeParameters
		for i, nodeType := range types {
			nodeType := nodeconf.NodeType(nodeType)

			if !slices.Contains(validTypesOptions, nodeType) {
				fmt.Println(nodeType)
				panic("Wrong node 'type' parameter")
			}

			debugAddress := ""
			if len(debugAddresses) > i {
				debugAddress = debugAddresses[i]
			}

			grpcAddress := ""
			if len(addresses) > i {
				grpcAddress = addresses[i]
			}

			nodeParams := gen.NodeParameters{
				DebugAddress: debugAddress,
				Address:      grpcAddress,
				NodeType:     nodeType,
			}

			nodesParams = append(nodesParams, nodeParams)
		}

		nodesList, accountsList, err := gen.GenerateNodesConfigs(nodesParams)
		nodes := nodeconf.Configuration{
			Id:           bson.NewObjectId().Hex(),
			NetworkId:    "",
			Nodes:        nodesList,
			CreationTime: time.Time{},
		}

		nodesBytes, err := yaml.Marshal(nodes)
		if err != nil {
			panic(fmt.Sprintf("could Marshal nodes: %v", err))
		}

		err = os.WriteFile(nodesYaml, nodesBytes, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("could not write nodes.yml to file: %v", err))
		}

		for index, account := range accountsList {
			pc := PrivateConf{Account: account}

			accountBytes, err := yaml.Marshal(pc)

			accountFilePath := fmt.Sprintf("account%d.yml", index)

			err = os.WriteFile(accountFilePath, accountBytes, os.ModePerm)
			if err != nil {
				panic(fmt.Sprintf("could not write accountBytes to file: %v", err))
			}
		}
	},
}

func init() {
	generateNodes.Flags().StringArray(typesFlag, []string{}, "fill this flag with one of three options [tree, file, coordinator]")
	generateNodes.MarkFlagRequired(typesFlag)

	generateNodes.Flags().StringArray(debugAddressFlag, []string{}, "fill this flag with specific debug address for node")

	generateNodes.Flags().StringArray(addressesFlag, []string{}, "fill this flag with specific grpc address for node")
}
