package deploy

import (
	"fmt"
	"github.com/nlewo/comin/config"
	"github.com/nlewo/comin/repository"
	"github.com/nlewo/comin/nix"
	"github.com/nlewo/comin/state"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/utils"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"time"
)

type Deployer struct {
	repository *repository.Repository
	config     types.Configuration
	dryRun     bool
}

func NewDeployer(dryRun bool, cfg types.Configuration) (Deployer, error) {
	gitConfig := config.MkGitConfig(cfg)
	stateFilepath := filepath.Join(cfg.StateFilepath)

	st, err := state.Load(stateFilepath)
	if err != nil {
		return Deployer{}, err
	}

	repository, err := repository.New(gitConfig, st.RepositoryStatus)
	if err != nil {
		return Deployer{}, fmt.Errorf("Failed to initialize the repository: %s", err)
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

	repositoryStatusInitial := deployer.repository.RepositoryStatus

	err = deployer.repository.Fetch(remoteName)
	if err != nil {
		return
	}
	err = deployer.repository.Update()
	if err != nil {
		return
	}

	if err != nil {
		logrus.Errorf("Error while deploying: %s", err)
		return
	}
	operation := "switch"
	if deployer.repository.RepositoryStatus.IsTesting() {
		operation = "test"
	}

	// We skip the deployment if commit and operation are identical
	if repositoryStatusInitial.SelectedCommitId == deployer.repository.RepositoryStatus.SelectedCommitId && st.LastOperation == operation {
		return nil
	}

	logrus.Debugf("Starting to deploy: repositoryStatusInitial.SelectedCommitId = '%s'; deployer.repository.RepositoryStatus.SelectedCommitId = '%s'; st.LastOperation = '%s'; operation = '%s'", repositoryStatusInitial.SelectedCommitId, deployer.repository.RepositoryStatus.SelectedCommitId, st.LastOperation, operation)

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

	st.LastOperation = operation
	st.RepositoryStatus = deployer.repository.RepositoryStatus
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
