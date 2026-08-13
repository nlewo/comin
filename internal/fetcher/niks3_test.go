package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/broker"
	"github.com/nlewo/comin/internal/types"
	"github.com/nlewo/comin/pkg/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestFetch(t *testing.T) {
	// Create a test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"test-pin","store_path":"/nix/store/path","created_at":"2024-01-01","updated_at":"2024-01-02"}`))
	}))
	defer ts.Close()

	pinInfo, err := fetch(ts.URL)
	assert.NoError(t, err)
	assert.NotNil(t, pinInfo)
	assert.Equal(t, "test-pin", pinInfo.Name)
	assert.Equal(t, "/nix/store/path", pinInfo.StorePath)
	assert.NotNil(t, pinInfo.CreatedAt)
	assert.Equal(t, "2024-01-01T00:00:00Z", pinInfo.CreatedAt.AsTime().Format(time.RFC3339))
	assert.NotNil(t, pinInfo.UpdatedAt)
	assert.Equal(t, "2024-01-02T00:00:00Z", pinInfo.UpdatedAt.AsTime().Format(time.RFC3339))
}

func TestFetchInvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json}`))
	}))
	defer ts.Close()

	pinInfo, err := fetch(ts.URL)
	assert.Error(t, err)
	assert.Nil(t, pinInfo)
}

func TestFetchServerError(t *testing.T) {
	// Create a test server that returns 500
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	pinInfo, err := fetch(ts.URL)
	assert.Error(t, err)
	assert.Nil(t, pinInfo)
}

func TestFetchInvalidURL(t *testing.T) {
	pinInfo, err := fetch("http://invalid-url-that-does-not-exist")
	assert.Error(t, err)
	assert.Nil(t, pinInfo)
}

func TestNiks3FetcherStart(t *testing.T) {
	// Create a test server with RFC3339 timestamp
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"test-pin","store_path":"/nix/store/path","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-02T12:00:00Z"}`))
	}))
	defer ts.Close()

	// Create broker and fetcher
	bk := broker.New()
	bk.Start()
	defer bk.Stop()

	config := types.Niks3Fetcher{
		Remotes: map[string]types.Niks3Remote{
			"test-remote": {URL: ts.URL},
		},
	}

	f := NewNiks3Fetcher(config, bk)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx)

	// Subscribe to broker events
	brokerEvents := bk.Subscribe()

	// Trigger fetch
	f.TriggerFetch([]string{"test-remote"})

	// Wait for the Fetched event
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		e := <-brokerEvents
		fetched := e.GetFetched()
		assert.NotNil(c, fetched)
		assert.True(c, fetched.Updated)
		assert.NotNil(c, fetched.Type)
		niks3Status := fetched.Type.(*protobuf.Event_Fetched_Niks3Status)
		assert.NotNil(c, niks3Status)
		assert.NotNil(c, niks3Status.Niks3Status)
		assert.Len(c, niks3Status.Niks3Status.Remotes, 1)
		assert.Equal(c, "test-remote", niks3Status.Niks3Status.Remotes[0].Name)
		assert.True(c, niks3Status.Niks3Status.Remotes[0].Fetched.GetValue())
		// Verify PinInfo is set
		assert.NotNil(c, niks3Status.Niks3Status.Remotes[0].Pininfo)
		assert.Equal(c, "test-pin", niks3Status.Niks3Status.Remotes[0].Pininfo.Name)
		assert.Equal(c, "/nix/store/path", niks3Status.Niks3Status.Remotes[0].Pininfo.StorePath)
		assert.NotNil(c, niks3Status.Niks3Status.Remotes[0].Pininfo.CreatedAt)
		assert.Equal(c, "2024-01-01T00:00:00Z", niks3Status.Niks3Status.Remotes[0].Pininfo.CreatedAt.AsTime().Format(time.RFC3339))
		assert.NotNil(c, niks3Status.Niks3Status.Remotes[0].Pininfo.UpdatedAt)
		assert.Equal(c, "2024-01-02T12:00:00Z", niks3Status.Niks3Status.Remotes[0].Pininfo.UpdatedAt.AsTime().Format(time.RFC3339))
		// Verify FetchedAt timestamp is set
		assert.NotNil(c, niks3Status.Niks3Status.Remotes[0].FetchedAt)
	}, 5*time.Second, 100*time.Millisecond, "expected Fetched event not received")

	// Verify the fetcher state
	state := f.GetState()
	assert.True(t, state.IsFetching.GetValue() == false)
	niks3Status := state.Status.(*protobuf.Fetcher_Niks3Status)
	assert.NotNil(t, niks3Status)
	assert.Len(t, niks3Status.Niks3Status.Remotes, 1)
	assert.Equal(t, "test-remote", niks3Status.Niks3Status.Remotes[0].Name)
	assert.True(t, niks3Status.Niks3Status.Remotes[0].Fetched.GetValue())
	// Verify PinInfo is set in state
	assert.NotNil(t, niks3Status.Niks3Status.Remotes[0].Pininfo)
	assert.Equal(t, "test-pin", niks3Status.Niks3Status.Remotes[0].Pininfo.Name)
	assert.Equal(t, "/nix/store/path", niks3Status.Niks3Status.Remotes[0].Pininfo.StorePath)
	assert.NotNil(t, niks3Status.Niks3Status.Remotes[0].Pininfo.CreatedAt)
	assert.Equal(t, "2024-01-01T00:00:00Z", niks3Status.Niks3Status.Remotes[0].Pininfo.CreatedAt.AsTime().Format(time.RFC3339))
	assert.NotNil(t, niks3Status.Niks3Status.Remotes[0].Pininfo.UpdatedAt)
	assert.Equal(t, "2024-01-02T12:00:00Z", niks3Status.Niks3Status.Remotes[0].Pininfo.UpdatedAt.AsTime().Format(time.RFC3339))
	// Verify FetchedAt timestamp is set
	assert.NotNil(t, niks3Status.Niks3Status.Remotes[0].FetchedAt)
}

