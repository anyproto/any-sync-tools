package cmd

import (
	"fmt"
	"os"
	"strconv"
	"path/filepath"

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
	DefaultLimits struct {
		SpaceMembersRead  int `yaml:"spaceMembersRead"`
		SpaceMembersWrite int `yaml:"spaceMembersWrite"`
		SharedSpacesLimit int `yaml:"sharedSpacesLimit"`
	} `yaml:"defaultLimits"`
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
		SyncOnStart       bool `yaml:"syncOnStart"`
		PeriodicSyncHours int  `yaml:"periodicSyncHours"`
	} `yaml:"nodeSync"`
	Log struct {
		Production   bool   `yaml:"production"`
		DefaultLevel string `yaml:"defaultLevel"`
		NamedLevels  struct {
		} `yaml:"namedLevels"`
	} `yaml:"log"`
	ApiServer struct {
		ListenAddr string `yaml:"listenAddr"`
	} `yaml:"apiServer"`
}

type FileNodeConfig struct {
	GeneralNodeConfig        `yaml:".,inline"`
	NetworkUpdateIntervalSec int `yaml:"networkUpdateIntervalSec"`
	DefaultLimit             int `yaml:"defaultLimit"`
	S3Store                  struct {
		Endpoint       string `yaml:"endpoint,omitempty"`
		Bucket         string `yaml:"bucket"`
		IndexBucket    string `yaml:"indexBucket"`
		Region         string `yaml:"region"`
		Profile        string `yaml:"profile"`
		MaxThreads     int    `yaml:"maxThreads"`
		ForcePathStyle bool   `yaml:"forcePathStyle"`
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
}

type DefaultConfig struct {
	ExternalAddr []string `yaml:"external-addresses"`

	AnySyncCoordinator struct {
		  ListenAddr    string `yaml:"listen"`
			YamuxPort     int `yaml:"yamuxPort"`
			QuicPort      int `yaml:"quicPort"`
			Mongo         struct {
					Connect  string `yaml:"connect"`
					Database string `yaml:"database"`
			} `yaml:"mongo"`
			DefaultLimits struct {
					SpaceMembersRead  int `yaml:"spaceMembersRead"`
					SpaceMembersWrite int `yaml:"spaceMembersWrite"`
					SharedSpacesLimit int `yaml:"sharedSpacesLimit"`
			} `yaml:"defaultLimits"`
	} `yaml:"any-sync-coordinator"`

	AnySyncConsensusNode struct {
		  ListenAddr    string `yaml:"listen"`
			YamuxPort int `yaml:"yamuxPort"`
			QuicPort  int `yaml:"quicPort"`
			Mongo     struct {
					Connect  string `yaml:"connect"`
					Database string `yaml:"database"`
			} `yaml:"mongo"`
	} `yaml:"any-sync-consensusnode"`

	AnySyncFilenode struct {
		  ListenAddr    string `yaml:"listen"`
			YamuxPort int `yaml:"yamuxPort"`
			QuicPort  int `yaml:"quicPort"`
			S3Store   struct {
					Endpoint       string `yaml:"endpoint"`
					Bucket         string `yaml:"bucket"`
					IndexBucket    string `yaml:"indexBucket"`
					Region         string `yaml:"region"`
					Profile        string `yaml:"profile"`
					ForcePathStyle bool   `yaml:"forcePathStyle"`
			} `yaml:"s3Store"`
			Redis struct {
					URL string `yaml:"url"`
			} `yaml:"redis"`
			DefaultLimit int `yaml:"defaultLimit"`
	} `yaml:"any-sync-filenode"`

	AnySyncNode struct {
		  ListenAddr []string `yaml:"listen"`
			YamuxPort  int `yaml:"yamuxPort"`
			QuicPort   int `yaml:"quicPort"`
	} `yaml:"any-sync-node"`
}

func loadDefaultTemplate() {
	data, err := os.ReadFile(templatePath)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		panic(err)
	}
}

