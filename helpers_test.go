package workerbase

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// TODO - standardize these in a lib
//

type fakeTestingT struct{}

func (fakeTestingT) Errorf(format string, args ...interface{}) {}

func eventually(t *testing.T, f func() bool) {
	require.Eventually(t, f, time.Second, 10*time.Millisecond)
}

func readErrorValue(t *testing.T, ch <-chan error) error {
	timeout := time.After(time.Second)

	select {
	case value := <-ch:
		return value
	case <-timeout:
		require.Fail(t, "timed out")
	}

	return nil
}

// TODO - document
func consistently(t *testing.T, f func() bool) {
	// TODO - need to do something similar in mockgen
	if assert.Eventually(fakeTestingT{}, func() bool { return !f() }, 100*time.Millisecond, 10*time.Millisecond) {
		require.Fail(t, "not consistent")
	}
}

// TODO - document
func assertStructChanDoesNotReceive(t *testing.T, ch <-chan struct{}) {
	consistently(t, func() bool {
		select {
		case <-ch:
			return false
		default:
			return true
		}
	})
}

// TODO - document
func receiveStruct(ch <-chan struct{}) func() bool {
	return func() bool {
		select {
		case _, ok := <-ch:
			return ok
		default:
			return false
		}
	}
}