func TestNiks3FetcherUpdatedComparison(t *testing.T) {
	// Create a test server with a newer timestamp
	newerTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"test-pin","store_path":"/nix/store/path","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-03T12:00:00Z"}`))
	}))
	defer newerTs.Close()

	// Create a test server with an older timestamp
	olderTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"test-pin","store_path":"/nix/store/path","created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T12:00:00Z"}`))
	}))
	defer olderTs.Close()

	// Create broker and fetcher with newer timestamp
	bk := broker.New()
	bk.Start()
	defer bk.Stop()

	config := types.Niks3Fetcher{
		Remotes: map[string]types.Niks3Remote{
			"test-remote": {URL: newerTs.URL},
		},
	}

	f := NewNiks3Fetcher(config, bk)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx)

	// Subscribe to broker events
	brokerEvents := bk.Subscribe()

	// Trigger first fetch with newer timestamp
	f.TriggerFetch([]string{"test-remote"})

	// Wait for the Fetched event and verify Updated is true
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		e := <-brokerEvents
		fetched := e.GetFetched()
		assert.NotNil(c, fetched)
		assert.True(c, fetched.Updated)
	}, 5*time.Second, 100*time.Millisecond, "expected Fetched event with Updated=true")

	// Verify the remote has Updated=true
	state := f.GetState()
	niks3Status := state.Status.(*protobuf.Fetcher_Niks3Status)
	assert.NotNil(t, niks3Status.Niks3Status.Remotes[0].Updated)
	assert.True(t, niks3Status.Niks3Status.Remotes[0].Updated.GetValue())

	// Update config to point to older timestamp server
	config.Remotes["test-remote"] = types.Niks3Remote{URL: olderTs.URL}
	f.config = config

	// Trigger second fetch with older timestamp
	f.TriggerFetch([]string{"test-remote"})

	// Wait for the Fetched event and verify Updated is false
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		e := <-brokerEvents
		fetched := e.GetFetched()
		assert.NotNil(c, fetched)
		assert.False(c, fetched.Updated)
	}, 5*time.Second, 100*time.Millisecond, "expected Fetched event with Updated=false")

	// Verify the remote has Updated=false (older timestamp)
	state = f.GetState()
	niks3Status = state.Status.(*protobuf.Fetcher_Niks3Status)
	assert.NotNil(t, niks3Status.Niks3Status.Remotes[0].Updated)
	assert.False(t, niks3Status.Niks3Status.Remotes[0].Updated.GetValue())
}