var cfg DefaultConfig

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

		fmt.Println("\033[1m  Network ID:\033[0m", network.NetworkID)

		loadDefaultTemplate()

		// Create coordinator node
		fmt.Println("\nCreating coordinator node...")

		var defaultCoordinatorAddress = cfg.AnySyncCoordinator.ListenAddr
		var defaultCoordinatorYamuxPort = strconv.Itoa(cfg.AnySyncCoordinator.YamuxPort)
		var defaultCoordinatorQuicPort = strconv.Itoa(cfg.AnySyncCoordinator.QuicPort)
		var	defaultCoordinatorMongoConnect = cfg.AnySyncCoordinator.Mongo.Connect
		var	defaultCoordinatorMongoDb = cfg.AnySyncCoordinator.Mongo.Database

		var coordinatorQs = []*survey.Question{
			{
				Name: "address",
				Prompt: &survey.Input{
					Message: "Any-Sync Coordinator Node address (without port)",
					Default: defaultCoordinatorAddress,

				},
				Validate: survey.Required,
			},
			{
				Name: "yamuxPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Coordinator Node Yamux (TCP) port",
					Default: defaultCoordinatorYamuxPort,
				},
				Validate: survey.Required,
			},
			{
				Name: "quicPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Coordinator Node Quic (UDP) port",
					Default: defaultCoordinatorQuicPort,
				},
				Validate: survey.Required,
			},
			{
				Name: "mongoConnect",
				Prompt: &survey.Input{
					Message: "Mongo connect URI",
					Default: defaultCoordinatorMongoConnect,
				},
				Validate: survey.Required,
			},
			{
				Name: "mongoDB",
				Prompt: &survey.Input{
					Message: "Mongo database name",
					Default: defaultCoordinatorMongoDb,
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
		}{
			Address:      defaultCoordinatorAddress,
			YamuxPort:   	defaultCoordinatorYamuxPort,
			QuicPort:   	defaultCoordinatorQuicPort,
			MongoConnect:	defaultCoordinatorMongoConnect,
			MongoDB:      defaultCoordinatorMongoDb,
		}

		if !autoFlag {
			err := survey.Ask(coordinatorQs, &coordinatorAs)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		coordinatorNode := defaultCoordinatorNode()
		coordinatorNode.Yamux.ListenAddrs = append(coordinatorNode.Yamux.ListenAddrs, coordinatorAs.Address+":"+coordinatorAs.YamuxPort)
		coordinatorNode.Quic.ListenAddrs = append(coordinatorNode.Quic.ListenAddrs, coordinatorAs.Address+":"+coordinatorAs.QuicPort)
		coordinatorNode.Mongo.Connect = coordinatorAs.MongoConnect
		coordinatorNode.Mongo.Database = coordinatorAs.MongoDB
		coordinatorNode.Account = generateAccount()
		coordinatorNode.Account.SigningKey, _ = crypto.EncodeKeyToString(netKey)

		addToNetwork(coordinatorNode.GeneralNodeConfig, "coordinator")

		// Create consensus node
		fmt.Println("\nCreating consensus node...")

		var defaultConsensusAddress = cfg.AnySyncConsensusNode.ListenAddr
		var defaultConsensusYamuxPort = strconv.Itoa(cfg.AnySyncConsensusNode.YamuxPort)
		var defaultConsensusQuicPort = strconv.Itoa(cfg.AnySyncConsensusNode.QuicPort)
		var defaultConsensusMongoDB = cfg.AnySyncConsensusNode.Mongo.Database

		var consensusQs = []*survey.Question{
			{
				Name: "address",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Node address (without port)",
					Default: defaultConsensusAddress,
				},
				Validate: survey.Required,
			},
			{
				Name: "yamuxPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Node Yamux (TCP) port",
					Default: defaultConsensusYamuxPort,
				},
				Validate: survey.Required,
			},
			{
				Name: "quicPort",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Node Quic (UDP) port",
					Default: defaultConsensusQuicPort,
				},
				Validate: survey.Required,
			},
			{
				Name: "mongoDB",
				Prompt: &survey.Input{
					Message: "Any-Sync Consensus Mongo database name",
					Default: defaultConsensusMongoDB,
				},
				Validate: survey.Required,
			},
		}

		consensusAs := struct {
			Address   string
			YamuxPort string
			QuicPort  string
			MongoDB   string
		}{
			Address:   defaultConsensusAddress,
			YamuxPort: defaultConsensusYamuxPort,
			QuicPort:  defaultConsensusQuicPort,
			MongoDB:   defaultConsensusMongoDB,
		}

		if !autoFlag {
			err := survey.Ask(consensusQs, &consensusAs)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		}

		consensusNode := defaultConsensusNode()
		consensusNode.Yamux.ListenAddrs = append(consensusNode.Yamux.ListenAddrs, consensusAs.Address + ":" + consensusAs.YamuxPort)
		consensusNode.Quic.ListenAddrs = append(consensusNode.Quic.ListenAddrs, consensusAs.Address + ":" + consensusAs.QuicPort)
		consensusNode.Mongo.Connect = cfg.AnySyncConsensusNode.Mongo.Connect
		consensusNode.Mongo.Database = consensusAs.MongoDB
		consensusNode.Account = generateAccount()

		addToNetwork(consensusNode.GeneralNodeConfig, "consensus")

		listenCount := len(cfg.AnySyncNode.ListenAddr)
		if !autoFlag {
			createSyncNode(0)
		} else {
			for node := 0; node < listenCount; node++ {
				createSyncNode(node)
			}
		}
		createFileNode()

		lastStepOptions()

		// Create configurations for all nodes
		fmt.Println("\nCreating config file...")

		coordinatorNode.Network = network
		createConfigFile(coordinatorNode, "etc/any-sync-coordinator/config")

		consensusNode.Network = network
		createConfigFile(consensusNode, "etc/any-sync-consensusnode/config")

		for i, syncNode := range syncNodes {
			syncNode.Network = network
			createConfigFile(syncNode, "etc/any-sync-node-"+strconv.Itoa(i+1)+"/config")
		}

		for i, fileNode := range fileNodes {
			fileNode.Network = network
			if i == 0 {
				createConfigFile(fileNode, "etc/any-sync-filenode/config")
			} else {
				createConfigFile(fileNode, "etc/any-sync-filenode-"+strconv.Itoa(i+1)+"/config")
			}
		}

		createConfigFile(network.HeartConfig, "etc/client") // to import to client app
		createConfigFile(network.HeartConfig, "etc/any-sync-coordinator/network") // to any-sync-confapply tool

		fmt.Println("Done!")
	},
}

