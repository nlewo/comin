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
	"time"
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
// If remoteName is "", all remotes are fetched.
func (deployer Deployer) Deploy(remoteName string) (err error) {
	stateFilepath := filepath.Join(deployer.config.StateFilepath)

	st, err := state.Load(stateFilepath)
	if err != nil {
		return
	}

	commitHash, remote, branch, err := cominGit.RepositoryUpdate(deployer.repository, remoteName, st.MainCommitId, st.HeadCommitId)
	if err != nil {
		return
	}
	logrus.Debugf("Commit is '%s' from '%s/%s'", commitHash.String(), remote, branch)
	operation := "switch"
	if cominGit.IsTesting(deployer.repository, remote, branch) {
		operation = "test"
		st.OnTesting = true
	} else {
		// When the main branch has been checked out, we
		// update the state to avoid non fast forward future
		// pulls.
		st.MainCommitId = commitHash.String()
		st.OnTesting = false
	}

	// We skip the deployment if commit and operation are identical
	if commitHash.String() == st.HeadCommitId && st.LastOperation == operation {
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
		st.HeadCommitDeployed = false
	} else {
		st.HeadCommitDeployed = true
		st.HeadCommitDeployedAt = time.Now()
	}

	st.HeadCommitId = commitHash.String()
	st.LastOperation = operation
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
