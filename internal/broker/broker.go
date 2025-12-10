package broker

import "github.com/nlewo/comin/internal/protobuf"

type Broker struct {
	stopCh    chan struct{}
	publishCh chan *protobuf.Event
	subCh     chan chan *protobuf.Event
	unsubCh   chan chan *protobuf.Event
}

func New() *Broker {
	return &Broker{
		stopCh:    make(chan struct{}),
		publishCh: make(chan *protobuf.Event, 1),
		subCh:     make(chan chan *protobuf.Event, 1),
		unsubCh:   make(chan chan *protobuf.Event, 1),
	}
}

func (b *Broker) Start() {
	go func() {
		subs := map[chan *protobuf.Event]struct{}{}
		for {
			select {
			case <-b.stopCh:
				return
			case msgCh := <-b.subCh:
				subs[msgCh] = struct{}{}
			case msgCh := <-b.unsubCh:
				delete(subs, msgCh)
			case msg := <-b.publishCh:
				for msgCh := range subs {
					// msgCh is buffered, use non-blocking send to protect the broker:
					select {
					case msgCh <- msg:
					default:
					}
				}
			}
		}
	}()
}

func (b *Broker) Stop() {
	close(b.stopCh)
}

func (b *Broker) Subscribe() chan *protobuf.Event {
	msgCh := make(chan *protobuf.Event, 5)
	b.subCh <- msgCh
	return msgCh
}

func (b *Broker) Unsubscribe(msgCh chan *protobuf.Event) {
	b.unsubCh <- msgCh
}

func (b *Broker) Publish(msg *protobuf.Event) {
	b.publishCh <- msg
}