var network = Network{}

func addToNetwork(node GeneralNodeConfig, nodeType string) {
	addresses := []string{}
	yamuxPort := 0
	quicPort := 0

	switch nodeType {
	case "coordinator":
		yamuxPort = cfg.AnySyncCoordinator.YamuxPort
		quicPort = cfg.AnySyncCoordinator.QuicPort
	case "consensus":
		yamuxPort = cfg.AnySyncConsensusNode.YamuxPort
		quicPort = cfg.AnySyncConsensusNode.QuicPort
	case "file":
		yamuxPort = cfg.AnySyncFilenode.YamuxPort
		quicPort = cfg.AnySyncFilenode.QuicPort
	case "tree":
		yamuxPort = cfg.AnySyncNode.YamuxPort
		quicPort = cfg.AnySyncNode.QuicPort
	}

	for _, addr := range node.Yamux.ListenAddrs {
		addresses = append(addresses, addr)
	}
	for _, addr := range node.Quic.ListenAddrs {
		addresses = append(addresses, "quic://"+addr)
	}
	for _, extAddr := range cfg.ExternalAddr {
		addresses = append(addresses, extAddr+":"+strconv.Itoa(yamuxPort))
		addresses = append(addresses, "quic://"+extAddr+":"+strconv.Itoa(quicPort))
	}
	network.Nodes = append(network.Nodes, Node{
		PeerID:    node.Account.PeerId,
		Addresses: addresses,
		Types:     []string{nodeType},
	})
}

var syncNodes = []SyncNodeConfig{}

