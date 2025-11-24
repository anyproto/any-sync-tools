package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anyproto/any-sync/accountservice"
	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
	"github.com/anyproto/any-sync/commonspace/object/accountdata"
	"github.com/anyproto/any-sync/commonspace/object/acl/aclrecordproto"
	"github.com/anyproto/any-sync/commonspace/object/acl/list"
	"github.com/anyproto/any-sync/commonspace/object/acl/recordverifier"
	"github.com/anyproto/any-sync/consensus/consensusproto"
	"github.com/anyproto/any-sync/coordinator/coordinatorclient"
	"github.com/anyproto/any-sync/coordinator/nodeconfsource"
	"github.com/anyproto/any-sync/net/peerservice"
	"github.com/anyproto/any-sync/net/pool"
	"github.com/anyproto/any-sync/net/rpc"
	"github.com/anyproto/any-sync/net/rpc/server"
	"github.com/anyproto/any-sync/net/secureservice"
	"github.com/anyproto/any-sync/net/transport/quic"
	"github.com/anyproto/any-sync/net/transport/yamux"
	"github.com/anyproto/any-sync/nodeconf"
	"github.com/anyproto/any-sync/nodeconf/nodeconfstore"
	"github.com/anyproto/any-sync/testutil/accounttest"
	"github.com/anyproto/any-sync/util/crypto"
	"gopkg.in/yaml.v3"
)

var (
	fNetwork = flag.String("n", "", "path to network config yml file")
	fHelp    = flag.Bool("h", false, "print this help")
	fSpaceId = flag.String("s", "", "spaceId")
	fLog     = flag.Bool("l", false, "show log")
)

func main() {
	flag.Parse()
	if *fHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

	logger.SetNamedLevels(logger.LevelsFromStr("*=WARN"))

	var ctx = context.Background()

	if *fNetwork == "" {
		log.Fatal("network config file not specified")
	}

	data, err := os.ReadFile(*fNetwork)
	if err != nil {
		log.Fatal(err)
	}

	var netConf nodeconf.Configuration
	if err = yaml.Unmarshal(data, &netConf); err != nil {
		log.Fatal(err)
	}

	cli, err := newApp(&Config{Network: netConf}, ctx)
	if err != nil {
		log.Fatal(err)
	}

	if *fLog {
		if err = cli.Log(ctx, *fSpaceId); err != nil {
			log.Fatal(err)
		}
	}
	if err = cli.Summary(ctx, *fSpaceId); err != nil {
		log.Fatal(err)
	}
}

type AclCli struct {
	coordinatorClient coordinatorclient.CoordinatorClient
	account           *accountdata.AccountKeys
	recordsBySpaceId  map[string][]*consensusproto.RawRecordWithId
}

func (a *AclCli) Init(ap *app.App) (err error) {
	a.coordinatorClient = ap.MustComponent(coordinatorclient.CName).(coordinatorclient.CoordinatorClient)
	a.account = ap.MustComponent(accountservice.CName).(accountservice.Service).Account()
	a.recordsBySpaceId = make(map[string][]*consensusproto.RawRecordWithId)
	return
}

func (a *AclCli) Name() (name string) {
	return "acl.cli"
}

func (a *AclCli) Log(ctx context.Context, spaceId string) (err error) {
	records, err := a.coordinatorClient.AclGetRecords(ctx, spaceId, "")
	if err != nil {
		return
	}
	for i, rec := range records {
		if i == 0 {
			if err = printAclRoot(rec); err != nil {
				return
			}
		} else {
			if err = printAclRecord(rec); err != nil {
				return
			}
		}
	}
	a.recordsBySpaceId[spaceId] = records
	return
}

func (a *AclCli) Summary(ctx context.Context, spaceId string) (err error) {
	records, ok := a.recordsBySpaceId[spaceId]
	if !ok {
		records, err = a.coordinatorClient.AclGetRecords(ctx, spaceId, "")
		if err != nil {
			return fmt.Errorf("failed to get acl records: %w", err)
		}
	}
	store, err := list.NewInMemoryStorage(spaceId, records)
	if err != nil {
		return fmt.Errorf("failed to create acl storage: %w", err)
	}
	aclList, err := list.BuildAclListWithIdentity(a.account, store, recordverifier.NewValidateFull())
	if err != nil {
		return fmt.Errorf("failed to build acl list: %w", err)
	}
	state := aclList.AclState()
	owner, err := state.OwnerPubKey()
	if err != nil {
		return fmt.Errorf("failed to get acl owner: %w", err)
	}
	fmt.Println("ACL state summary:")
	fmt.Printf("\tOwner=%s\n", formatAccount(owner.Account()))

	fmt.Println("\tInvites:")
	for _, inv := range state.Invites() {
		fmt.Printf("\t\tid=%v; type=%v\n", formatId(inv.Id), inv.Type.String())
	}

	fmt.Println("\tAccount statuses:")
	var accByState [7]int
	var accByRole [6]int
	for _, acc := range state.CurrentAccounts() {
		accByRole[acc.Permissions]++
		accByState[acc.Status]++
	}

	var statusNames = [7]string{"none", "joining", "active", "removed", "declined", "removing", "canceled"}

	for i, cnt := range accByState {
		status := statusNames[i]
		if cnt > 0 {
			fmt.Printf("\t\t%v=%d\n", status, cnt)
		}
	}

	fmt.Println("\tAccount roles:")
	for i, cnt := range accByRole {
		role := aclrecordproto.AclUserPermissions(i).String()
		if cnt > 0 {
			fmt.Printf("\t\t%v=%d\n", role, cnt)
		}
	}

	return
}

