package utils

import (
	"testing"

	pb "github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestRunPostCommand(t *testing.T) {
	g := &pb.Generation{Uuid: "uuid"}

	out, err := RunPostCommand("env", "build", g, "built", "")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_PHASE=build")
	assert.Contains(t, out, "COMIN_GIT_SHA=")
	assert.Contains(t, out, "COMIN_GENERATION=uuid")
	assert.Contains(t, out, "COMIN_STATUS=built")

	out, err = RunPostCommand("env", "deploy", g, "done", "")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_PHASE=deploy")
	assert.Contains(t, out, "COMIN_STATUS=done")
}

func TestRunPostCommandExtras(t *testing.T) {
	g := &pb.Generation{Uuid: "uuid"}

	out, err := RunPostCommand("env", "build", g, "built", "", "COMIN_DRV_PATH=/nix/store/abc")
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_DRV_PATH=/nix/store/abc")
}