func createSyncNode(index int) {
	var defaultSyncNodeAddress = cfg.AnySyncNode.ListenAddr[index]
	var defaultSyncNodeYamuxPort = strconv.Itoa(cfg.AnySyncNode.YamuxPort)
	var defaultSyncNodeQuicPort = strconv.Itoa(cfg.AnySyncNode.QuicPort)

	fmt.Println("\nCreating sync node...")

	var syncQs = []*survey.Question{
		{
			Name: "address",
			Prompt: &survey.Input{
				Message: "Any-Sync Node address (without port)",
				Default: defaultSyncNodeAddress,
			},
			Validate: survey.Required,
		},
		{
			Name: "yamuxPort",
			Prompt: &survey.Input{
				Message: "Any-Sync Node Yamux (TCP) port",
				Default: defaultSyncNodeYamuxPort,
			},
			Validate: survey.Required,
		},
		{
			Name: "quicPort",
			Prompt: &survey.Input{
				Message: "Any-Sync Node Quic (UDP) port",
				Default: defaultSyncNodeQuicPort,
			},
			Validate: survey.Required,
		},
	}

	answers := struct {
		Address   string
		YamuxPort string
		QuicPort  string
	}{
		Address:   defaultSyncNodeAddress,
		YamuxPort: defaultSyncNodeYamuxPort,
		QuicPort:  defaultSyncNodeQuicPort,
	}

	if !autoFlag {
		err := survey.Ask(syncQs, &answers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}

	syncNode := defaultSyncNode()
	syncNode.Yamux.ListenAddrs = append(syncNode.Yamux.ListenAddrs, answers.Address+":"+answers.YamuxPort)
	syncNode.Quic.ListenAddrs = append(syncNode.Quic.ListenAddrs, answers.Address+":"+answers.QuicPort)
	syncNode.Account = generateAccount()

	addToNetwork(syncNode.GeneralNodeConfig, "tree")
	syncNodes = append(syncNodes, syncNode)

	// Increase sync node port
	cfg.AnySyncNode.YamuxPort++
	cfg.AnySyncNode.QuicPort++
}

var fileNodes = []FileNodeConfig{}

func createFileNode() {
	var defaultFileNodeAddress = cfg.AnySyncFilenode.ListenAddr
	var defaultFileNodeYamuxPort = strconv.Itoa(cfg.AnySyncFilenode.YamuxPort)
	var defaultFileNodeQuicPort = strconv.Itoa(cfg.AnySyncFilenode.QuicPort)
	var defaultS3Endpoint = cfg.AnySyncFilenode.S3Store.Endpoint
	var defaultS3Region = cfg.AnySyncFilenode.S3Store.Region
	var defaultS3Profile = cfg.AnySyncFilenode.S3Store.Profile
	var defaultS3Bucket = cfg.AnySyncFilenode.S3Store.Bucket
	var defaultRedisUrl = cfg.AnySyncFilenode.Redis.URL
	var defaultRedisCluster = "false"

	fmt.Println("\nCreating file node...")

	var fileQs = []*survey.Question{
		{
			Name: "address",
			Prompt: &survey.Input{
				Message: "Any-Sync File Node address (without port)",
				Default: defaultFileNodeAddress,
			},
			Validate: survey.Required,
		},
		{
			Name: "yamuxPort",
			Prompt: &survey.Input{
				Message: "Any-Sync File Node Yamux (TCP) port",
				Default:  defaultFileNodeYamuxPort,
			},
			Validate: survey.Required,
		},
		{
			Name: "quicPort",
			Prompt: &survey.Input{
				Message: "Any-Sync File Node Quic (UDP) port",
				Default:  defaultFileNodeQuicPort,
			},
			Validate: survey.Required,
		},
		{
			Name: "s3Endpoint",
			Prompt: &survey.Input{
				Message: "S3 Endpoint",
				Help: "Required only in the case you self-host S3-compatible object storage",
				Default: defaultS3Endpoint,
			},
		},
		{
			Name: "s3Region",
			Prompt: &survey.Input{
				Message: "S3 Region",
				Default: defaultS3Region,
			},
			Validate: survey.Required,
		},
		{
			Name: "s3Profile",
			Prompt: &survey.Input{
				Message: "S3 Profile",
				Default: defaultS3Profile,
			},
			Validate: survey.Required,
		},
		{
			Name: "s3Bucket",
			Prompt: &survey.Input{
				Message: "S3 Bucket",
				Default: defaultS3Bucket,
			},
			Validate: survey.Required,
		},
		{
			Name: "redisURL",
			Prompt: &survey.Input{
				Message: "Redis URL",
				Default: defaultRedisUrl,
			},
			Validate: survey.Required,
		},
		{
			Name: "redisCluster",
			Prompt: &survey.Select{
				Message: "Is your redis installation a cluster?",
				Options: []string{"true", "false"},
				Default: defaultRedisCluster,
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
	}{
		Address:      defaultFileNodeAddress,
		YamuxPort:    defaultFileNodeYamuxPort,
		QuicPort:     defaultFileNodeQuicPort,
		S3Endpoint:   defaultS3Endpoint,
		S3Region:     defaultS3Region,
		S3Profile:    defaultS3Profile,
		S3Bucket:     defaultS3Bucket,
		RedisURL:     defaultRedisUrl,
		RedisCluster: defaultRedisCluster,
	}

	if !autoFlag {
		err := survey.Ask(fileQs, &answers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
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
	cfg.AnySyncFilenode.YamuxPort++
	cfg.AnySyncFilenode.QuicPort++
}

func lastStepOptions() {
	fmt.Println()
	prompt := &survey.Select{
		Message: "Do you want to add more nodes?",
		Options: []string{"No, generate configs", "Add sync-node", "Add file-node"},
		Default: "No, generate configs",
	}

	if !autoFlag {
		option := ""
		survey.AskOne(prompt, &option, survey.WithValidator(survey.Required))
		switch option {
		case "Add sync-node":
			createSyncNode(0)
			lastStepOptions()
		case "Add file-node":
			createFileNode()
			lastStepOptions()
		default:
			return
		}
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
		NetworkStorePath: "/networkStore",
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
		DefaultLimits: struct {
			SpaceMembersRead  int "yaml:\"spaceMembersRead\""
			SpaceMembersWrite int "yaml:\"spaceMembersWrite\""
			SharedSpacesLimit int "yaml:\"sharedSpacesLimit\""
		}{
			SpaceMembersRead:  cfg.AnySyncCoordinator.DefaultLimits.SpaceMembersRead,
			SpaceMembersWrite: cfg.AnySyncCoordinator.DefaultLimits.SpaceMembersWrite,
			SharedSpacesLimit: cfg.AnySyncCoordinator.DefaultLimits.SharedSpacesLimit,
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
			SyncOnStart       bool "yaml:\"syncOnStart\""
			PeriodicSyncHours int  "yaml:\"periodicSyncHours\""
		}{
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
		ApiServer: struct {
			ListenAddr string "yaml:\"listenAddr\""
		}{
			ListenAddr: "0.0.0.0:8080",
		},
	}
}

func defaultFileNode() FileNodeConfig {
	return FileNodeConfig{
		GeneralNodeConfig:        defaultGeneralNode(),
		NetworkUpdateIntervalSec: 600,
		DefaultLimit:             cfg.AnySyncFilenode.DefaultLimit,
		S3Store: struct {
			Endpoint       string "yaml:\"endpoint,omitempty\""
			Bucket         string "yaml:\"bucket\""
			IndexBucket    string "yaml:\"indexBucket\""
			Region         string "yaml:\"region\""
			Profile        string "yaml:\"profile\""
			MaxThreads     int    "yaml:\"maxThreads\""
			ForcePathStyle bool   "yaml:\"forcePathStyle\""
		}{
			MaxThreads: 16,
			IndexBucket: cfg.AnySyncFilenode.S3Store.IndexBucket,
			ForcePathStyle: cfg.AnySyncFilenode.S3Store.ForcePathStyle,
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

	dir := filepath.Dir(ymlFilename)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		panic(fmt.Sprintf("Could not create the directory: %v", err))
	}

	err = os.WriteFile(ymlFilename+".yml", bytes, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("Could not write the config to file: %v", err))
	}
}

func init() {
}
