package embedded

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	godog "github.com/cucumber/godog"
	rconfig "go.rovy.net/api/config"
	rnode "go.rovy.net/node"
)

type keyfilesCtxKey struct{}
type nodesCtxKey struct{}
type responseCtxKey struct{}

func nodeSteps(sctx *godog.ScenarioContext) {
	sctx.Step(`^a keyfile named '(\w+\.toml)'$`, aKeyfileNamed)
	sctx.Step(`^node '([^']*)' from keyfile '(\w+\.toml)'$`, nodeFromKeyfile)
	sctx.Step(`^I start node '(\w+)'$`, iStartNode)
	sctx.Step(`^I stop node '(\w+)'$`, iStopNode)
	sctx.Step(`^node '(\w+)' (is|is not) running$`, nodeIsRunning)
	sctx.Step(`^a '(\w+)' call on '(\w+)' is successful$`, aCallIsSuccessful)
	sctx.Step(`^response value '([\w.]+)' is '([\w:]+)'$`, responseValueIsString)
	sctx.Step(`^response value '([\w.]+)' is (true|false)$`, responseValueIsBool)

	sctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		keyfiles := map[string]*rconfig.Keyfile{}
		ctx = context.WithValue(ctx, keyfilesCtxKey{}, keyfiles)
		nodes := map[string]*rnode.Node{}
		return context.WithValue(ctx, nodesCtxKey{}, nodes), nil
	})
	sctx.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		// TODO: shut down lingering nodes
		return ctx, nil
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

	return node.Start()
}

func iStopNode(ctx context.Context, name string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
	node, ok := nodes[name]
	if !ok {
		return fmt.Errorf("unknown rovy node: %s", name)
	}
	return node.Stop()
}

func nodeIsRunning(ctx context.Context, name string, not string) error {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
	node, ok := nodes[name]
	if !ok {
		return fmt.Errorf("unknown rovy node: %s", name)
	}

	info, _ := node.Info()

	if not == "is" {
		if true != info.Running {
			return fmt.Errorf("expected Running to be true, got false")
		}
	} else if not == "is not" {
		if false != info.Running {
			return fmt.Errorf("expected Running to be false, got true")
		}
	} else {
		return fmt.Errorf("must be either 'is' or 'is not'")
	}

	if node.PeerID() != info.PeerID {
		return fmt.Errorf("expected PeerID '%s', got '%s'", node.PeerID(), info.PeerID)
	}

	return nil
}

// TODO: make this work for methods outside of NodeAPI
func aCallIsSuccessful(ctx context.Context, cmd, name string) (context.Context, error) {
	nodes := ctx.Value(nodesCtxKey{}).(map[string]*rnode.Node)
	node, ok := nodes[name]
	if !ok {
		return ctx, fmt.Errorf("unknown rovy node: %s", name)
	}

	v := reflect.ValueOf(node)
	m := v.MethodByName(strings.Title(cmd))
	if m == (reflect.Value{}) {
		return ctx, fmt.Errorf("unknown api method: %s", cmd)
	}

	res := m.Call([]reflect.Value{})
	if !res[len(res)-1].IsZero() {
		err := res[len(res)-1].Interface().(error)
		return ctx, fmt.Errorf("api call %s failed: %s", cmd, err)
	}

	return context.WithValue(ctx, responseCtxKey{}, res), nil
}

func responseValueIsBool(ctx context.Context, key string, trueOrFalse string) error {
	res := ctx.Value(responseCtxKey{}).([]reflect.Value)

	v := res[0].FieldByName(key)
	if v == (reflect.Value{}) {
		return fmt.Errorf("unknown response field: %s", key)
	}

	if v.Kind() != reflect.Bool {
		return fmt.Errorf("response field %s isn't boolean", key)
	}

	expected, _ := strconv.ParseBool(trueOrFalse)
	actual := v.Bool()
	if actual != expected {
		return fmt.Errorf("mismatch: %s should be %t, is %t", key, expected, actual)
	}
	return nil
}

func responseValueIsString(ctx context.Context, key, val string) error {
	res := ctx.Value(responseCtxKey{}).([]reflect.Value)

	v := res[0].FieldByName(key)
	if v == (reflect.Value{}) {
		return fmt.Errorf("unknown response field: %s", key)
	}

	var actual string
	if v.Kind() == reflect.String {
		actual = v.Interface().(string)
	} else {
		m := v.MethodByName("String")
		if m == (reflect.Value{}) {
			return fmt.Errorf("response field %s has no String method", key)
		}
		actual = m.Call([]reflect.Value{})[0].Interface().(string)
	}

	if actual != val {
		return fmt.Errorf("mismatch: %s should be '%s', is '%s'", key, val, actual)
	}
	return nil
}
