package state

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
)

// The state is only used to avoid unnecessary rebuilds and doesn't
// need to be persisted.
type State struct {
	// Operation is the last nixos-rebuild operation
	// (basically, test or switch)
	Operation string
	// The last commit that has been tried to be deployed
	CommitId string
	// If the current deployment is testing
	IsTesting bool
	Deployed  bool
	// The last commit of the Main branch. This is used to
	// garantees the main branch is only fast forwarded.
	MainCommitId string
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
