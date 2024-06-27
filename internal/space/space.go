package space

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nlewo/comin/internal/deployment"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

type Space struct {
	url            string
	apiKey         string
	agentName      string
	agentMachineID string
	agentID        *uuid.UUID
	client         *ClientWithResponses
}

func New(agentName, agentMachineID string) Space {
	apiKeyFilepath := os.Getenv("COMIN_SPACE_API_KEY_FILEPATH")
	apiKey, err := os.ReadFile(apiKeyFilepath)
	if err != nil {
		logrus.Errorf("space: could not read the password in the file %s: %s", apiKeyFilepath, err)
	}
	url := os.Getenv("COMIN_SPACE_URL")
	return Space{
		url:            url,
		apiKey:         string(apiKey),
		agentName:      agentName,
		agentMachineID: agentMachineID,
	}
}

func (s *Space) agentRegister(name, machineID string) error {
	if s.client == nil {
		hc := http.Client{}
		reqEditor := func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "token "+s.apiKey)
			return nil
		}
		c, err := NewClientWithResponses(s.url+"/api", WithHTTPClient(&hc), WithRequestEditorFn(reqEditor))
		if err != nil {
			return fmt.Errorf("space: failed to NewClientWithResponses: %s", err)
		}
		s.client = c
	}
	if s.agentID != nil {
		return nil
	}
	logrus.Infof("space: registering the agent with name %s and machineID %s", s.agentName, s.agentMachineID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.client.GetAgentsWithResponse(ctx, &GetAgentsParams{MachineID: &s.agentMachineID})
	if err != nil {
		return fmt.Errorf("space: failed to GetAgentsWithResponse: %s", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("space: expected HTTP 200 but received %d", resp.StatusCode())
	}
	agents := *resp.JSON200
	if len(agents) == 1 {
		s.agentID = &agents[0].Id
	} else {
		newAgent := NewAgent{
			Name:      name,
			MachineID: machineID,
		}
		resp, err := s.client.PostAgentsWithResponse(ctx, newAgent)
		if err != nil {
			return fmt.Errorf("space: failed to PostAgentsWithResponse: %s", err)
		}
		if resp.StatusCode() != http.StatusCreated {
			return fmt.Errorf("space: expected HTTP 201 but received %d", resp.StatusCode())
		}
		s.agentID = &resp.JSON201.Id
	}
	return nil
}

func (s *Space) deploymentRegister(ctx context.Context, d deployment.Deployment) error {
	resp, err := s.client.GetDeploymentById(ctx, d.UUID)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	} else if resp.StatusCode == http.StatusNotFound {
		uuidStr, _ := uuid.Parse(d.UUID)
		body := PostDeploymentsJSONRequestBody{
			Id:            uuidStr,
			AgentID:       *s.agentID,
			CommitID:      d.Generation.SelectedCommitId,
			CommitMessage: d.Generation.SelectedCommitMsg,
			RemoteName:    d.Generation.SelectedRemoteName,
			RemoteUrl:     d.Generation.SelectedRemoteUrl,
			BranchName:    d.Generation.SelectedBranchName,
			Operation:     d.Operation,
			Status:        deployment.StatusToString(d.Status),
			EndedAt:       d.EndAt,
		}
		_, err = s.client.PostDeploymentsWithResponse(ctx, body)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Space) AgentNeedToReboot(needToReboot bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	patchAgent := AgentPatchRequest{
		NeedToReboot: &needToReboot,
	}
	logrus.Debugf("space: PatchAgentWithResponse(%s, %#v)", s.agentID.String(), patchAgent)
	respPatch, err := s.client.PatchAgentWithResponse(ctx, *s.agentID, patchAgent)
	if err != nil {
		logrus.Error(err)
		return
	}
	if respPatch.StatusCode() != http.StatusOK {
		logrus.Errorf("space: expected HTTP 200 while patching the agent but received %d", respPatch.StatusCode())
		return
	}
}

func (s *Space) AgentUpdate(needToReboot bool, d deployment.Deployment) {
	if err := s.agentRegister(s.agentName, s.agentMachineID); err != nil {
		logrus.Errorf("space: fail to update the agent: %s", err)
		return
	}
	logrus.Infof("space: updating the agent with name %s and machineID %s", s.agentName, s.agentMachineID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	newAgent := NewAgent{
		Name:      s.agentName,
		MachineID: s.agentMachineID,
	}

	logrus.Debugf("space: PutAgentWithResponse(%s, %#v)", s.agentID.String(), newAgent)
	resp, err := s.client.PutAgentWithResponse(ctx, s.agentID.String(), newAgent)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	if resp.StatusCode() != http.StatusCreated {
		logrus.Errorf("space: expected HTTP 201 while updating the agent but received %d", resp.StatusCode())
		return
	}

	var uuidStrPtr *uuid.UUID
	if err := s.deploymentRegister(ctx, d); err != nil {
		logrus.Errorf("space: failed to register the deployment %s: %s", d.UUID, err)
		// TODO: convert deployment.UUID to uuid type
		uuidStr, _ := uuid.Parse(d.UUID)
		uuidStrPtr = &uuidStr
	}

	patchAgent := AgentPatchRequest{
		DeploymentID: uuidStrPtr,
		NeedToReboot: &needToReboot,
	}
	logrus.Debugf("space: PatchAgentWithResponse(%s, %#v)", s.agentID.String(), patchAgent)
	respPatch, err := s.client.PatchAgentWithResponse(ctx, *s.agentID, patchAgent)
	if err != nil {
		logrus.Error(err)
		return
	}
	if respPatch.StatusCode() != http.StatusOK {
		logrus.Errorf("space: expected HTTP 200 while patching the agent but received %d", respPatch.StatusCode())
		return
	}
}
