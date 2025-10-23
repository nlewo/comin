package manager

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestControllerBuildEnable(t *testing.T) {
	c := NewController(true, true)
	var genSubmitted string
	go func() {
		genSubmitted = <-c.Build.submit
	}()
	// We ask for deploy.
	// The generation is marked but not deployed
	c.AskForBuild("gen-1")
	assert.Equal(t, "gen-1", c.Build.state.Needed)
	assert.Never(t, func() bool {
		return genSubmitted != ""
	}, 1*time.Second, 100*time.Millisecond)

	// We confirm the deployment, it is then deployed
	c.Build.Confirm("gen-1")
	assert.Equal(t, "gen-1", c.Build.state.Allowed)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "gen-1", genSubmitted)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestControllerBuildDisable(t *testing.T) {
	c := NewController(false, false)
	var genSubmitted string
	go func() {
		genSubmitted = <-c.Build.submit
	}()
	// We ask for deploy.
	// The generation is deployed
	c.AskForBuild("gen-1")
	assert.Equal(t, "gen-1", c.Build.state.Allowed)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "gen-1", genSubmitted)
	}, 1*time.Second, 100*time.Millisecond)
}
