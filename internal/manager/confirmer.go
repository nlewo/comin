package manager

import (
	"fmt"
	"time"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Mode int64

const (
	// The user needs to confirm the generation
	Manual Mode = iota
	// Confirm after several seconds the generation. This can be
	// cancelled by the user.
	Auto
	// Immediately confirm the submitted generation
	Without
)

func ParseMode(s string) (Mode, error) {
	switch s {
	case "manual":
		return Manual, nil
	case "auto":
		return Auto, nil
	case "without":
		return Without, nil
	default:
		return -1, fmt.Errorf("confirmer: invalid confirmer mode: '%s'", s)
	}
}

// Confirmer allows to handle user confirmations. A generation is
// submitted to the confirmer. Once a generation has been confirmed,
// it is pushed to the submit channel.
type Confirmer struct {
	state     *protobuf.Confirmer
	confirmed chan string
	timer     *time.Timer
	// To send a command such as confirm or cancel to the confirmer
	command    chan Command
	statusResp chan *protobuf.Confirmer
	statusReq  chan struct{}
}

func NewConfirmer(mode Mode, duration time.Duration) *Confirmer {
	return &Confirmer{
		state: &protobuf.Confirmer{
			AutoconfirmDuration: int64(duration.Seconds()),
			Mode:                int64(mode),
		},
		confirmed:  make(chan string),
		statusReq:  make(chan struct{}),
		statusResp: make(chan *protobuf.Confirmer),
		command:    make(chan Command),
	}
}

type Command struct {
	action string
	uuid   string
}

func (c *Confirmer) status() *protobuf.Confirmer {
	c.statusReq <- struct{}{}
	return <-c.statusResp
}

func (c *Confirmer) Submit(generationUuid string) {
	c.command <- Command{
		action: "submit",
		uuid:   generationUuid,
	}
}

func (c *Confirmer) Confirm(generationUuid string) {
	c.command <- Command{
		action: "confirm",
		uuid:   generationUuid,
	}
}
func (c *Confirmer) Cancel() {
	c.command <- Command{
		action: "cancel",
	}
}

func (c *Confirmer) Start() {
	go c.start()
}

func (c *Confirmer) start() {
	logrus.Infof("confirmer: starting with the autoconfirm duration: %d seconds", c.state.AutoconfirmDuration)
	var timer <-chan time.Time
	var notified bool
	for {
		select {
		case <-c.statusReq:
			c.statusResp <- proto.CloneOf(c.state)
		case command := <-c.command:
			switch command.action {
			case "submit":
				notified = false
				c.state.Submitted = command.uuid
				switch Mode(c.state.Mode) {
				case Manual:
					logrus.Infof("confirmer: generation %s has been submitted", command.uuid)
				case Without:
					logrus.Infof("confirmer: generation %s has been submitted and confirmed", command.uuid)
					c.state.Confirmed = command.uuid
				case Auto:
					logrus.Infof("confirmer: generation %s has been submitted and will be confirmed in %d seconds", command.uuid, c.state.AutoconfirmDuration)
					if c.timer != nil {
						c.timer.Stop()
					}
					c.timer = time.NewTimer(time.Duration(c.state.AutoconfirmDuration) * time.Second)
					timer = c.timer.C
					c.state.AutoconfirmStarted = wrapperspb.Bool(true)
					c.state.AutoconfirmStartedAt = timestamppb.New(time.Now().UTC())
				}
			case "confirm":
				logrus.Infof("confirmer: generation %s has been confirmed", command.uuid)
				c.state.Confirmed = command.uuid
			case "cancel":
				logrus.Infof("confirmer: confirmation of generation %s has been cancelled", command.uuid)
				c.state.Confirmed = ""
				c.state.AutoconfirmStarted = wrapperspb.Bool(false)
				if c.timer != nil {
					c.timer.Stop()
				}
			}
		case <-timer:
			logrus.Infof("confirmer: timer confirmed generation %s", c.state.Submitted)
			c.state.AutoconfirmStarted = wrapperspb.Bool(false)
			c.state.Confirmed = c.state.Submitted
		}
		if !notified && c.state.Submitted != "" && c.state.Confirmed != "" && c.state.Confirmed == c.state.Submitted {
			notified = true
			logrus.Infof("confirmer: confirmed generation %s", c.state.Confirmed)
			c.confirmed <- c.state.Confirmed
			c.state.Confirmed = ""
			c.state.Submitted = ""
			c.state.AutoconfirmStarted = wrapperspb.Bool(false)
		}
	}
}
