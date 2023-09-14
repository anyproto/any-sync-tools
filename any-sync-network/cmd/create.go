package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/util/crypto"
	"github.com/spf13/cobra"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/yaml.v3"
)

type GeneralNodeConfig struct {
	Account accountservice.Config `yaml:"account"`
	Drpc    struct {
		Stream struct {
			MaxMsgSizeMb int `yaml:"maxMsgSizeMb"`
		} `yaml:"stream"`
	} `yaml:"drpc"`
	Yamux struct {
		ListenAddrs     []string `yaml:"listenAddrs"`
		WriteTimeoutSec int      `yaml:"writeTimeoutSec"`
		DialTimeoutSec  int      `yaml:"dialTimeoutSec"`
	} `yaml:"yamux"`
	Quic struct {
		ListenAddrs     []string `yaml:"listenAddrs"`
		WriteTimeoutSec int      `yaml:"writeTimeoutSec"`
		DialTimeoutSec  int      `yaml:"dialTimeoutSec"`
	} `yaml:"quic"`
	Network          Network `yaml:"network"`
	NetworkStorePath string  `yaml:"networkStorePath"`
}

type CoordinatorNodeConfig struct {
	GeneralNodeConfig `yaml:".,inline"`
	Mongo             struct {
		Connect  string `yaml:"connect"`
		Database string `yaml:"database"`
	} `yaml:"mongo"`
	SpaceStatus struct {
		RunSeconds         int `yaml:"runSeconds"`
		DeletionPeriodDays int `yaml:"deletionPeriodDays"`
	} `yaml:"spaceStatus"`
}

type ConsensusNodeConfig struct {
	GeneralNodeConfig `yaml:".,inline"`
	Mongo             struct {
		Connect  string `yaml:"connect"`
		Database string `yaml:"database"`
		LogCollection string `yaml:"logCollection"`
	} `yaml:"mongo"`
	NetworkUpdateIntervalSec int `yaml:"networkUpdateIntervalSec"`
}

type SyncNodeConfig struct {
	GeneralNodeConfig        `yaml:".,inline"`
	NetworkUpdateIntervalSec int `yaml:"networkUpdateIntervalSec"`
	Space                    struct {
		GcTTL      int `yaml:"gcTTL"`
		SyncPeriod int `yaml:"syncPeriod"`
	} `yaml:"space"`
	Storage struct {
		Path string `yaml:"path"`
	} `yaml:"storage"`
	NodeSync struct {
		HotSync struct {
			SimultaneousRequests int `yaml:"simultaneousRequests"`
		} `yaml:"hotSync"`
		SyncOnStart       bool `yaml:"syncOnStart"`
		PeriodicSyncHours int  `yaml:"periodicSyncHours"`
	} `yaml:"nodeSync"`
	Log struct {
		Production   bool   `yaml:"production"`
		DefaultLevel string `yaml:"defaultLevel"`
		NamedLevels  struct {
		} `yaml:"namedLevels"`
	} `yaml:"log"`
}

type FileNodeConfig struct {
	GeneralNodeConfig        `yaml:".,inline"`
	NetworkUpdateIntervalSec int `yaml:"networkUpdateIntervalSec"`
	S3Store                  struct {
		Endpoint   string `yaml:"endpoint,omitempty"`
		Region     string `yaml:"region"`
		Profile    string `yaml:"profile"`
		Bucket     string `yaml:"bucket"`
		MaxThreads int    `yaml:"maxThreads"`
	} `yaml:"s3Store"`
	Redis struct {
		IsCluster bool   `yaml:"isCluster"`
		URL       string `yaml:"url"`
	} `yaml:"redis"`
}

type Node struct {
	PeerID    string   `yaml:"peerId"`
	Addresses []string `yaml:"addresses"`
	Types     []string `yaml:"types"`
}

type HeartConfig struct {
	NetworkID string `yaml:"networkId"`
	Nodes     []Node `yaml:"nodes"`
}

type Network struct {
	ID           string `yaml:"id"`
	HeartConfig  `yaml:".,inline"`
	CreationTime time.Time `yaml:"creationTime"`
}

