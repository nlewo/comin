package cmd

import (
	"time"

	"github.com/nlewo/comin/internal/client"
	"github.com/nlewo/comin/internal/protobuf"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func dStarted(status string) *protobuf.Event {
	return &protobuf.Event{Type: &protobuf.Event_DeploymentStartedType{DeploymentStartedType: &protobuf.Event_DeploymentStarted{Deployment: &protobuf.Deployment{
		Uuid:      "b9c9c304-cb7d-4682-a052-57abd427e2b0",
		Operation: "switch",
		Status:    status,
		Generation: &protobuf.Generation{
			Uuid:               "d3b9c304-cb7d-4682-a052-57abd427e2b0",
			SelectedRemoteName: "origin",
			SelectedBranchName: "main",
			SelectedCommitId:   "d3b9c304-cb7d-4682-a052-57abd427e2b0",
			SelectedCommitMsg:  "commit message\nmulti line",
		},
	}}}}
}
func dFinished(status string) *protobuf.Event {
	return &protobuf.Event{Type: &protobuf.Event_DeploymentFinishedType{DeploymentFinishedType: &protobuf.Event_DeploymentFinished{Deployment: &protobuf.Deployment{
		Uuid:      "b9c9c304-cb7d-4682-a052-57abd427e2b0",
		Operation: "switch",
		Status:    status,
		Generation: &protobuf.Generation{
			SelectedRemoteName: "origin",
			SelectedBranchName: "main",
			Uuid:               "d3b9c304-cb7d-4682-a052-57abd427e2b0",
			SelectedCommitId:   "d3b9c304-cb7d-4682-a052-57abd427e2b0",
			SelectedCommitMsg:  "commit message\nmulti line",
		},
	}}}}
}
func bStarted(status string) *protobuf.Event {
	return &protobuf.Event{Type: &protobuf.Event_BuildStartedType{BuildStartedType: &protobuf.Event_BuildStarted{Generation: &protobuf.Generation{
		Uuid:               "b9c9c304-cb7d-4682-a052-57abd427e2b0",
		BuildStatus:        status,
		SelectedRemoteName: "origin",
		SelectedBranchName: "main",
		SelectedCommitId:   "d3b9c304-cb7d-4682-a052-57abd427e2b0",
		SelectedCommitMsg:  "commit message\nmulti line",
	}}}}
}
func bFinished(status string) *protobuf.Event {
	return &protobuf.Event{Type: &protobuf.Event_BuildFinishedType{BuildFinishedType: &protobuf.Event_BuildFinished{Generation: &protobuf.Generation{
		Uuid:               "b9c9c304-cb7d-4682-a052-57abd427e2b0",
		BuildStatus:        status,
		SelectedRemoteName: "origin",
		SelectedBranchName: "main",
		SelectedCommitId:   "d3b9c304-cb7d-4682-a052-57abd427e2b0",
		SelectedCommitMsg:  "commit message\nmulti line",
	}}}}
}

func watchScenario() chan client.Streamer {
	state := &protobuf.State{
		NeedToReboot: wrapperspb.Bool(false),
		Deployer: &protobuf.Deployer{
			IsDeploying: wrapperspb.Bool(false),
			Deployment:  dFinished("done").GetDeploymentFinishedType().Deployment,
		},
	}
	init := &protobuf.Event{Type: &protobuf.Event_ManagerState_{ManagerState: &protobuf.Event_ManagerState{State: state}}}

	events := []client.Streamer{
		{Event: init},
		{FailureMsg: "not connected"},
		{FailureMsg: ""},
		{Event: bStarted("initialized")},
		{Event: bFinished("built")},
		{Event: dStarted("init")},
		{Event: dFinished("done")},
		{Event: dFinished("failed")},
	}
	ch := make(chan client.Streamer)
	go func() {
		for _, e := range events {
			ch <- e
			time.Sleep(1 * time.Second)
		}
	}()
	return ch
}
