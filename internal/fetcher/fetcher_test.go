package fetcher

import (
	"testing"
	"time"

	"github.com/nlewo/comin/internal/repository"
	"github.com/nlewo/comin/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestFetcher(t *testing.T) {
	r := utils.NewRepositoryMock()
	f := NewFetcher(r)
	f.Start()

	for i := 0; i < 2; i++ {
		assert.False(t, f.IsFetching)
		f.TriggerFetch([]string{"remote"})

		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.True(c, f.IsFetching)
		}, 5*time.Second, 100*time.Millisecond, "fetcher is not fetching")

		// This is to simulate a git fetch
		r.RsCh <- repository.RepositoryStatus{
			SelectedCommitId: "foo",
		}
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			rs := <-f.RepositoryStatusCh
			assert.Equal(c, "foo", rs.SelectedCommitId)
		}, 5*time.Second, 100*time.Millisecond, "fetcher failed to fetch")

		assert.False(t, f.IsFetching)
	}
}

func TestUnion(t *testing.T) {
	res := union([]string{"r1", "r2"}, []string{"r1", "r3"})
	assert.Equal(t, []string{"r1", "r2", "r3"}, res)
}
