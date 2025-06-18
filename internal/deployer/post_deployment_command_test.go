package deployer

import (
	"testing"
	"time"

	"github.com/nlewo/comin/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {

	startedAt := time.Now()
	endedAt := startedAt.Add(10 * time.Second)
	deployment := store.Deployment{
		UUID: "uuid",
		// Generation builder.Generation
		StartedAt:    startedAt,
		EndedAt:      endedAt,
		Err:          nil,
		ErrorMsg:     "",
		RestartComin: false,
		ProfilePath:  "",
		Status:       store.Done,
		Operation:    "",
	}

	out, err := runPostDeploymentCommand("env", &deployment)
	assert.NoError(t, err)
	assert.Contains(t, out, "COMIN_GIT_SHA=")
}
