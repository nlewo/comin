package executor

import (
	"context"
)

type Executor interface {
	Eval(ctx context.Context, flakeUrl, hostname string) (drvPath string, outPath string, machineId string, err error)
	Build(ctx context.Context, drvPath string) (err error)
	Deploy(ctx context.Context, outPath, operation string) (needToRestartComin bool, profilePath string, err error)
}

func New(configurationAttr string) (e Executor, err error) {
	e, err = NewNixExecutor(configurationAttr)
	return
}
