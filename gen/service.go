package gen

import (
	"fmt"
	coordinatorConf "github.com/anytypeio/any-sync-coordinator/config"
	"github.com/anytypeio/any-sync-coordinator/db"
	"github.com/anytypeio/any-sync-coordinator/spacestatus"
	"github.com/anytypeio/any-sync-node/config"
	"github.com/anytypeio/any-sync-node/nodestorage"
	"github.com/anytypeio/any-sync-node/nodesync"
	"github.com/anytypeio/any-sync/accountservice"
	commonaccount "github.com/anytypeio/any-sync/accountservice"
	"github.com/anytypeio/any-sync/app/logger"
	"github.com/anytypeio/any-sync/commonspace"
	"github.com/anytypeio/any-sync/metric"
	"github.com/anytypeio/any-sync/net"
	"github.com/anytypeio/any-sync/nodeconf"
	"github.com/anytypeio/any-sync/util/keys"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/encryptionkey"
	"github.com/anytypeio/any-sync/util/keys/asymmetric/signingkey"
	"github.com/anytypeio/any-sync/util/peer"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/badgerprovider"
	clconfig "github.com/anytypeio/go-anytype-infrastructure-experiments/client/config"
)

type NodeConfigInfo struct {
	Config         any
	DebugApiServer net.Config
	Account        commonaccount.Config
	Nodes          []nodeconf.NodeConfig
}

type NodeParameters struct {
	DebugAddress, Address, DBPath string
	NodeType                      nodeconf.NodeType
}

func GenNodeConfig(addresses []string, types []nodeconf.NodeType) (nodeconf.NodeConfig, accountservice.Config, error) {
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

func GenerateNodesConfigs(nodes []NodeParameters) (nodesConf []nodeconf.NodeConfig, accounts []accountservice.Config, err error) {
	for _, node := range nodes {
		commonConfig, accountConfig, err := GenNodeConfig([]string{node.Address}, []nodeconf.NodeType{node.NodeType})

		if err != nil {
			panic(err)
		}

		nodesConf = append(nodesConf, commonConfig)
		accounts = append(accounts, accountConfig)
	}

	return
}

func GenerateFullNodesConfigs(nodes []NodeParameters) (fullNodesConfig []NodeConfigInfo, err error) {
	nodesConf, accounts, err := GenerateNodesConfigs(nodes)

	if err != nil {
		panic(err)
	}

	stream := net.StreamConfig{
		TimeoutMilliseconds: 1000,
		MaxMsgSizeMb:        256,
	}

	for index, account := range accounts {
		nodeConf := nodesConf[index]
		debugAddress := nodes[index].DebugAddress
		nodeType := nodes[index].NodeType
		debugServer := net.Config{
			Server: net.ServerConfig{ListenAddrs: []string{debugAddress}},
			Stream: stream,
		}
		dbPath := nodes[index].DBPath
		grpcServcer := net.Config{
			Server: net.ServerConfig{ListenAddrs: nodeConf.Addresses},
			Stream: stream,
		}

		var anyConf any

		switch nodeType {
		case nodeconf.NodeTypeCoordinator:
			anyConf = coordinatorConf.Config{
				Account:    account,
				GrpcServer: grpcServcer,
				Metric:     metric.Config{""},
				Nodes:      nodesConf,
				Mongo: db.Mongo{
					Connect:          "mongodb://localhost:27017",
					Database:         "coordinator_test",
					SpacesCollection: "spaces",
					LogCollection:    "log",
				},
				SpaceStatus: spacestatus.Config{RunSeconds: 20, DeletionPeriodDays: 0},
			}
		default:
			anyConf = config.Config{
				GrpcServer: grpcServcer,
				Account:    account,
				APIServer:  debugServer,
				Nodes:      nodesConf,
				Space: commonspace.Config{
					GCTTL:      60,
					SyncPeriod: 20,
				},
				Storage: nodestorage.Config{Path: dbPath},
				Metric:  metric.Config{""},
				Log: logger.Config{
					Production:   false,
					DefaultLevel: "",
					NamedLevels:  make(map[string]string),
				},
				NodeSync: nodesync.Config{
					SyncOnStart:       false,
					PeriodicSyncHours: 0,
				},
			}
		}

		nodeInfo := NodeConfigInfo{
			Config:         anyConf,
			DebugApiServer: debugServer,
			Account:        account,
			Nodes:          nodesConf,
		}

		fullNodesConfig = append(fullNodesConfig, nodeInfo)
	}

	return
}

// Temporary here
func GenerateClientConfig(nodesConfig []nodeconf.NodeConfig, address string, grpcPort, debugPort int) (cfg clconfig.Config, err error) {
	encClientKey, _, err := encryptionkey.GenerateRandomRSAKeyPair(2048)
	if err != nil {
		panic(fmt.Sprintf("could not generate client encryption key: %s", err.Error()))
	}

	signClientKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		panic(fmt.Sprintf("could not generate client signing key: %s", err.Error()))
	}

	peerKey, _, err := signingkey.GenerateRandomEd25519KeyPair()
	if err != nil {
		return clconfig.Config{}, err
	}

	encEncKey, err := keys.EncodeKeyToString(encClientKey)
	if err != nil {
		return clconfig.Config{}, err
	}

	encSignKey, err := keys.EncodeKeyToString(signClientKey)
	if err != nil {
		return clconfig.Config{}, err
	}

	encPeerKey, err := keys.EncodeKeyToString(peerKey)
	if err != nil {
		return clconfig.Config{}, err
	}

	peerID, err := peer.IdFromSigningPubKey(peerKey.GetPublic())
	if err != nil {
		return clconfig.Config{}, err
	}

	debugAddress := fmt.Sprintf("%s:%d", address, debugPort)
	grpcAddress := fmt.Sprintf("%s:%d", address, grpcPort)
	return clconfig.Config{
		GrpcServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: []string{grpcAddress},
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Storage: badgerprovider.Config{Path: fmt.Sprintf("db/client/%d/data", debugPort)},
		Account: commonaccount.Config{
			PeerId:        peerID.String(),
			PeerKey:       encPeerKey,
			SigningKey:    encSignKey,
			EncryptionKey: encEncKey,
		},
		APIServer: net.Config{
			Server: net.ServerConfig{
				ListenAddrs: []string{debugAddress},
			},
			Stream: net.StreamConfig{
				TimeoutMilliseconds: 1000,
				MaxMsgSizeMb:        256,
			},
		},
		Nodes: nodesConfig,
		Space: commonspace.Config{
			GCTTL:      60,
			SyncPeriod: 20,
		},
	}, nil
}
