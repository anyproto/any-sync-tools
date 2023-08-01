package main

import (
	"context"
	"flag"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/coordinator/coordinatorproto"
	"github.com/anyproto/any-sync/net/connutil"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/secureservice/handshake"
	"github.com/anyproto/any-sync/net/secureservice/handshake/handshakeproto"
	"github.com/anyproto/any-sync/net/transport"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/go-chash"
	yamux2 "github.com/hashicorp/yamux"
	"github.com/matishsiao/goInfo"
	"go.uber.org/zap"
	"net"
	"storj.io/drpc/drpcconn"
	"time"
)

var ctx = context.Background()

var log = logger.NewNamed("netcheck")

var verbose = flag.Bool("v", false, "verbose logs")

func main() {
	flag.Parse()

	if *verbose {
		logger.SetNamedLevels(logger.LevelsFromStr("*=DEBUG"))
	} else {
		logger.SetNamedLevels(logger.LevelsFromStr("*=INFO"))
	}

	info, err := goInfo.GetInfo()
	if err != nil {
		log.Warn("can't get system info", zap.Error(err))
	} else {
		info.VarDump()
	}
	a := new(app.App)
	bootstrap(a)
	if err := a.Start(ctx); err != nil {
		panic(err)
	}

	var addrs = []string{"prod-any-sync-coordinator1.toolpad.org:443", "prod-any-sync-coordinator2.toolpad.org:443", "prod-any-sync-coordinator3.toolpad.org:443"}
	for _, addr := range addrs {
		probe(a, addr)
	}
}

func probe(a *app.App, addr string) {
	ss := a.MustComponent(secureservice.CName).(secureservice.SecureService)
	l := log.With(zap.String("addr", addr))

	l.Debug("open TCP conn")
	st := time.Now()
	conn, err := net.DialTimeout("tcp", addr, time.Second*60)
	if err != nil {
		l.Warn("open TCP conn error", zap.Error(err), zap.Duration("dur", time.Since(st)))
		return
	} else {
		l.Debug("TCP conn established", zap.Duration("dur", time.Since(st)))
		l = l.With(zap.String("ip", conn.RemoteAddr().String()))
	}
	defer conn.Close()

	l.Debug("start handshake")
	hst := time.Now()
	cctx, err := ss.SecureOutbound(ctx, conn)
	if err != nil {
		l.Warn("handshake error", zap.Error(err), zap.Duration("dur", time.Since(hst)))
		return
	} else {
		l.Debug("handshake success", zap.Duration("dur", time.Since(hst)), zap.Duration("total", time.Since(st)))
	}

	yst := time.Now()
	l.Debug("open yamux session")
	sess, err := yamux2.Client(conn, yamux2.DefaultConfig())
	if err != nil {
		l.Warn("yamux session error", zap.Error(err), zap.Duration("dur", time.Since(yst)))
		return
	} else {
		l.Debug("yamux session success", zap.Duration("dur", time.Since(yst)), zap.Duration("total", time.Since(st)))
	}

	mc := yamux.NewMultiConn(cctx, connutil.NewLastUsageConn(conn), conn.RemoteAddr().String(), sess)
	l.Debug("open sub connection")
	scst := time.Now()
	sc, err := mc.Open(ctx)
	if err != nil {
		l.Warn("open sub connection error", zap.Error(err), zap.Duration("dur", time.Since(scst)))
		return
	} else {
		l.Debug("open sub conn success", zap.Duration("dur", time.Since(scst)), zap.Duration("total", time.Since(st)))
		defer sc.Close()
	}

	l.Debug("start proto handshake")
	phst := time.Now()
	if err = handshake.OutgoingProtoHandshake(ctx, sc, handshakeproto.ProtoType_DRPC); err != nil {
		l.Warn("proto handshake error", zap.Duration("dur", time.Since(phst)), zap.Error(err))
		return
	} else {
		l.Debug("proto handshake success", zap.Duration("dur", time.Since(phst)), zap.Duration("total", time.Since(st)))
	}

	l.Debug("start configuration request")
	rst := time.Now()
	resp, err := coordinatorproto.NewDRPCCoordinatorClient(drpcconn.New(sc)).NetworkConfiguration(ctx, &coordinatorproto.NetworkConfigurationRequest{})
	if err != nil {
		l.Warn("configuration request error", zap.Error(err), zap.Duration("dur", time.Since(rst)))
		return
	} else {
		l.Debug("configuration request success", zap.Duration("dur", time.Since(rst)), zap.Duration("total", time.Since(st)), zap.String("nid", resp.GetNetworkId()))
	}
	l.Info("success", zap.Duration("dur", time.Since(st)))
}

func printDebugInfo() {

}

func bootstrap(a *app.App) {
	a.Register(&config{}).
		Register(&nodeConf{}).
		Register(&accounttest.AccountTestService{}).
		Register(secureservice.New())
}

type config struct {
}

func (c config) Name() string          { return "config" }
func (c config) Init(a *app.App) error { return nil }

func (c config) GetYamux() yamux.Config {
	return yamux.Config{
		WriteTimeoutSec:    60,
		DialTimeoutSec:     60,
		KeepAlivePeriodSec: 120,
	}
}

type nodeConf struct {
}

func (n nodeConf) Id() string {
	return "test"
}

func (n nodeConf) Configuration() nodeconf.Configuration {
	return nodeconf.Configuration{
		Id:           "test",
		NetworkId:    "",
		Nodes:        nil,
		CreationTime: time.Time{},
	}
}

func (n nodeConf) NodeIds(spaceId string) []string {
	return nil
}

func (n nodeConf) IsResponsible(spaceId string) bool {
	return false
}

func (n nodeConf) FilePeers() []string {
	return nil
}

func (n nodeConf) ConsensusPeers() []string {
	return nil
}

func (n nodeConf) CoordinatorPeers() []string {
	return nil
}

func (n nodeConf) PeerAddresses(peerId string) (addrs []string, ok bool) {
	return nil, false
}

func (n nodeConf) CHash() chash.CHash {
	return nil
}

func (n nodeConf) Partition(spaceId string) (part int) {
	return 0
}

func (n nodeConf) NodeTypes(nodeId string) []nodeconf.NodeType {
	return []nodeconf.NodeType{nodeconf.NodeTypeCoordinator}
}

func (n nodeConf) NetworkCompatibilityStatus() nodeconf.NetworkCompatibilityStatus {
	return 0
}

func (n nodeConf) Init(a *app.App) (err error) {
	return nil
}

func (n nodeConf) Name() (name string) {
	return nodeconf.CName
}

func (n nodeConf) Run(ctx context.Context) (err error) {
	return nil
}

func (n nodeConf) Close(ctx context.Context) (err error) {
	return nil
}

type accepter struct {
}

func (a accepter) Accept(mc transport.MultiConn) (err error) {
	return nil
}
