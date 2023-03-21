package deploy

import (
	"encoding/json"
	"fmt"
	"github.com/nlewo/comin/config"
	cominGit "github.com/nlewo/comin/git"
	"github.com/nlewo/comin/nix"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Deployer struct {
	repository types.Repository
	config     types.Configuration
	dryRun     bool
}

func NewDeployer(dryRun bool, cfg types.Configuration) (Deployer, error) {
	gitConfig := config.MkGitConfig(cfg)
	repository, err := cominGit.RepositoryOpen(gitConfig)
	if err != nil {
		return Deployer{}, fmt.Errorf("Failed to open the repository: %s", err)
	}
	return Deployer{
		repository: repository,
		config:     cfg,
		dryRun:     dryRun,
	}, nil
}

// Deploy update the tracked repository, deploys the configuration and
// update the state file.
func (deployer Deployer) Deploy() error {
	stateFile := filepath.Join(deployer.config.StateDir, "state.json")
	commitHash, branch, err := cominGit.RepositoryUpdate(deployer.repository)
	if err != nil {
		return err
	}
	logrus.Debugf("Commit is '%s' from branch '%s'", commitHash.String(), branch)
	operation := "switch"
	if branch == deployer.repository.GitConfig.Testing {
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
	err = nix.Deploy(deployer.config, deployer.repository.GitConfig.Path, operation, deployer.dryRun)
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
