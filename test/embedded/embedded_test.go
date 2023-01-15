package embedded

import (
	"testing"

	godog "github.com/cucumber/godog"
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
		nodeSteps(sctx)
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
