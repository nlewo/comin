package state

import (
	"encoding/json"
	"github.com/nlewo/comin/generation"
	"github.com/nlewo/comin/repository"
	"github.com/nlewo/comin/types"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"sync"
)

type State struct {
	Hostname         string                      `json:"hostname"`
	RepositoryStatus repository.RepositoryStatus `json:"repository_status"`
	Generations      []generation.Generation     `json:"generations"`
}

type StateManager struct {
	state    State
	mu       sync.Mutex
	filepath string
}

func New(config types.Configuration) (*StateManager, error) {
	state, err := Load(config.StateFilepath)
	state.Hostname = config.Hostname
	if err != nil {
		return &StateManager{}, err
	}
	return &StateManager{
		filepath: config.StateFilepath,
		state:    state,
	}, nil
}

func (sm *StateManager) Get() State {
	sm.mu.Lock()
	state := sm.state
	logrus.Debugf("Get the state: %#v", state)
	defer sm.mu.Unlock()
	return state
}

func (sm *StateManager) Set(state State) {
	logrus.Debugf("Set the state: %#v", state)
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state = state
	err := Save(sm.filepath, sm.state)
	if err != nil {
		logrus.Errorf("Could not save the state file: %s", err)
	}
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
	logrus.Debugf("Writing the state to '%s'", stateFilepath)
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