var create = &cobra.Command{
	Use:   "create",
	Short: "Creates new network configuration",
	Run: func(cmd *cobra.Command, args []string) {
		// Create Network
		fmt.Println("Creating network...")
		netKey, _, _ := crypto.GenerateRandomEd25519KeyPair()
		network = Network{
			HeartConfig: HeartConfig{
				Nodes: []Node{},
			},
		}
		network.ID = bson.NewObjectId().Hex()
		network.NetworkID = netKey.GetPublic().Network()
		network.CreationTime = time.Now()

		fmt.Println("\033[1m  Network ID:\033[0m", network.NetworkID)

		// Create coordinator node
		fmt.Println("\nCreating coordinator node...")

		var coordinatorQs = []*survey.Question{
			{
				Name: "address",
				Prompt: &survey.Input{
					Message: "Any-Sync Coordinator Node address (without port)",
					Default: "127.0.0.1",
				},
				Validate: survey.Required,
			},
			{
				Name: "yamuxPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Coordinator Node Yamux (TCP) port",
					Default: "4830",
				},
				Validate: survey.Required,
			},
			{
				Name: "quicPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Coordinator Node Quic (UDP) port",
					Default: "5830",
				},
				Validate: survey.Required,
			},
			{
				Name: "mongoConnect",
				Prompt: &survey.Input{
					Message: "Mongo connect URI",
					Default: "mongodb://localhost:27017",
				},
				Validate: survey.Required,
			},
			{
				Name: "mongoDB",
				Prompt: &survey.Input{
					Message: "Mongo database name",
					Default: "coordinator",
				},
				Validate: survey.Required,
			},
		}

		coordinatorAs := struct {
			Address      string
			YamuxPort    string
			QuicPort     string
			MongoConnect string
			MongoDB      string
		}{}

		err := survey.Ask(coordinatorQs, &coordinatorAs)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		coordinatorNode := defaultCoordinatorNode()
		coordinatorNode.Yamux.ListenAddrs = append(coordinatorNode.Yamux.ListenAddrs, coordinatorAs.Address + ":" + coordinatorAs.YamuxPort)
		coordinatorNode.Quic.ListenAddrs = append(coordinatorNode.Quic.ListenAddrs, coordinatorAs.Address + ":" + coordinatorAs.QuicPort)
		coordinatorNode.Mongo.Connect = coordinatorAs.MongoConnect
		coordinatorNode.Mongo.Database = coordinatorAs.MongoDB
		coordinatorNode.Account = generateAccount()
		coordinatorNode.Account.SigningKey, _ = crypto.EncodeKeyToString(netKey)

		addToNetwork(coordinatorNode.GeneralNodeConfig, "coordinator")

		// Create consensus node
		fmt.Println("\nCreating consensus node...")

		var consensusQs = []*survey.Question{
			{
				Name: "address",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Node address (without port)",
					Default: "127.0.0.1",
				},
				Validate: survey.Required,
			},
			{
				Name: "yamuxPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Node Yamux (TCP) port",
					Default: "4530",
				},
				Validate: survey.Required,
			},
			{
				Name: "quicPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Node Quic (UDP) port",
					Default: "5530",
				},
				Validate: survey.Required,
			},
			{
				Name: "mongoDB",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Mongo database name",
					Default: "consensus",
				},
				Validate: survey.Required,
			},
		}

		consensusAs := struct {
			Address      string
			YamuxPort    string
			QuicPort     string
			MongoDB      string
		}{}

		err = survey.Ask(consensusQs, &consensusAs)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		consensusNode := defaultConsensusNode()
		consensusNode.Yamux.ListenAddrs = append(consensusNode.Yamux.ListenAddrs, consensusAs.Address + ":" + consensusAs.YamuxPort)
		consensusNode.Quic.ListenAddrs = append(consensusNode.Quic.ListenAddrs, consensusAs.Address + ":" + consensusAs.QuicPort)
		consensusNode.Mongo.Database = consensusAs.MongoDB
		consensusNode.Account = generateAccount()

		addToNetwork(consensusNode.GeneralNodeConfig, "consensus")

		createSyncNode()

		createFileNode()

		lastStepOptions()

		// Create configurations for all nodes
		fmt.Println("\nCreating config file...")

		coordinatorNode.Network = network
		createConfigFile(coordinatorNode, "coordinator")

		consensusNode.Network = network
		createConfigFile(consensusNode, "consensus")

		for i, syncNode := range syncNodes {
			syncNode.Network = network
			createConfigFile(syncNode, "sync_"+strconv.Itoa(i+1))
		}

		for i, fileNode := range fileNodes {
			fileNode.Network = network
			createConfigFile(fileNode, "file_"+strconv.Itoa(i+1))
		}

		createConfigFile(network.HeartConfig, "heart")

		fmt.Println("Done!")
	},
}

