package executor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// DarwinDeployCommand is the comin subcommand that deploys in a process that survives comin.
const DarwinDeployCommand = "darwin-deploy"

var darwinDeployFiles = []string{"stdout", "stderr", "lock"}

var darwinDeploy = deployDarwin

type darwinDeployResult struct {
	NeedToRestartComin bool   `json:"need_to_restart_comin"`
	ProfilePath        string `json:"profile_path"`
	Error              string `json:"error"`
}

// RunDarwinDeploy deploys outPath and records the output and the result in dir.
func RunDarwinDeploy(dir, outPath, operation string, profilePaths []string) error {
	// Only this process holds the lock it inherits, not the activation it starts.
	syscall.CloseOnExec(3)
	// Writing logs to a comin that is gone must not stop the deployment.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
	var outputs [2]*os.File
	for i, name := range darwinDeployFiles[:2] {
		output, err := os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer output.Close() // nolint: errcheck
		outputs[i] = output
	}

	var result darwinDeployResult
	var err error
	result.NeedToRestartComin, result.ProfilePath, err = darwinDeploy(context.Background(), outPath, operation, profilePaths, outputs[0], outputs[1])
	if err != nil {
		result.Error = err.Error()
	}
	content, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "result"), content, 0600)
}

// A deployment can make launchd reload comin, which kills comin, its process group and the readers of its pipes.
func startDarwinDeploy(dir, outPath, operation string, profilePaths []string) (err error) {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	var files [3]*os.File
	for i, name := range darwinDeployFiles {
		file, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		defer file.Close() // nolint: errcheck
		files[i] = file
	}
	if err := syscall.Flock(int(files[2].Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	comin, err := os.Executable()
	if err != nil {
		return err
	}
	// The out path is written last since it is what marks a deployment to wait for.
	if err := os.WriteFile(filepath.Join(dir, "out-path"), []byte(outPath), 0600); err != nil {
		return err
	}
	cmd := exec.Command(comin, append([]string{DarwinDeployCommand, dir, outPath, operation}, profilePaths...)...)
	cmd.Stdout, cmd.Stderr, cmd.ExtraFiles = os.Stdout, os.Stderr, files[2:]
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // nolint: errcheck
	return nil
}

// waitDarwinDeploy also waits for a deployment that a previous comin process started.
func waitDarwinDeploy(dir string, stdout, stderr io.Writer) (needToRestartComin bool, profilePath string, err error) {
	defer os.RemoveAll(dir) // nolint: errcheck
	var files [3]*os.File
	for i, name := range darwinDeployFiles {
		file, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return false, "", err
		}
		defer file.Close() // nolint: errcheck
		files[i] = file
	}
	unlocked := make(chan struct{})
	go func() {
		for errors.Is(syscall.Flock(int(files[2].Fd()), syscall.LOCK_SH), syscall.EINTR) {
		}
		close(unlocked)
	}()
	for ended := false; !ended; {
		select {
		case <-unlocked:
			ended = true
		case <-time.After(100 * time.Millisecond):
		}
		_, _ = io.Copy(stdout, files[0])
		_, _ = io.Copy(stderr, files[1])
	}

	var result darwinDeployResult
	content, _ := os.ReadFile(filepath.Join(dir, "result"))
	if json.Unmarshal(content, &result) != nil {
		return false, "", errors.New("the deployment has been interrupted")
	}
	if result.Error != "" {
		err = errors.New(result.Error)
	}
	return result.NeedToRestartComin, result.ProfilePath, err
}
