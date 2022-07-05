package embedded_test

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"testing"

	godog "github.com/cucumber/godog"
	cid "github.com/ipfs/go-cid"
	rovy "go.rovy.net"
	rovynode "go.rovy.net/node"
)

func TestEmbedded(t *testing.T) {
	suite := godog.TestSuite{
		Options: &godog.Options{
			Format:        "pretty",
			Paths:         []string{"../features"},
			NoColors:      false,
			TestingT:      t,
			Randomize:     -1, // let godog generate the seed
			StopOnFailure: true,
			Strict:        false,
		},
	}

	suite.ScenarioInitializer = func(sctx *godog.ScenarioContext) {
		sctx.Step(`^the following test node keyfiles:$`, theFollowingTestNodeKeyfiles)
		sctx.Step(`^node '(\w+)' from test keyfile '(\w+)'$`, nodeFromTestKeyfile)
		sctx.Step(`^I start node '(\w+)'$`, iStartNode)
		sctx.Step(`^the PeerID of '(\w+)' is '(\w+)'$`, thePeerIDOfIs)
		sctx.Step(`^the IP of '(\w+)' is '([\w:]+)'$`, theIPOfIs)

		sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
			nodes := map[string]*rovynode.Node{}
			return context.WithValue(ctx, nodesCtxKey{}, nodes), nil
		})
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

type nodesCtxKey struct{}

type keyfilesCtxKey struct{}
type keyfile struct {
	peerid     rovy.PeerID
	ip         netip.Addr
	privatekey rovy.PrivateKey
}

func theFollowingTestNodeKeyfiles(ctx context.Context, table *godog.Table) (context.Context, error) {
	if len(table.Rows) < 2 {
		return ctx, fmt.Errorf("expecting keyfile table with header row and at least 1 data row")
	}

	var col struct {
		name int
		pid  int
		ip   int
		priv int
	}
	for i, cell := range table.Rows[0].Cells {
		switch cell.Value {
		case "name":
			col.name = i
		case "peerid":
			col.pid = i
		case "ip":
			col.ip = i
		case "privatekey":
			col.priv = i
		default:
			return ctx, fmt.Errorf("unknown keyfiles table column: %s", cell.Value)
		}
	}

	keyfiles := map[string]keyfile{}
	for _, row := range table.Rows[1:] {
		kfname := row.Cells[col.name].Value
		c, err := cid.Decode(row.Cells[col.pid].Value)
		if err != nil {
			return ctx, err
		}
		pid, err := rovy.PeerIDFromCid(c)
		if err != nil {
			return ctx, err
		}
		ip, err := netip.ParseAddr(row.Cells[col.ip].Value)
		if err != nil {
			return ctx, err
		}
		priv, err := rovy.ParsePrivateKey(row.Cells[col.priv].Value)
		if err != nil {
			return ctx, err
		}
		keyfiles[kfname] = keyfile{pid, ip, priv}
	}

	return context.WithValue(ctx, keyfilesCtxKey{}, keyfiles), nil
}

func nodeFromTestKeyfile(ctx context.Context, name, kfname string) (context.Context, error) {
	keyfiles := ctx.Value(keyfilesCtxKey{}).(map[string]keyfile)

	kf, ok := keyfiles[kfname]
	if !ok {
		return ctx, fmt.Errorf("unknown test keyfile '%s'", kfname)
	}

	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rovynode.Node)
	nodes[name] = rovynode.NewNode(kf.privatekey, log.Default())
	return context.WithValue(ctx, nodesCtxKey{}, nodes), nil
}

func iStartNode(ctx context.Context, name string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rovynode.Node)
	node, ok := nodes[name]
	if !ok {
		return fmt.Errorf("unknown rovy node: %s", name)
	}
	// return node.Start()
	_ = node
	return godog.ErrPending
}

func thePeerIDOfIs(ctx context.Context, name, peerid string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rovynode.Node)
	node, ok := nodes[name]
	if !ok {
		return fmt.Errorf("unknown rovy node: %s", name)
	}

	actual := node.PeerID().String()
	if actual != peerid {
		return fmt.Errorf("expected PeerID '%s', got '%s'", peerid, actual)
	}
	return nil
}

func theIPOfIs(ctx context.Context, name, ip string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rovynode.Node)
	node, ok := nodes[name]
	if !ok {
		return fmt.Errorf("unknown rovy node: %s", name)
	}

	actual := node.IPAddr().String()
	if actual != ip {
		return fmt.Errorf("expected PeerID '%s', got '%s'", ip, actual)
	}
	return nil
}