var network = Network{}

func addToNetwork(node GeneralNodeConfig, nodeType string) {
	addresses := []string{}
	for _, addr := range node.Yamux.ListenAddrs {
		addresses = append(addresses, "yamux://"+addr)
	}
	for _, addr := range node.Quic.ListenAddrs {
		addresses = append(addresses, "quic://"+addr)
	}
	network.Nodes = append(network.Nodes, Node{
		PeerID:    node.Account.PeerId,
		Addresses: addresses,
		Types:     []string{nodeType},
	})
}

var syncNodeYamuxPort = "4430"
var syncNodeQuicPort = "5430"
var syncNodes = []SyncNodeConfig{}

func createSyncNode() {
	fmt.Println("\nCreating sync node...")

	var syncQs = []*survey.Question{
		{
			Name: "address",
			Prompt: &survey.Input{
				Message: "Any-Sync Node address (without port)",
				Default: "127.0.0.1",
			},
			Validate: survey.Required,
		},
		{
			Name: "yamuxPort",
			Prompt: &survey.Input{
				Message: "Any-Sync Node Yamux (TCP) port",
				Default:  syncNodeYamuxPort,
			},
			Validate: survey.Required,
		},
		{
			Name: "quicPort",
			Prompt: &survey.Input{
				Message: "Any-Sync Node Quic (UDP) port",
				Default:  syncNodeQuicPort,
			},
			Validate: survey.Required,
		},
	}

	answers := struct {
		Address string
		YamuxPort string
		QuicPort string
	}{}

	err := survey.Ask(syncQs, &answers)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	syncNode := defaultSyncNode()
	syncNode.Yamux.ListenAddrs = append(syncNode.Yamux.ListenAddrs, answers.Address + ":" + answers.YamuxPort)
	syncNode.Quic.ListenAddrs = append(syncNode.Quic.ListenAddrs, answers.Address + ":" + answers.QuicPort)
	syncNode.Account = generateAccount()

	addToNetwork(syncNode.GeneralNodeConfig, "tree")
	syncNodes = append(syncNodes, syncNode)

	// Increase sync node port
	port_num, _ := strconv.ParseInt(syncNodeYamuxPort, 10, 0)
	port_num += 1
	syncNodeYamuxPort = strconv.FormatInt(port_num, 10)
	port_num, _ = strconv.ParseInt(syncNodeQuicPort, 10, 0)
	port_num += 1
	syncNodeQuicPort = strconv.FormatInt(port_num, 10)
}

var fileNodeYamuxPort = "4730"
var fileNodeQuicPort = "5730"
var fileNodes = []FileNodeConfig{}

