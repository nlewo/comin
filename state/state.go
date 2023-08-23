package state

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"time"
	"github.com/nlewo/comin/repository"
)

// The state is used for security purposes and to avoid unnecessary
// rebuilds.
type State struct {
	// Operation is the last nixos-rebuild operation
	// (basically, test or switch)
	LastOperation string `json:"last_operation"`
	HeadCommitDeployed   bool      `json:"head_commit_deployed"`
	HeadCommitDeployedAt time.Time `json:"head_commit_deployed_at"`
	RepositoryStatus repository.RepositoryStatus `json:"repository_status"`
}

func Load(stateFilepath string) (state State, err error) {
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

func Save(stateFilepath string, state State) error {
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
