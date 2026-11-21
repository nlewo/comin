package cmd

import (
	"testing"

	"github.com/gen2brain/beeep"
	"github.com/stretchr/testify/assert"
)

func TestSetAppName(t *testing.T) {
	old := beeep.AppName
	t.Cleanup(func() { beeep.AppName = old })

	t.Run("custom title", func(t *testing.T) {
		beeep.AppName = "DefaultAppName"
		setAppName("foo")
		assert.Equal(t, "foo", beeep.AppName)
	})

	t.Run("default title", func(t *testing.T) {
		beeep.AppName = "DefaultAppName"
		def := desktopCmd.Flags().Lookup("title").DefValue
		setAppName(def)
		assert.Equal(t, "comin", beeep.AppName)
	})
}
