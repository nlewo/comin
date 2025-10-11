package manager

import (
	"sync"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Confirmer struct {
	state  *protobuf.Confirmer
	submit chan string
}

func NewConfirmer(enable bool) *Confirmer {
	return &Confirmer{
		state: &protobuf.Confirmer{
			Enabled: wrapperspb.Bool(enable),
		},
		submit: make(chan string, 1),
	}
}

func (c *Confirmer) ask(generationUUID string) {
	c.state.Needed = generationUUID
	if !c.state.Enabled.GetValue() || c.state.Allowed == generationUUID {
		c.Confirm(generationUUID)
	}
}

func (c *Confirmer) Confirm(generationUUID string) {
	c.state.Allowed = generationUUID
	if c.state.Needed == generationUUID {
		c.state.Needed = ""
		c.submit <- generationUUID
	}
}

type Controller struct {
	Build  *Confirmer
	Deploy *Confirmer
	mu     sync.Mutex
}

func NewController(enableConfirmationForBuild, enableConfirmationForDeploy bool) *Controller {
	return &Controller{
		Build:  NewConfirmer(enableConfirmationForBuild),
		Deploy: NewConfirmer(enableConfirmationForDeploy),
	}
}

func (c *Controller) State() *protobuf.Controller {
	return &protobuf.Controller{
		Build:  c.Build.state,
		Deploy: c.Deploy.state,
	}
}

// AskForBuild indicates the buildment of a generation needs to be
// confirmed. If the confirmation for buildment is disable or if the
// generation has been already confirmed, it then notifies the
// generation is allowed to be builded.
func (c *Controller) AskForBuild(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: ask for building generation %s", generationUUID)
	c.Build.ask(generationUUID)
}

func (c *Controller) ConfirmForBuild(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: confirm for building generation %s", generationUUID)
	c.Build.Confirm(generationUUID)
}

// AskForDeploy indicates the deployment of a generation needs to be
// confirmed. If the confirmation for deployment is disable or if the
// generation has been already confirmed, it then notifies the
// generation is allowed to be deployed.
func (c *Controller) AskForDeploy(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	logrus.Infof("controller: ask for deploying generation %s", generationUUID)
	c.Deploy.ask(generationUUID)
}

func (c *Controller) ConfirmForDeploy(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: confirm for deploying generation %s", generationUUID)
	c.Deploy.Confirm(generationUUID)
}

func (c *Controller) ConfirmForBuildAndDeploy(generationUUID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logrus.Infof("controller: confirm for building and deploying generation %s", generationUUID)
	c.Build.Confirm(generationUUID)
	c.Deploy.Confirm(generationUUID)
}
