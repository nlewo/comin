package executor

import (
	"context"
	"errors"

	"github.com/nlewo/comin/internal/types"
)

type Executor interface {
	Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error)
	Build(ctx context.Context, drvPath string) (err error)
	Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error)
}

func New(config types.ExecutorConfig) (e Executor, err error) {
	if config.Type == "garnix" {
		e, err = NewGarnixExecutor(config.GarnixConfig)
	} else if config.Type == "nix" {
		e, err = NewNixExecutor()
	} else if config.Type == "" {
		e, err = NewNixExecutor()
	} else {
		err = errors.New("Unknown executor type in config, must be one of {garnix, nix}")
	}
	return
}
