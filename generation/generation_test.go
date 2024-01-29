package generation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nlewo/comin/repository"
	"github.com/stretchr/testify/assert"
)

func TestEval(t *testing.T) {
	var evalResult EvalResult
	var ctx context.Context
	evalDone := make(chan struct{})

	nixEvalMock := func(ctx context.Context, repositoryPath string, hostname string) (string, string, string, error) {
		select {
		case <-ctx.Done():
			return "", "", "", fmt.Errorf("timeout exceeded")
		case <-evalDone:
			return "", "", "", nil
		}
	}
	nixBuildMock := func(ctx context.Context, drv string) error {
		return nil
	}

	repositoryPath := "repository/path/"
	hostname := "machine"
	g := New(repository.RepositoryStatus{}, repositoryPath, hostname, "", nixEvalMock, nixBuildMock)
	g.evalTimeout = 1 * time.Second

	// The eval job never terminates so it should timeout
	ctx = context.Background()
	evalResultCh := g.Eval(ctx)
	evalResult = <-evalResultCh
	assert.NotNil(t, evalResult.Err)
	assert.EqualError(t, evalResult.Err, "timeout exceeded")

	ctx = context.Background()
	evalResultCh = g.Eval(ctx)
	// This is to simulate the eval completion
	close(evalDone)
	evalResult = <-evalResultCh
	assert.Nil(t, evalResult.Err)
}
