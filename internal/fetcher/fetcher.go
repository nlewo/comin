package fetcher

import (
	"context"

	"github.com/nlewo/comin/pkg/protobuf"
)

type Fetcher interface {
	Start(ctx context.Context)
	IsFetching() bool
	TriggerFetch(remotes []string)
	GetState() *protobuf.Fetcher
}
