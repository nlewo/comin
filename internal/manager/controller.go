package manager

import (
	"sync"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
)

type Controller struct {
	submitGenerationForBuild  chan string
	submitGenerationForDeploy chan string
	state                     *protobuf.Controller
	mu                        sync.Mutex
}

func NewController(enableConfirmationForBuild, enableConfirmationForDeploy bool) *Controller {
	return &Controller{
		submitGenerationForBuild:  make(chan string, 1),
		submitGenerationForDeploy: make(chan string, 1),
		state: &protobuf.Controller{
			GenerationEnableConfirmationForBuild:  enableConfirmationForBuild,
			GenerationEnableConfirmationForDeploy: enableConfirmationForDeploy,
		},
	}
}

func (c *Controller) State() *protobuf.Controller {
	return c.state
}

// AskForBuild indicates the buildment of a generation needs to be
// confirmed. If the confirmation for buildment is disable or if the
// generation has been already confirmed, it then notifies the
// generation is allowed to be builded.
func (c *Controller) AskForBuild(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	logrus.Infof("controller: ask for building generation %s", generationUUID)
	c.state.GenerationNeedConfirmationForBuild = generationUUID
	if !c.state.GenerationEnableConfirmationForBuild || c.state.GenerationAllowedForBuild == generationUUID {
		c.confirmForBuild(generationUUID)
	}
}

func (c *Controller) ConfirmForBuild(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: confirm for building generation %s", generationUUID)
	c.confirmForBuild(generationUUID)
}

func (c *Controller) confirmForBuild(generationUUID string) {
	c.state.GenerationAllowedForBuild = generationUUID
	if c.state.GenerationNeedConfirmationForBuild == generationUUID {
		c.submitGenerationForBuild <- generationUUID
	}
}

// AskForDeploy indicates the deployment of a generation needs to be
// confirmed. If the confirmation for deployment is disable or if the
// generation has been already confirmed, it then notifies the
// generation is allowed to be deployed.
func (c *Controller) AskForDeploy(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	logrus.Infof("controller: ask for deploying generation %s", generationUUID)
	c.state.GenerationNeedConfirmationForDeploy = generationUUID
	if !c.state.GenerationEnableConfirmationForDeploy || c.state.GenerationAllowedForDeploy == generationUUID {
		c.confirmForDeploy(generationUUID)
	}
}

func (c *Controller) ConfirmForDeploy(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: confirm for deploying generation %s", generationUUID)
	c.confirmForDeploy(generationUUID)
}

func (c *Controller) confirmForDeploy(generationUUID string) {
	c.state.GenerationAllowedForDeploy = generationUUID
	if c.state.GenerationNeedConfirmationForDeploy == generationUUID {
		c.submitGenerationForDeploy <- generationUUID
	}
}

func (c *Controller) ConfirmForBuildAndDeploy(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: confirm for building and deploying generation %s", generationUUID)
	c.confirmForBuild(generationUUID)
	c.confirmForDeploy(generationUUID)
}
