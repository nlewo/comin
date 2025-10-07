package manager

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestControllerDeployEnable(t *testing.T) {
	c := NewController(true, true)
	var genSubmitted string
	go func() {
		genSubmitted = <-c.submitGenerationForDeploy
	}()
	// We ask for deploy.
	// The generation is marked but not deployed
	c.AskForDeploy("gen-1")
	assert.Equal(t, "gen-1", c.state.GenerationNeedConfirmationForDeploy)
	assert.Never(t, func() bool {
		return genSubmitted != ""
	}, 1*time.Second, 100*time.Millisecond)

	// We confirm the deployment, it is then deployed
	c.confirmForDeploy("gen-1")
	assert.Equal(t, "gen-1", c.state.GenerationNeedConfirmationForDeploy)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "gen-1", genSubmitted)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestControllerDeployDisable(t *testing.T) {
	c := NewController(false, false)
	var genSubmitted string
	go func() {
		genSubmitted = <-c.submitGenerationForDeploy
	}()
	// We ask for deploy.
	// The generation is deployed
	c.AskForDeploy("gen-1")
	assert.Equal(t, "gen-1", c.state.GenerationAllowedForDeploy)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "gen-1", genSubmitted)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestControllerBuildEnable(t *testing.T) {
	c := NewController(true, true)
	var genSubmitted string
	go func() {
		genSubmitted = <-c.submitGenerationForBuild
	}()
	// We ask for deploy.
	// The generation is marked but not deployed
	c.AskForBuild("gen-1")
	assert.Equal(t, "gen-1", c.state.GenerationNeedConfirmationForBuild)
	assert.Never(t, func() bool {
		return genSubmitted != ""
	}, 1*time.Second, 100*time.Millisecond)

	// We confirm the deployment, it is then deployed
	c.confirmForBuild("gen-1")
	assert.Equal(t, "gen-1", c.state.GenerationNeedConfirmationForBuild)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "gen-1", genSubmitted)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestControllerBuildDisable(t *testing.T) {
	c := NewController(false, false)
	var genSubmitted string
	go func() {
		genSubmitted = <-c.submitGenerationForBuild
	}()
	// We ask for deploy.
	// The generation is deployed
	c.AskForBuild("gen-1")
	assert.Equal(t, "gen-1", c.state.GenerationAllowedForBuild)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "gen-1", genSubmitted)
	}, 1*time.Second, 100*time.Millisecond)
}