func createFileNode() {
	fmt.Println("\nCreating file node...")

	var fileQs = []*survey.Question{
		{
			Name: "address",
			Prompt: &survey.Input{
				Message: "Any-Sync File Node address (without port)",
				Default: "127.0.0.1",
			},
			Validate: survey.Required,
		},
		{
			Name: "yamuxPort",
			Prompt: &survey.Input{
				Message: "Any-Sync File Node Yamux (TCP) port",
				Default:  fileNodeYamuxPort,
			},
			Validate: survey.Required,
		},
		{
			Name: "quicPort",
			Prompt: &survey.Input{
				Message: "Any-Sync File Node Quic (UDP) port",
				Default:  fileNodeQuicPort,
			},
			Validate: survey.Required,
		},
		{
			Name: "s3Endpoint",
			Prompt: &survey.Input{
				Message: "S3 Endpoint",
				Help: "Required only in the case you self-host S3-compatible object storage",
			},
		},
		{
			Name: "s3Region",
			Prompt: &survey.Input{
				Message: "S3 Region",
				Default: "eu-central-1",
			},
			Validate: survey.Required,
		},
		{
			Name: "s3Profile",
			Prompt: &survey.Input{
				Message: "S3 Profile",
				Default: "default",
			},
			Validate: survey.Required,
		},
		{
			Name: "s3Bucket",
			Prompt: &survey.Input{
				Message: "S3 Bucket",
				Default: "any-sync-files",
			},
			Validate: survey.Required,
		},
		{
			Name: "redisURL",
			Prompt: &survey.Input{
				Message: "Redis URL",
				Default: "redis://127.0.0.1:6379/?dial_timeout=3&db=1&read_timeout=6s&max_retries=2",
			},
			Validate: survey.Required,
		},
		{
			Name: "redisCluster",
			Prompt: &survey.Select{
				Message: "Is your redis installation a cluster?",
				Options: []string{"true", "false"},
				Default: "false",
			},
			Validate: survey.Required,
		},
	}

	answers := struct {
		Address      string
		YamuxPort    string
		QuicPort     string
		S3Endpoint   string
		S3Region     string
		S3Profile    string
		S3Bucket     string
		RedisURL     string
		RedisCluster string
	}{}

	err := survey.Ask(fileQs, &answers)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fileNode := defaultFileNode()
	fileNode.Yamux.ListenAddrs = append(fileNode.Yamux.ListenAddrs, answers.Address + ":" + answers.YamuxPort)
	fileNode.Quic.ListenAddrs = append(fileNode.Quic.ListenAddrs, answers.Address + ":" + answers.QuicPort)
	fileNode.S3Store.Endpoint = answers.S3Endpoint
	fileNode.S3Store.Region = answers.S3Region
	fileNode.S3Store.Profile = answers.S3Profile
	fileNode.S3Store.Bucket = answers.S3Bucket
	fileNode.Redis.URL = answers.RedisURL
	fileNode.Redis.IsCluster, _ = strconv.ParseBool(answers.RedisCluster)
	fileNode.Account = generateAccount()

	addToNetwork(fileNode.GeneralNodeConfig, "file")
	fileNodes = append(fileNodes, fileNode)

	// Increase file node port
	port_num, _ := strconv.ParseInt(fileNodeYamuxPort, 10, 0)
	port_num += 1
	fileNodeYamuxPort = strconv.FormatInt(port_num, 10)
	port_num, _ = strconv.ParseInt(fileNodeQuicPort, 10, 0)
	port_num += 1
	fileNodeQuicPort = strconv.FormatInt(port_num, 10)
}

func lastStepOptions() {
	fmt.Println()
	prompt := &survey.Select{
		Message: "Do you want to add more nodes?",
		Options: []string{"No, generate configs", "Add sync-node", "Add file-node"},
		Default: "No, generate configs",
	}

	option := ""
	survey.AskOne(prompt, &option, survey.WithValidator(survey.Required))
	switch option {
	case "Add sync-node":
		createSyncNode()
		lastStepOptions()
	case "Add file-node":
		createFileNode()
		lastStepOptions()
	default:
		return
	}
}

func generateAccount() accountservice.Config {
	signKey, _, _ := crypto.GenerateRandomEd25519KeyPair()

	encPeerSignKey, err := crypto.EncodeKeyToString(signKey)
	if err != nil {
		return accountservice.Config{}
	}

	peerID := signKey.GetPublic().PeerId()

	return accountservice.Config{
		PeerId:     peerID,
		PeerKey:    encPeerSignKey,
		SigningKey: encPeerSignKey,
	}
}

