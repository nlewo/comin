package builder

import (
	"testing"

	"github.com/nlewo/comin/internal/store"
	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestPostBuildCommand(t *testing.T) {
	generation := &pb.Generation{
		Uuid: "uuid",
	}

	out, err := runPostBuildCommand("env", generation, store.Built.String(), "")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_PHASE=build")
	assert.Contains(t, out, "COMIN_GIT_SHA=")
	assert.Contains(t, out, "COMIN_STATUS=built")
}
