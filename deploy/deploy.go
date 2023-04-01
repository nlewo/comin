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

func loadState(stateFilepath string) (state types.State, err error) {
	if _, err := os.Stat(stateFilepath); err == nil {
		logrus.Debugf("Loading state file located at %s", stateFilepath)
		content, err := ioutil.ReadFile(stateFilepath)
		if err != nil {
			return state, err
		}
		err = json.Unmarshal(content, &state)
		if err != nil {
			return state, err
		}
		logrus.Debugf("State is %#v", state)
	}
	return state, nil
}

func saveState(stateFilepath string, state types.State) error {
	res, err := json.MarshalIndent(state, "", "\t")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(stateFilepath, []byte(res), 0644)
	if err != nil {
		return err
	}
	return nil
}

// Deploy update the tracked repository, deploys the configuration and
// update the state file.
func (deployer Deployer) Deploy() error {
	stateFilepath := filepath.Join(deployer.config.StateDir, "state.json")
	commitHash, branch, err := cominGit.RepositoryUpdate(deployer.repository)
	if err != nil {
		return err
	}
	logrus.Debugf("Commit is '%s' from branch '%s'", commitHash.String(), branch)
	operation := "switch"
	if branch == deployer.repository.GitConfig.Testing {
		operation = "test"
	}

	state, err := loadState(stateFilepath)
	if err != nil {
		return err
	}

	// We skip the deployment if commit and operation are identical
	if commitHash.String() == state.CommitId && state.Operation == operation {
		return nil
	}

	logrus.Infof("Starting to deploy commit '%s'", commitHash)
	err = nix.Deploy(deployer.config, deployer.repository.GitConfig.Path, operation, deployer.dryRun)
	if err != nil {
		logrus.Errorf("%s", err)
		logrus.Infof("Deployment failed")
	} else {
		logrus.Infof("Deployment succeeded")
	}

	state.Deployed = err == nil
	state.CommitId = commitHash.String()
	state.Operation = operation
	if err := saveState(stateFilepath, state); err != nil {
		return err
	}

	return err
}
