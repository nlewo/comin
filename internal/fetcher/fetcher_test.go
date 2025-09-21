package fetcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/nlewo/comin/internal/protobuf"
	"github.com/nlewo/comin/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestFetcher(t *testing.T) {
	r := utils.NewRepositoryMock()
	f := NewFetcher(r)
	f.Start()
	var commitId string

	for i := 0; i < 2; i++ {
		assert.False(t, f.IsFetching())
		f.TriggerFetch([]string{"remote"})

		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.True(c, f.IsFetching())
		}, 5*time.Second, 100*time.Millisecond, "fetcher is not fetching")

		// This is to simulate a git fetch
		commitId = fmt.Sprintf("id-%d", i)
		r.RsCh <- &protobuf.RepositoryStatus{
			SelectedCommitId: commitId,
		}
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			rs := <-f.RepositoryStatusCh
			assert.Equal(c, commitId, rs.SelectedCommitId)
		}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")

		assert.False(t, f.IsFetching())
	}

	f.TriggerFetch([]string{"remote"})
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id-5",
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		rs := <-f.RepositoryStatusCh
		assert.Equal(c, "id-5", rs.SelectedCommitId)
	}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")

	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id-5",
	}
	r.RsCh <- &protobuf.RepositoryStatus{
		SelectedCommitId: "id-6",
	}
	rs := <-f.RepositoryStatusCh
	assert.NotEqual(t, "id-5", rs.SelectedCommitId)
}

func TestUnion(t *testing.T) {
	res := union([]string{"r1", "r2"}, []string{"r1", "r3"})
	assert.Equal(t, []string{"r1", "r2", "r3"}, res)
}
