package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func NewNiks3Fetcher(config types.Niks3Fetcher, broker *broker.Broker) *Niks3Fetcher {
	f := &Niks3Fetcher{
		config:        config,
		broker:        broker,
		submitRemotes: make(chan []string),
	}
	for k, v := range config.Remotes {
		f.remotes = append(f.remotes, &protobuf.Niks3Remote{
			Name: k,
			Url:  v.URL,
		})
	}
	return f
}

type Niks3Fetcher struct {
	config        types.Niks3Fetcher
	isFetching    atomic.Bool
	mu            sync.RWMutex
	submitRemotes chan []string
	broker        *broker.Broker
	remotes       []*protobuf.Niks3Remote
}

func (f *Niks3Fetcher) IsFetching() bool {
	return f.isFetching.Load()
}

func (f *Niks3Fetcher) TriggerFetch(remotes []string) {
	f.submitRemotes <- remotes
}

func (f *Niks3Fetcher) GetState() *protobuf.Fetcher {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return &protobuf.Fetcher{
		IsFetching: wrapperspb.Bool(f.isFetching.Load()),
		Status: &protobuf.Fetcher_Niks3Status{
			Niks3Status: &protobuf.Niks3Status{
				Remotes: f.remotes,
			},
		},
	}
}

// pinInfoJSON is a temporary struct for unmarshaling JSON with string timestamps
type pinInfoJSON struct {
	Name      string `json:"name"`
	StorePath string `json:"store_path"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func fetch(route string) (*protobuf.PinInfo, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(route)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pinInfoJSON pinInfoJSON
	err = json.Unmarshal(body, &pinInfoJSON)
	if err != nil {
		return nil, err
	}

	// parseTimestamp tries multiple formats to parse a timestamp string
	parseTimestamp := func(s string) (*timestamppb.Timestamp, error) {
		if s == "" {
			return nil, nil
		}
		// Try RFC3339 first (e.g., "2024-01-02T12:00:00Z")
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return timestamppb.New(parsed), nil
		}
		// Try date only (e.g., "2024-01-02")
		if parsed, err := time.Parse("2006-01-02", s); err == nil {
			return timestamppb.New(parsed), nil
		}
		return nil, fmt.Errorf("failed to parse timestamp: %s", s)
	}

	// Convert string timestamps to timestamppb.Timestamp
	createdAt, err := parseTimestamp(pinInfoJSON.CreatedAt)
	if err != nil {
		return nil, err
	}

	updatedAt, err := parseTimestamp(pinInfoJSON.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &protobuf.PinInfo{
		Name:      pinInfoJSON.Name,
		StorePath: pinInfoJSON.StorePath,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (f *Niks3Fetcher) Start(ctx context.Context) {
	logrus.Info("fetcher niks3: starting")
	go func() {
		for {
			updated := false
			submittedRemotes := <-f.submitRemotes
			logrus.Debugf("niks3 fetcher: remotes submitted: %v", submittedRemotes)
			f.isFetching.Store(true)
			for _, remoteName := range submittedRemotes {
				pinInfo, err := fetch(f.config.Remotes[remoteName].URL)
				errMsg := ""
				if err != nil {
					logrus.Errorf("niks3 fetcher: failed to fetch remote %s: %v", remoteName, err)
					errMsg = err.Error()
				}
				f.mu.Lock()
				for _, r := range f.remotes {
					if r.Name == remoteName {
						r.FetchErrorMsg = errMsg
						r.Fetched = wrapperspb.Bool(err == nil)
						r.FetchedAt = timestamppb.New(time.Now().UTC())
						r.Updated = wrapperspb.Bool(false)
						if err == nil {
							// Check if this is an update (newer timestamp)
							if r.Pininfo != nil && pinInfo.UpdatedAt != nil && r.Pininfo.UpdatedAt != nil {
								if pinInfo.UpdatedAt.AsTime().After(r.Pininfo.UpdatedAt.AsTime()) {
									r.Updated = wrapperspb.Bool(true)
									updated = true
								}
							} else if r.Pininfo == nil || r.Pininfo.UpdatedAt == nil {
								// First fetch or no previous timestamp
								if pinInfo.UpdatedAt != nil {
									r.Updated = wrapperspb.Bool(true)
									updated = true
								}
							}
							r.Pininfo = pinInfo
						}
					}
				}
				f.mu.Unlock()
				if err == nil {
					logrus.Debugf("niks3 fetcher: fetched remote %s, pin: %s", remoteName, pinInfo.Name)
				}
			}
			f.isFetching.Store(false)
			// Publish fetched event
			f.broker.Publish(&protobuf.Event{
				Type: &protobuf.Event_Fetched_{Fetched: &protobuf.Event_Fetched{
					Type: &protobuf.Event_Fetched_Niks3Status{
						Niks3Status: &protobuf.Niks3Status{
							Remotes: f.remotes,
						},
					},
					Updated:  updated,
					Verified: false,
				}},
				CreatedAt: timestamppb.New(time.Now().UTC()),
			})
		}
	}()
}
