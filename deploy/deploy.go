package deploy

import (
	"encoding/json"
	cominGit "github.com/nlewo/comin/git"
	"github.com/nlewo/comin/nix"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Deploy update the tracked repository, deploys the configuration and
// update the state file.
func Deploy(repository types.Repository, config types.Configuration, dryRun bool) error {
	stateFile := filepath.Join(config.StateDir, "state.json")
	commitHash, branch, err := cominGit.RepositoryUpdate(repository)
	if err != nil {
		return err
	}
	operation := "switch"
	if branch == repository.GitConfig.Testing {
		operation = "test"
	}

	var state types.State
	if _, err := os.Stat(stateFile); err == nil {
		logrus.Debugf("Loading state file")
		content, err := ioutil.ReadFile(stateFile)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content, &state)
		if err != nil {
			return err
		}
		logrus.Debugf("State is %#v", state)
	}

	if commitHash.String() == state.CommitId && state.Operation == operation {
		return nil
	}

	logrus.Infof("Starting to deploy commit '%s'", commitHash)
	err = nix.Deploy(config, repository.GitConfig.Path, operation, dryRun)
	if err != nil {
		logrus.Errorf("%s", err)
		logrus.Infof("Deploy failed")
	} else {
		logrus.Infof("Deploy succeeded")
	}
	state.Deployed = err == nil
	state.CommitId = commitHash.String()
	state.Operation = operation

	res, err := json.MarshalIndent(state, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(stateFile, []byte(res), 0644)
	if err != nil {
		return err
	}

	return err
}