func printAclRoot(rec *consensusproto.RawRecordWithId) (err error) {
	var rawRec = &consensusproto.RawRecord{}
	if err = rawRec.UnmarshalVT(rec.Payload); err != nil {
		return fmt.Errorf("unmarshal raw root record failed: %v", err)
	}

	acceptTime := time.Unix(rawRec.AcceptorTimestamp, 0)

	aclRec := &aclrecordproto.AclRoot{}
	if err = aclRec.UnmarshalVT(rawRec.Payload); err != nil {
		return fmt.Errorf("unmarshal acl root record failed: %v", err)
	}

	var rootFlags []string

	if aclRec.MasterKey != nil {
		rootFlags = append(rootFlags, "masterKey")
	}
	if aclRec.MetadataPubKey != nil {
		rootFlags = append(rootFlags, "metaKey")
	}
	if aclRec.EncryptedReadKey != nil {
		rootFlags = append(rootFlags, "readKey")
	}
	if aclRec.OneToOneInfo != nil {
		rootFlags = append(rootFlags, "oneToOne")
	}

	fmt.Printf("%s\troot\t%s\t%s\t%s\n", formatId(rec.Id), formatIdentity(aclRec.Identity), acceptTime.Format(time.RFC3339), strings.Join(rootFlags, ","))
	return
}

func formatIdentity(ident []byte) string {
	key, err := crypto.UnmarshalEd25519PublicKeyProto(ident)
	if err != nil {
		log.Fatal(err)
	}
	if key != nil {
		accId := key.Account()
		return formatAccount(accId)
	}
	return "invalid"
}

func formatAccount(accountId string) string {
	return accountId[:4] + ".." + accountId[len(accountId)-4:]
}

