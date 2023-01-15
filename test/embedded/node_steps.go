package embedded

import (
	"context"
	"fmt"
	"log"
	"strings"

	godog "github.com/cucumber/godog"
	rconfig "go.rovy.net/api/config"
	rnode "go.rovy.net/node"
)

type keyfilesCtxKey struct{}
type nodesCtxKey struct{}

func nodeSteps(sctx *godog.ScenarioContext) {
	sctx.Step(`^a keyfile named '([^']*)'$`, aKeyfileNamed)
	sctx.Step(`^node '([^']*)' from keyfile '([^']*)'$`, nodeFromKeyfile)
	sctx.Step(`^I start node '(\w+)'$`, iStartNode)
	sctx.Step(`^the PeerID of '(\w+)' is '(\w+)'$`, thePeerIDOfIs)
	sctx.Step(`^the IP of '(\w+)' is '([\w:]+)'$`, theIPOfIs)

	sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		keyfiles := map[string]*rconfig.Keyfile{}
		ctx = context.WithValue(ctx, keyfilesCtxKey{}, keyfiles)
		nodes := map[string]*rnode.Node{}
		return context.WithValue(ctx, nodesCtxKey{}, nodes), nil
	})
}

func aKeyfileNamed(ctx context.Context, name string, kfs *godog.DocString) (context.Context, error) {
	kf, err := rconfig.NewKeyfile(strings.NewReader(kfs.Content))
	if err != nil {
		return ctx, err
	}

	keyfiles := ctx.Value(keyfilesCtxKey{}).(map[string]*rconfig.Keyfile)
	keyfiles[name] = kf

	return context.WithValue(ctx, keyfilesCtxKey{}, keyfiles), nil
}

func nodeFromKeyfile(ctx context.Context, name, kfname string) (context.Context, error) {
	keyfiles := ctx.Value(keyfilesCtxKey{}).(map[string]*rconfig.Keyfile)

	kf, ok := keyfiles[kfname]
	if !ok {
		return ctx, fmt.Errorf("unknown keyfile '%s'", kfname)
	}

	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
	nodes[name] = rnode.NewNode(kf.PrivateKey, log.Default())
	return context.WithValue(ctx, nodesCtxKey{}, nodes), nil
}

func iStartNode(ctx context.Context, name string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
	node, ok := nodes[name]
	if !ok {
		return fmt.Errorf("unknown rovy node: %s", name)
	}
	if err := node.Start(); err != nil {
		return err
	}
	if !node.Running() {
		return fmt.Errorf("node isn't running")
	}
	return nil
}

func thePeerIDOfIs(ctx context.Context, name, peerid string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
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
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
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
