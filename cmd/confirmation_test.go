package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAcceptTarget(t *testing.T) {
	cases := []struct {
		name, build, deploy, forWhat, uuid, scope string
	}{
		{"default prefers the build and confirms both", "b", "d", "", "b", "all"},
		{"default falls back to the deploy", "", "d", "", "d", "deploy"},
		{"default with nothing waiting", "", "", "", "", "deploy"},
		{"build only", "b", "d", "build", "b", "build"},
		{"build only with nothing to build", "", "d", "build", "", "build"},
		{"deploy only skips a waiting build", "b", "d", "deploy", "d", "deploy"},
		{"deploy only with nothing to deploy", "b", "", "deploy", "", "deploy"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uuid, scope := acceptTarget(c.build, c.deploy, c.forWhat)
			assert.Equal(t, c.uuid, uuid)
			assert.Equal(t, c.scope, scope)
		})
	}
}
