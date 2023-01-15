package deploy

import (
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/nix"
	"encoding/json"
	"github.com/sirupsen/logrus"
	cominGit "github.com/nlewo/comin/git"
	"os"
	"io/ioutil"
)

func Deploy(repository types.Repository, config types.Config) error {
	commitHash, branch, err := cominGit.RepositoryUpdate(repository)
	if err != nil {
		return err
	}
	operation := "switch"
	if branch == repository.GitConfig.Testing {
		operation = "test"
	}

	var state types.State
        if _, err := os.Stat(config.StateFile); err == nil {
		logrus.Debugf("Loading state file")
		content, err := ioutil.ReadFile(config.StateFile)
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
	err = nix.Deploy(config, repository.GitConfig.Path, operation)
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
	err = ioutil.WriteFile(config.StateFile, []byte(res), 0644)
	if err != nil {
		return err
	}

	return err
}
