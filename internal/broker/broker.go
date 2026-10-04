package broker

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Filter is a function that takes an event and returns true if the event should be delivered
// to the subscriber
type Filter func(*protobuf.Event) bool

// subscription holds a channel, its filter function, and a name
type subscription struct {
	name    string
	channel chan *protobuf.Event
	filter  Filter
}

type Broker struct {
	stopCh      chan struct{}
	publishCh   chan *protobuf.Event
	subscribers []*subscription
	mu          sync.RWMutex
}

func New() *Broker {
	return &Broker{
		stopCh:      make(chan struct{}),
		publishCh:   make(chan *protobuf.Event, 1),
		subscribers: make([]*subscription, 0),
	}
}

func (b *Broker) Start() {
	go func() {
		for {
			select {
			case <-b.stopCh:
				return
			case msg := <-b.publishCh:
				b.mu.RLock()
				for _, sub := range b.subscribers {
					// Apply filter - only send if the event passes the filter
					if sub.filter(msg) {
						// msgCh is buffered, use non-blocking send to protect the broker:
						select {
						case sub.channel <- msg:
						default:
						}
					}
				}
				b.mu.RUnlock()
			}
		}
	}()
}

func (b *Broker) Stop() {
	close(b.stopCh)
}

func (b *Broker) Subscribe(name string) chan *protobuf.Event {
	return b.SubscribeWithFilter(name, nil)
}

// SubscribeWithFilter creates a new subscription with a name and a custom filter function.
// The filter function receives each event and should return true to deliver the event
// to the subscriber, or false to filter it out.
// If the filter is nil, all events will be delivered (same as Subscribe())
func (b *Broker) SubscribeWithFilter(name string, filter Filter) chan *protobuf.Event {
	msgCh := make(chan *protobuf.Event, 5)
	
	// Default to accept all events if no filter is provided
	if filter == nil {
		filter = func(*protobuf.Event) bool { return true }
	}
	
	b.mu.Lock()
	b.subscribers = append(b.subscribers, &subscription{
		name:    name,
		channel: msgCh,
		filter:  filter,
	})
	b.mu.Unlock()
	
	return msgCh
}

func (b *Broker) Unsubscribe(msgCh chan *protobuf.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	for i, sub := range b.subscribers {
		if sub.channel == msgCh {
			// Remove this subscription by swapping with the last element and popping
			b.subscribers[i] = b.subscribers[len(b.subscribers)-1]
			b.subscribers = b.subscribers[:len(b.subscribers)-1]
			break
		}
	}
}

func (b *Broker) Publish(msg *protobuf.Event) {
	b.publishCh <- msg
}

// GetLogger return a stdout et stderr writers. They have to be closed by the caller.
func (b *Broker) GetLogger(obj, objUuid string) (stdout, stderr io.WriteCloser) {
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	b.Publish(
		&protobuf.Event{
			Type: &protobuf.Event_Log_{
				Log: &protobuf.Event_Log{
					ObjectType: obj,
					ObjectUuid: objUuid,
					Type: &protobuf.Event_Log_Open_{
						Open: &protobuf.Event_Log_Open{},
					},
				},
			},
			CreatedAt: timestamppb.New(time.Now().UTC()),
		},
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logrus.Debug("broker: starting to scan stdout")
		scanner := bufio.NewScanner(stdoutR)
		for scanner.Scan() {
			line := scanner.Text()
			logrus.Infof("logs: %s", line)
			b.Publish(
				&protobuf.Event{
					Type: &protobuf.Event_Log_{
						Log: &protobuf.Event_Log{
							ObjectType: obj,
							ObjectUuid: objUuid,
							Type: &protobuf.Event_Log_Line_{
								Line: &protobuf.Event_Log_Line{
									Source: "stdout",
									Msg:    line,
								},
							},
						},
					},
					CreatedAt: timestamppb.New(time.Now().UTC()),
				},
			)
		}
		logrus.Debug("broken: stdout/" + obj + "/" + objUuid + ": stdout closed")
	}()

	go func() {
		defer wg.Done()
		logrus.Debug("broker: starting to scan stderr")
		scanner := bufio.NewScanner(stderrR)
		for scanner.Scan() {
			line := scanner.Text()
			logrus.Infof("logs: %s", line)
			b.Publish(
				&protobuf.Event{
					Type: &protobuf.Event_Log_{
						Log: &protobuf.Event_Log{
							ObjectType: obj,
							ObjectUuid: objUuid,
							Type: &protobuf.Event_Log_Line_{
								Line: &protobuf.Event_Log_Line{
									Source: "stderr",
									Msg:    line,
								},
							},
						},
					},
					CreatedAt: timestamppb.New(time.Now().UTC()),
				},
			)
		}
		logrus.Debug("broken: stdout/" + obj + "/" + objUuid + ": stdout closed")
	}()

	go func() {
		wg.Wait()
		b.Publish(
			&protobuf.Event{
				Type: &protobuf.Event_Log_{
					Log: &protobuf.Event_Log{
						ObjectType: obj,
						ObjectUuid: objUuid,
						Type: &protobuf.Event_Log_Close_{
							Close: &protobuf.Event_Log_Close{},
						},
					},
				},
				CreatedAt: timestamppb.New(time.Now().UTC()),
			},
		)
	}()

	return stdoutW, stderrW
}
