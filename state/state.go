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

type StateManager struct {
	state State
	filepath string
	chGet chan State
	chSet chan State
}

func New(stateFilepath string) (StateManager, error) {
	chSet := make(chan State, 1)
	chGet := make(chan State, 1)
	state, err := Load(stateFilepath)
	if err != nil {
		return StateManager{}, err
	}

	return StateManager{
		chGet: chGet,
		chSet: chSet,
		filepath: stateFilepath,
		state: state,
	}, nil
}

func (sm StateManager) Start() {
	manager := func () {
		for {
			select {
			case state := <- sm.chSet:
				sm.state = state
				err := Save(sm.filepath, sm.state)
				if err != nil {
					logrus.Errorf("Could not save the state file: %s", err)
				}
				
			case sm.chGet <- sm.state:
				continue
			}
		}
	}
	logrus.Infof("Starting the state manager")
	go manager()
}

func (sm StateManager) Get() State {
	state := <- sm.chGet
	return state
}

func (sm StateManager) Set(State) {
	sm.chSet <- sm.state
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
