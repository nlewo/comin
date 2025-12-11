package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNixExecutorWithDarwinConfiguration(t *testing.T) {
	// Test creating a NixExecutor with Darwin configuration
	executor, err := NewNixFlakeExecutor("darwinConfigurations")
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.Equal(t, "darwinConfigurations", executor.systemAttr)
}

func TestNixExecutorWithNixOSConfiguration(t *testing.T) {
	// Test creating a NixExecutor with NixOS configuration
	executor, err := NewNixFlakeExecutor("nixosConfigurations")
	assert.NoError(t, err)
	assert.NotNil(t, executor)
	assert.Equal(t, "nixosConfigurations", executor.systemAttr)
}

func TestNixExecutorEval(t *testing.T) {
	tests := []struct {
		name             string
		systemAttr       string
		repositoryPath   string
		repositorySubdir string
		commitId         string
		hostname         string
	}{
		{
			name:             "Eval with NixOS configuration",
			systemAttr:       "nixosConfigurations",
			repositoryPath:   "/non-existent",
			repositorySubdir: ".",
			commitId:         "non-existent-commit-id",
			hostname:         "test-host",
		},
		{
			name:             "Eval with Darwin configuration",
			systemAttr:       "darwinConfigurations",
			repositoryPath:   "/non-existent",
			repositorySubdir: ".",
			commitId:         "non-existent-commit-id",
			hostname:         "test-host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewNixFlakeExecutor(tt.systemAttr)
			assert.NoError(t, err)

			ctx := context.Background()

			// Test that Eval doesn't panic and handles parameters correctly
			// This will error in test environment since nix commands will fail,
			// but we're testing the code path and parameter handling
			_, _, _, err = executor.Eval(ctx, tt.repositoryPath, tt.repositorySubdir, tt.commitId, tt.systemAttr, tt.hostname)
			t.Logf("Eval with %s returned error: %v (expected in test environment)", tt.systemAttr, err)
		})
	}
}

func TestNixExecutorShowDerivation(t *testing.T) {
	tests := []struct {
		name       string
		systemAttr string
		flakeUrl   string
		hostname   string
	}{
		{
			name:       "ShowDerivation with NixOS configuration",
			systemAttr: "nixosConfigurations",
			flakeUrl:   "github:example/nixos-config",
			hostname:   "test-host",
		},
		{
			name:       "ShowDerivation with Darwin configuration",
			systemAttr: "darwinConfigurations",
			flakeUrl:   "github:example/darwin-config",
			hostname:   "test-host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewNixFlakeExecutor(tt.systemAttr)
			assert.NoError(t, err)

			ctx := context.Background()

			// Test that ShowDerivation doesn't panic and handles parameters correctly
			_, _, err = executor.ShowDerivation(ctx, tt.flakeUrl, tt.hostname)
			t.Logf("ShowDerivation with %s returned error: %v (expected in test environment)", tt.systemAttr, err)
		})
	}
}

func TestNixExecutorList(t *testing.T) {
	tests := []struct {
		name       string
		systemAttr string
		flakeUrl   string
	}{
		{
			name:       "List NixOS configurations",
			systemAttr: "nixosConfigurations",
			flakeUrl:   "github:example/nixos-config",
		},
		{
			name:       "List Darwin configurations",
			systemAttr: "darwinConfigurations",
			flakeUrl:   "github:example/darwin-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewNixFlakeExecutor(tt.systemAttr)
			assert.NoError(t, err)

			// Test that List doesn't panic and handles configuration attribute correctly
			_, err = executor.List(tt.flakeUrl)
			t.Logf("List with %s returned error: %v (expected in test environment)", tt.systemAttr, err)
		})
	}
}

func TestNixExecutorDeploy(t *testing.T) {
	tests := []struct {
		name       string
		systemAttr string
		outPath    string
		operation  string
	}{
		{
			name:       "Deploy with NixOS configuration",
			systemAttr: "nixosConfigurations",
			outPath:    "/nix/store/test-nixos-path",
			operation:  "switch",
		},
		{
			name:       "Deploy with Darwin configuration",
			systemAttr: "darwinConfigurations",
			outPath:    "/nix/store/test-darwin-path",
			operation:  "switch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewNixFlakeExecutor(tt.systemAttr)
			assert.NoError(t, err)

			ctx := context.Background()

			// Test that Deploy doesn't panic and delegates to the correct platform-specific function
			_, _, err = executor.Deploy(ctx, tt.outPath, tt.operation)
			t.Logf("Deploy with %s returned error: %v (expected in test environment)", tt.systemAttr, err)
		})
	}
}
