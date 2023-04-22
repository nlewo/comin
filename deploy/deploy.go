package deploy

import (
	"fmt"
	"github.com/nlewo/comin/config"
	cominGit "github.com/nlewo/comin/git"
	"github.com/nlewo/comin/nix"
	"github.com/nlewo/comin/state"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/utils"
	"github.com/sirupsen/logrus"
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
func (deployer Deployer) Deploy() (err error) {
	stateFilepath := filepath.Join(deployer.config.StateDir, "state.json")

	st, err := state.Load(stateFilepath)
	if err != nil {
		return
	}

	commitHash, branch, err := cominGit.RepositoryUpdate(deployer.repository, st.MainCommitId, st.CommitId)
	if err != nil {
		return
	}
	logrus.Debugf("Commit is '%s' from branch '%s'", commitHash.String(), branch)
	operation := "switch"
	if branch == deployer.repository.GitConfig.Testing {
		operation = "test"
		st.IsTesting = true
	} else {
		// When the main branch has been checked out, we
		// update the state to avoid non fast forward future
		// pulls.
		st.MainCommitId = commitHash.String()
	}

	// We skip the deployment if commit and operation are identical
	if commitHash.String() == st.CommitId && st.Operation == operation {
		return nil
	}

	logrus.Infof("Starting to deploy commit '%s'", commitHash)
	cominNeedRestart, err := nix.Deploy(
		deployer.config.Hostname,
		deployer.config.StateDir,
		deployer.repository.GitConfig.Path,
		operation,
		deployer.dryRun,
	)
	if err != nil {
		logrus.Error(err)
		logrus.Infof("Deployment failed")
	} else {
		st.Deployed = true
	}

	st.CommitId = commitHash.String()
	st.Operation = operation
	if err = state.Save(stateFilepath, st); err != nil {
		return
	}

	if cominNeedRestart {
		if err = utils.CominServiceRestart(); err != nil {
			return
		}
	}

	return
}
