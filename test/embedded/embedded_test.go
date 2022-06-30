package embedded_test

import (
	"testing"

	godog "github.com/cucumber/godog"
	// rovy "go.rovy.net"
	// rovyapi "go.rovy.net/api"
	// rovynode "go.rovy.net/node"
)

func TestEmbedded(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^a node (\w+)$`, aNode)
	ctx.Step(`^I start it$`, iStartIt)
	ctx.Step(`^the PeerID is (\w+)$`, thePeerIDIs)
}

func aNode(peerid string) error {
	return godog.ErrPending
}

func iStartIt() error {
	return godog.ErrPending
}

func thePeerIDIs(peerid string) error {
	return godog.ErrPending
}