func defaultGeneralNode() GeneralNodeConfig {
	return GeneralNodeConfig{
		Drpc: struct {
			Stream struct {
				MaxMsgSizeMb int "yaml:\"maxMsgSizeMb\""
			} "yaml:\"stream\""
		}{
			Stream: struct {
				MaxMsgSizeMb int "yaml:\"maxMsgSizeMb\""
			}{
				MaxMsgSizeMb: 256,
			},
		},
		Yamux: struct {
			ListenAddrs     []string "yaml:\"listenAddrs\""
			WriteTimeoutSec int      "yaml:\"writeTimeoutSec\""
			DialTimeoutSec  int      "yaml:\"dialTimeoutSec\""
		}{
			WriteTimeoutSec: 10,
			DialTimeoutSec:  10,
		},
		Quic: struct {
			ListenAddrs     []string "yaml:\"listenAddrs\""
			WriteTimeoutSec int      "yaml:\"writeTimeoutSec\""
			DialTimeoutSec  int      "yaml:\"dialTimeoutSec\""
		}{
			WriteTimeoutSec: 10,
			DialTimeoutSec:  10,
		},
		NetworkStorePath: ".",
	}
}

func defaultCoordinatorNode() CoordinatorNodeConfig {
	return CoordinatorNodeConfig{
		GeneralNodeConfig: defaultGeneralNode(),
		Mongo: struct {
			Connect  string "yaml:\"connect\""
			Database string "yaml:\"database\""
		}{},
		SpaceStatus: struct {
			RunSeconds         int "yaml:\"runSeconds\""
			DeletionPeriodDays int "yaml:\"deletionPeriodDays\""
		}{
			RunSeconds:         20,
			DeletionPeriodDays: 1,
		},
	}
}

func defaultConsensusNode() ConsensusNodeConfig {
	return ConsensusNodeConfig{
		GeneralNodeConfig: defaultGeneralNode(),
		Mongo: struct {
			Connect  string "yaml:\"connect\""
			Database string "yaml:\"database\""
			LogCollection string "yaml:\"logCollection\""
		}{
			LogCollection: "log",
		},
		NetworkUpdateIntervalSec: 600,
	}
}

func defaultSyncNode() SyncNodeConfig {
	return SyncNodeConfig{
		GeneralNodeConfig:        defaultGeneralNode(),
		NetworkUpdateIntervalSec: 600,
		Space: struct {
			GcTTL      int "yaml:\"gcTTL\""
			SyncPeriod int "yaml:\"syncPeriod\""
		}{
			GcTTL:      60,
			SyncPeriod: 240,
		},
		Storage: struct {
			Path string "yaml:\"path\""
		}{
			Path: "db",
		},
		NodeSync: struct {
			HotSync struct {
				SimultaneousRequests int "yaml:\"simultaneousRequests\""
			} "yaml:\"hotSync\""
			SyncOnStart       bool "yaml:\"syncOnStart\""
			PeriodicSyncHours int  "yaml:\"periodicSyncHours\""
		}{
			HotSync: struct {
				SimultaneousRequests int "yaml:\"simultaneousRequests\""
			}{
				SimultaneousRequests: 400,
			},
			SyncOnStart:       true,
			PeriodicSyncHours: 2,
		},
		Log: struct {
			Production   bool     "yaml:\"production\""
			DefaultLevel string   "yaml:\"defaultLevel\""
			NamedLevels  struct{} "yaml:\"namedLevels\""
		}{
			Production:   false,
			DefaultLevel: "",
			NamedLevels:  struct{}{},
		},
	}
}

func defaultFileNode() FileNodeConfig {
	return FileNodeConfig{
		GeneralNodeConfig:        defaultGeneralNode(),
		NetworkUpdateIntervalSec: 600,
		S3Store: struct {
			Endpoint   string "yaml:\"endpoint,omitempty\""
			Region     string "yaml:\"region\""
			Profile    string "yaml:\"profile\""
			Bucket     string "yaml:\"bucket\""
			MaxThreads int    "yaml:\"maxThreads\""
		}{
			MaxThreads: 16,
		},
		Redis: struct {
			IsCluster bool   "yaml:\"isCluster\""
			URL       string "yaml:\"url\""
		}{},
	}
}

func createConfigFile(in interface{}, ymlFilename string) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal the keys: %v", err))
	}

	err = os.WriteFile(ymlFilename+".yml", bytes, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("Could not write the config to file: %v", err))
	}
}

func init() {
}
