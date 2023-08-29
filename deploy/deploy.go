package deploy

import (
	"fmt"
	"github.com/nlewo/comin/config"
	"github.com/nlewo/comin/generation"
	"github.com/nlewo/comin/nix"
	"github.com/nlewo/comin/repository"
	"github.com/nlewo/comin/state"
	"github.com/nlewo/comin/types"
	"github.com/nlewo/comin/utils"
	"github.com/sirupsen/logrus"
	"time"
)

type Deployer struct {
	repository   *repository.Repository
	config       types.Configuration
	dryRun       bool
	stateManager *state.StateManager
	generations  *generation.Generations
}

func NewDeployer(dryRun bool, cfg types.Configuration, stateManager *state.StateManager) (Deployer, error) {
	gitConfig := config.MkGitConfig(cfg)

	state := stateManager.Get()
	repositoryStatus := repository.RepositoryStatus{}
	if len(state.Generations) > 0 {
		repositoryStatus = state.Generations[0].RepositoryStatus
	}
	repository, err := repository.New(gitConfig, repositoryStatus)
	if err != nil {
		return Deployer{}, fmt.Errorf("Failed to initialize the repository: %s", err)
	}
	// FIXME: the generation limit number should come from the configuration
	generations := generation.NewGenerations(100, state.Generations)

	return Deployer{
		repository:   repository,
		config:       cfg,
		dryRun:       dryRun,
		stateManager: stateManager,
		generations:  generations,
	}, nil
}

// Deploy update the tracked repository, deploys the configuration and
// update the state file.
// If remoteName is "", all remotes are fetched.
func (deployer Deployer) Deploy(remoteName string) (err error) {
	st := deployer.stateManager.Get()

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
	if len(st.Generations) > 0 && st.Generations[0].RepositoryStatus.SelectedCommitId == deployer.repository.RepositoryStatus.SelectedCommitId && st.Generations[0].SwitchOperation == operation {
		return nil
	}

	gen := generation.Generation{
		SwitchOperation:     operation,
		Status:              "running",
		RepositoryStatus:    deployer.repository.RepositoryStatus,
		DeploymentStartedAt: time.Now(),
	}
	deployer.generations.InsertNewGeneration(gen)
	st.Generations = deployer.generations.Generations
	deployer.stateManager.Set(st)

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
		gen.Status = "failed"
	} else {
		gen.Status = "succeeded"
	}

	gen.DeploymentEndedAt = time.Now()
	deployer.generations.ReplaceGenerationAt(0, gen)
	st.Generations = deployer.generations.Generations
	deployer.stateManager.Set(st)

	if cominNeedRestart {
		if err = utils.CominServiceRestart(); err != nil {
			return
		}
	}

	return
}
