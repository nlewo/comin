package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The test binary is also the comin that startDarwinDeploy runs.
func TestMain(m *testing.M) {
	if len(os.Args) > 4 && os.Args[1] == DarwinDeployCommand {
		darwinDeploy = func(ctx context.Context, outPath, operation string, profilePaths []string, stdout, stderr io.WriteCloser) (bool, string, error) {
			return true, "profile-path", switchToConfigurationDarwin(operation, outPath, false, stdout, stderr)
		}
		if err := RunDarwinDeploy(os.Args[2], os.Args[3], os.Args[4], os.Args[5:]); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func darwinSystem(t *testing.T, activate string) string {
	outPath := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(outPath, "activate-user"), []byte("#!/bin/sh\n"), 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(outPath, "activate"), []byte("#!/bin/sh\n"+activate+"\n"), 0755))
	return outPath
}

func TestDarwinDeploy(t *testing.T) {
	pgidFile := filepath.Join(t.TempDir(), "pgid")
	tests := []struct {
		name          string
		activate      string
		expectedError string
	}{
		{
			name:     "the deployment succeeds",
			activate: fmt.Sprintf("ps -o pgid= -p $$ > %s; echo before; sleep 1; echo after; echo err >&2", pgidFile),
		},
		{
			name:          "the deployment fails",
			activate:      "echo before; sleep 1; echo after; echo err >&2; exit 3",
			expectedError: "/activate fails with exit status 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := GitNixFlake{systemAttr: "darwinConfigurations", darwinDeployDir: filepath.Join(t.TempDir(), "darwin-deploy")}
			var stdout, stderr bytes.Buffer
			needToRestartComin, profilePath, err := executor.Deploy(t.Context(), darwinSystem(t, tt.activate), "test", nil, &NopWriteCloser{&stdout}, &NopWriteCloser{&stderr})

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedError)
			}
			assert.True(t, needToRestartComin)
			assert.Equal(t, "profile-path", profilePath)
			assert.Equal(t, "before\nafter\n", stdout.String())
			assert.Equal(t, "err\n", stderr.String())
			assert.NoDirExists(t, executor.darwinDeployDir)
		})
	}

	pgid, err := os.ReadFile(pgidFile)
	assert.NoError(t, err)
	assert.NotEqual(t, fmt.Sprint(syscall.Getpgrp()), strings.TrimSpace(string(pgid)))
}

func TestDarwinDeployResumed(t *testing.T) {
	executor := GitNixFlake{systemAttr: "darwinConfigurations", darwinDeployDir: filepath.Join(t.TempDir(), "darwin-deploy")}
	_, found := executor.InterruptedDeploy()
	assert.False(t, found)

	// Once started, the deployment is on its own, as if comin had been killed.
	outPath := darwinSystem(t, "echo before; sleep 1; echo after")
	assert.NoError(t, startDarwinDeploy(executor.darwinDeployDir, outPath, "test", nil))
	interruptedOutPath, found := executor.InterruptedDeploy()
	assert.True(t, found)
	assert.Equal(t, outPath, interruptedOutPath)

	var stdout, stderr bytes.Buffer
	profilePath, err := executor.ResumeDeploy(&stdout, &stderr)

	assert.NoError(t, err)
	assert.Equal(t, "profile-path", profilePath)
	assert.Equal(t, "before\nafter\n", stdout.String())
	assert.NoDirExists(t, executor.darwinDeployDir)
}

func TestDarwinDeployWithoutResult(t *testing.T) {
	dir := t.TempDir()
	for _, name := range darwinDeployFiles {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("before\n"), 0600))
	}

	var stdout, stderr bytes.Buffer
	_, _, err := waitDarwinDeploy(dir, &stdout, &stderr)

	assert.EqualError(t, err, "the deployment has been interrupted")
	assert.Equal(t, "before\n", stdout.String())
	assert.NoDirExists(t, dir)
}
