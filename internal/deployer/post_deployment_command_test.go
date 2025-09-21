package deployer

import (
	"testing"
	"time"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/store"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestBasic(t *testing.T) {

	startedAt := time.Now()
	endedAt := startedAt.Add(10 * time.Second)
	deployment := &protobuf.Deployment{
		Uuid:         "uuid",
		Generation:   &protobuf.Generation{},
		StartedAt:    timestamppb.New(startedAt),
		EndedAt:      timestamppb.New(endedAt),
		ErrorMsg:     "",
		RestartComin: wrapperspb.Bool(false),
		ProfilePath:  "",
		Status:       store.StatusToString(store.Done),
		Operation:    "",
	}

	out, err := runPostDeploymentCommand("env", deployment)
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_GIT_SHA=")
}
