package nats

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/manager"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Nats struct {
	manager   *manager.Manager
	broker    *broker.Broker
	jsEvents  jetstream.JetStream
	jsFetched jetstream.JetStream
	pqueue    *PersistentQueue
}

func New(m *manager.Manager, b *broker.Broker) *Nats {
	n := &Nats{
		manager: m,
		broker:  b,
	}

	// Initialize persistent queue with a worker that publishes to NATS
	var initErr error
	n.pqueue, initErr = NewPersistentQueue("/tmp/comin-nats-pqueue.db", n.pqueueWorker)
	if initErr != nil {
		logrus.Errorf("nats: failed to initialize persistent queue: %s", initErr)
	}

	return n
}

func (n *Nats) pqueueWorker(ctx context.Context, stream, subject string, payload []byte) error {
	enrichedSubject := n.manager.Hostname + "." + subject
	switch subject {
	case "fetched":
		if n.jsFetched == nil {
			return fmt.Errorf("failed to publish to stream %s because it is not initialized yet", stream)
		}
		_, err := n.jsFetched.Publish(ctx, enrichedSubject, payload)
		if err != nil {
			return fmt.Errorf("failed to publish to stream %s: %w", stream, err)
		}
	default:
		if n.jsFetched == nil {
			return fmt.Errorf("failed to publish to stream %s because it is not initialized yet", stream)
		}
		_, err := n.jsEvents.Publish(ctx, enrichedSubject, payload)
		if err != nil {
			return fmt.Errorf("failed to publish to stream %s: %w", stream, err)
		}
	}
	return nil
}

func (n *Nats) listen() (err error) {
	subscriber := n.broker.Subscribe()
	defer n.broker.Unsubscribe(subscriber)

	s := n.manager.GetState()
	e := &protobuf.Event{
		Type: &protobuf.Event_ManagerState_{
			ManagerState: &protobuf.Event_ManagerState{
				State: s,
			},
		},
		CreatedAt: timestamppb.New(time.Now().UTC()),
	}
	data, marshalErr := proto.Marshal(e)
	if marshalErr != nil {
		return fmt.Errorf("nats: failed to marshal event: %s", marshalErr)
	}
	err = n.pqueue.Add("events", getEventType(e), data)
	if err != nil {
		return fmt.Errorf("nats: failed to add event to persistent queue: %s", err)
	}

	for event := range subscriber {
		data, marshalErr := proto.Marshal(event)
		if marshalErr != nil {
			logrus.Errorf("nats: failed to marshal event: %s", marshalErr)
			continue
		}

		subject := getEventType(event)
		stream := "events"
		if subject == "fetched" {
			stream = "fetched"
		}
		err := n.pqueue.Add(stream, subject, data)
		if err != nil {
			logrus.Errorf("nats: failed to add event to persistent queue: %s", err)
		}
	}

	return nil
}

func getEventType(event *protobuf.Event) string {
	switch event.GetType().(type) {
	case *protobuf.Event_EvalStartedType:
		return "eval.started"
	case *protobuf.Event_EvalFinishedType:
		return "eval.finished"
	case *protobuf.Event_BuildStartedType:
		return "build.started"
	case *protobuf.Event_BuildFinishedType:
		return "build.finished"
	case *protobuf.Event_ConfirmationSubmittedType:
		return "confirmation.submitted"
	case *protobuf.Event_ConfirmationCancelledType:
		return "confirmation.cancelled"
	case *protobuf.Event_ConfirmationConfirmedType:
		return "confirmation.confirmed"
	case *protobuf.Event_Resume_:
		return "resume"
	case *protobuf.Event_Suspend_:
		return "suspend"
	case *protobuf.Event_DeploymentStartedType:
		return "deployment.started"
	case *protobuf.Event_DeploymentFinishedType:
		return "deployment.finished"
	case *protobuf.Event_RebootRequired_:
		return "reboot.required"
	case *protobuf.Event_ManagerState_:
		return "manager.state"
	case *protobuf.Event_Fetched_:
		return "fetched"
	default:
		return "events"
	}
}

func (n *Nats) Start() (err error) {
	logrus.Info("nats: starting the client and listening to the event stream")
	go n.listen()
	natsUrl := os.Getenv("NATS_URL")
	natsToken := os.Getenv("NATS_TOKEN")
	nc, err := nats.Connect(natsUrl,
		nats.Token(natsToken),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.PingInterval(20*time.Second),
		nats.ConnectHandler(func(c *nats.Conn) {
			logrus.Info("nats: initial connection")
			n.jsEvents, err = jetstream.New(c)
			if err != nil {
				logrus.Errorf("nats: failed to create jetstream: %s", err)
				return
			}
			n.jsFetched, err = jetstream.New(c)
			if err != nil {
				logrus.Errorf("nats: failed to create jetstream: %s", err)
				return
			}
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logrus.Info("nats: reconnection")
			n.jsEvents, err = jetstream.New(c)
			if err != nil {
				logrus.Errorf("nats: failed to create jetstream: %s", err)
				return
			}
			n.jsFetched, err = jetstream.New(c)
			if err != nil {
				logrus.Errorf("nats: failed to create jetstream: %s", err)
				return
			}
		}),
		nats.ReconnectErrHandler(func(_ *nats.Conn, reconnectErr error) {
			fmt.Printf("nats: reconnection failed: %s\n", reconnectErr)
		}))
	if err != nil {
		return err
	}

	if nc.IsConnected() {
		logrus.Infof("nats: is connected")
	} else {
		logrus.Infof("nats: is not connected (but it will retry every minute)")
	}

	return nil
}
