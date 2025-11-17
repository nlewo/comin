package manager

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfirmerSubmit(t *testing.T) {
	c := NewConfirmer(Manual, time.Second)
	c.Start()
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "", c.status().Submitted)
	}, 3*time.Second, 100*time.Millisecond)

	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "uuid1", c.status().Submitted)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestConfirmerManual(t *testing.T) {
	c := NewConfirmer(Manual, time.Second)
	go func() {
		<-c.confirmed
	}()
	c.Start()
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "", c.status().Submitted)
	}, 3*time.Second, 100*time.Millisecond)

	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "uuid1", c.status().Submitted)
		assert.Equal(ct, "", c.status().Confirmed)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestConfirmerWithout(t *testing.T) {
	c := NewConfirmer(Without, 0)
	var expectedUuid atomic.Bool
	go func() {
		t := <-c.confirmed
		if t == "uuid1" {
			expectedUuid.Store(true)
		}
	}()
	c.Start()
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "", c.status().Submitted)
	}, 3*time.Second, 100*time.Millisecond)

	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.True(ct, expectedUuid.Load())
	}, 1*time.Second, 100*time.Millisecond)
}

func TestConfirmerAuto(t *testing.T) {
	c := NewConfirmer(Auto, 2*time.Second)
	var expectedUuid atomic.Bool
	go func() {
		t := <-c.confirmed
		if t == "uuid1" {
			expectedUuid.Store(true)
		}
	}()
	c.Start()
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "", c.status().Submitted)
	}, 1*time.Second, 100*time.Millisecond)

	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "uuid1", c.status().Submitted)
		assert.Equal(ct, "", c.status().Confirmed)
	}, 1*time.Second, 100*time.Millisecond)

	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.True(ct, expectedUuid.Load())
	}, 3*time.Second, 100*time.Millisecond)
}

func TestConfirmerAutoCancel(t *testing.T) {
	c := NewConfirmer(Auto, 2*time.Second)
	go func() {
		<-c.confirmed
	}()
	c.Start()
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "", c.status().Submitted)
	}, 1*time.Second, 100*time.Millisecond)

	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "uuid1", c.status().Submitted)
		assert.True(ct, c.status().AutoconfirmStarted.GetValue())
	}, 1*time.Second, 100*time.Millisecond)

	c.Cancel()
	assert.Never(t, func() bool {
		return c.status().Confirmed == "uuid1"
	}, 3*time.Second, 100*time.Millisecond)
}

func TestConfirmerResubmit(t *testing.T) {
	c := NewConfirmer(Auto, 3*time.Second)
	var expectedUuid atomic.Bool
	go func() {
		t := <-c.confirmed
		if t == "uuid2" {
			expectedUuid.Store(true)
		}
	}()
	c.Start()
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "", c.status().Submitted)
	}, 1*time.Second, 100*time.Millisecond)

	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.Equal(ct, "uuid1", c.status().Submitted)
		assert.True(ct, c.status().AutoconfirmStarted.GetValue())
	}, 1*time.Second, 100*time.Millisecond)
	assert.Never(t, func() bool {
		return c.status().Confirmed == "uuid1"
	}, 1*time.Second, 100*time.Millisecond)

	c.Submit("uuid2")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.True(ct, c.status().AutoconfirmStarted.GetValue())
	}, 1*time.Second, 100*time.Millisecond)
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.True(ct, expectedUuid.Load())
	}, 4*time.Second, 100*time.Millisecond)
}

func TestConfirmerConfirmBeforeSubmit(t *testing.T) {
	c := NewConfirmer(Auto, 3*time.Second)
	var expectedUuid atomic.Bool
	go func() {
		t := <-c.confirmed
		if t == "uuid1" {
			expectedUuid.Store(true)
		}
	}()
	c.Start()
	c.Confirm("uuid1")
	assert.Never(t, func() bool {
		return expectedUuid.Load()
	}, 1*time.Second, 100*time.Millisecond)
	c.Submit("uuid1")
	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		assert.True(ct, expectedUuid.Load())
	}, 4*time.Second, 100*time.Millisecond)
}