func printAclRecord(rec *consensusproto.RawRecordWithId) (err error) {
	var rawRec = &consensusproto.RawRecord{}
	if err = rawRec.UnmarshalVT(rec.Payload); err != nil {
		return fmt.Errorf("unmarshal raw record failed: %v", err)
	}

	acceptTime := time.Unix(rawRec.AcceptorTimestamp, 0)

	cRec := &consensusproto.Record{}
	if err = cRec.UnmarshalVT(rawRec.Payload); err != nil {
		return fmt.Errorf("unmarshal consensus record failed: %v", err)
	}

	aclData := &aclrecordproto.AclData{}
	if err = aclData.UnmarshalVT(cRec.Data); err != nil {
		return fmt.Errorf("unmarshal acl data failed: %v", err)
	}

	for _, cont := range aclData.GetAclContent() {
		var aclType string
		var info []string

		switch {
		case cont.GetOwnershipChange() != nil:
			aclType = "ownerChange"
			info = append(
				info,
				fmt.Sprintf("newOwner=%s", formatIdentity(cont.GetOwnershipChange().GetNewOwnerIdentity())),
			)
		case cont.GetInviteChange() != nil:
			aclType = "inviteChange"
			info = append(
				info,
				fmt.Sprintf("invId=%s", formatId(cont.GetInviteChange().GetInviteRecordId())),
				fmt.Sprintf("perm=%s", cont.GetInviteChange().GetPermissions().String()),
			)
		case cont.GetInviteJoin() != nil:
			aclType = "inviteJoin"
			info = append(
				info,
				fmt.Sprintf("invId=%s", formatId(cont.GetInviteJoin().GetInviteRecordId())),
				fmt.Sprintf("perm=%s", cont.GetInviteJoin().GetPermissions().String()),
			)
			if meta := cont.GetInviteJoin().GetMetadata(); len(meta) > 0 {
				info = append(info, "meta")
			}
		case cont.GetPermissionChange() != nil:
			aclType = "permChange"
			info = append(
				info,
				fmt.Sprintf("ident=%s", formatIdentity(cont.GetPermissionChange().GetIdentity())),
				fmt.Sprintf("perm=%s", cont.GetPermissionChange().GetPermissions().String()),
			)
		case cont.GetInvite() != nil:
			aclType = "invite"
			info = append(
				info,
				fmt.Sprintf("type=%s", cont.GetInvite().GetInviteType().String()),
				fmt.Sprintf("perm=%s", cont.GetInvite().GetPermissions().String()),
			)
		case cont.GetInviteRevoke() != nil:
			aclType = "inviteRevoke"
			info = append(
				info,
				fmt.Sprintf("invId=%s", formatId(cont.GetInviteRevoke().GetInviteRecordId())),
			)
		case cont.GetRequestJoin() != nil:
			aclType = "requestJoin"
			info = append(
				info,
				fmt.Sprintf("invId=%s", formatId(cont.GetRequestJoin().GetInviteRecordId())),
				fmt.Sprintf("ident=%s", formatIdentity(cont.GetRequestJoin().GetInviteIdentity())),
			)
		case cont.GetRequestAccept() != nil:
			aclType = "requestAccept"
			info = append(
				info,
				fmt.Sprintf("reqId=%s", formatId(cont.GetRequestAccept().GetRequestRecordId())),
				fmt.Sprintf("ident=%s", formatIdentity(cont.GetRequestAccept().GetIdentity())),
			)
		case cont.GetRequestDecline() != nil:
			aclType = "requestDecline"
			info = append(
				info,
				fmt.Sprintf("reqId=%s", formatId(cont.GetRequestDecline().GetRequestRecordId())),
			)
		case cont.GetRequestCancel() != nil:
			aclType = "requestCancel"
			info = append(
				info,
				fmt.Sprintf("reqId=%s", formatId(cont.GetRequestCancel().GetRecordId())),
			)
		case cont.GetAccountRemove() != nil:
			aclType = "accountRemove"
			for _, ident := range cont.GetAccountRemove().GetIdentities() {
				info = append(
					info,
					fmt.Sprintf("ident=%s", formatIdentity(ident)),
				)
			}
			info = append(
				info,
				fmt.Sprintf("newKeyForIdents=%d", len(cont.GetAccountRemove().GetReadKeyChange().GetAccountKeys())),
			)
		case cont.GetReadKeyChange() != nil:
			aclType = "readKeyChange"
			for _, invKey := range cont.GetReadKeyChange().GetInviteKeys() {
				info = append(
					info,
					fmt.Sprintf("ikIdent=%s", formatIdentity(invKey.GetIdentity())),
				)
			}
			for _, accKey := range cont.GetReadKeyChange().GetAccountKeys() {
				info = append(
					info,
					fmt.Sprintf("accIdent=%s", formatIdentity(accKey.GetIdentity())),
				)
			}
		case cont.GetAccountRequestRemove() != nil:
			aclType = "accountRequestRemove"
		case cont.GetAccountsAdd() != nil:
			aclType = "accountsAdd"
			for _, add := range cont.GetAccountsAdd().GetAdditions() {
				info = append(
					info,
					fmt.Sprintf("ident=%s", formatIdentity(add.GetIdentity())),
				)
			}
		case cont.GetPermissionChanges() != nil:
			aclType = "permissionChanges"
			for _, pCh := range cont.GetPermissionChanges().GetChanges() {
				info = append(
					info,
					fmt.Sprintf("ident=%s", formatIdentity(pCh.GetIdentity())),
					fmt.Sprintf("perm=%s", pCh.GetPermissions().String()),
				)
			}
		default:
			aclType = "unknown"
			info = append(info, aclData.String())
		}

		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", formatId(rec.Id), aclType, formatIdentity(cRec.Identity), acceptTime.Format(time.RFC3339), strings.Join(info, ","))
	}
	return
}

func formatId(id string) (res string) {
	if len(id) < 5 {
		return id
	}
	return id[len(id)-5:]
}

func newApp(conf *Config, ctx context.Context) (aclCli *AclCli, err error) {
	a := new(app.App)
	aclCli = new(AclCli)
	a.Register(conf).
		Register(&accounttest.AccountTestService{}).
		Register(nodeconfsource.New()).
		Register(nodeconfstore.New()).
		Register(nodeconf.New()).
		Register(coordinatorclient.New()).
		Register(secureservice.New()).
		Register(peerservice.New()).
		Register(server.New()).
		Register(yamux.New()).
		Register(quic.New()).
		Register(pool.New()).
		Register(aclCli)

	if err = a.Start(ctx); err != nil {
		return
	}
	return
}

type Config struct {
	Network nodeconf.Configuration
}

func (c Config) Name() string          { return "config" }
func (c Config) Init(a *app.App) error { return nil }

func (c Config) GetYamux() yamux.Config {
	return yamux.Config{
		WriteTimeoutSec:    60,
		DialTimeoutSec:     60,
		KeepAlivePeriodSec: 120,
	}
}

func (c Config) GetQuic() quic.Config {
	return quic.Config{
		WriteTimeoutSec:    60,
		DialTimeoutSec:     60,
		KeepAlivePeriodSec: 120,
	}
}

func (c Config) GetNodeConf() nodeconf.Configuration {
	return c.Network
}

func (c Config) GetNodeConfStorePath() string {
	return "."
}

func (c Config) GetDrpc() rpc.Config {
	return rpc.Config{
		Stream: rpc.StreamConfig{
			MaxMsgSizeMb: 256,
		},
		Snappy: true,
	}
}
