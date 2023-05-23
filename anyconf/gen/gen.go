package gen

import (
	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/net"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/util/crypto"
	"golang.org/x/exp/slices"
)

type NodeConfigInfo struct {
	Config         any
	DebugApiServer net.Config
	Account        accountservice.Config
	Nodes          nodeconf.Configuration
}

type NodeParameters struct {
	DebugAddress, Address, DBPath string
	NodeType                      nodeconf.NodeType
}

func GenNodeConfig(addresses []string, types []nodeconf.NodeType, netKey crypto.PrivKey) (nc nodeconf.Node, ac accountservice.Config, err error) {

	signKey, _, err := crypto.GenerateRandomEd25519KeyPair()
	if err != nil {
		return
	}

	encPeerSignKey, err := crypto.EncodeKeyToString(signKey) //encSignKey
	if err != nil {
		return
	}

	peerID := signKey.GetPublic().PeerId()

	nc = nodeconf.Node{
		PeerId:    peerID,
		Addresses: addresses,
		Types:     types,
	}

	encSignKey := encPeerSignKey
	if slices.Contains(types, nodeconf.NodeTypeCoordinator) {
		if netKey != nil {
			encSignKey, _ = crypto.EncodeKeyToString(netKey)
		} else {
			encSignKey = ""
		}
	}

	ac = accountservice.Config{
		PeerId:     peerID,
		PeerKey:    encPeerSignKey,
		SigningKey: encSignKey,
	}

	return
}

func GenerateNodesConfigs(nodes []NodeParameters) (nodesConf []nodeconf.Node, accounts []accountservice.Config, err error) {
	for _, node := range nodes {
		commonConfig, accountConfig, err := GenNodeConfig([]string{node.Address}, []nodeconf.NodeType{node.NodeType}, nil)

		if err != nil {
			panic(err)
		}

		nodesConf = append(nodesConf, commonConfig)
		accounts = append(accounts, accountConfig)
	}

	return
}

/*


func GenerateFullNodesConfigs(nodes []NodeParameters, additionalAccounts []nodeconf.Node) (fullNodesConfig []NodeConfigInfo, err error) {
	nodesConf, accounts, err := GenerateNodesConfigs(nodes)
	nodesConf = append(nodesConf, additionalAccounts...)

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
*/
