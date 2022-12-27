package memoize

import (
	"errors"
	"testing"
	"time"

	. "github.com/smartystreets/assertions"
	"github.com/smartystreets/gunit"
)

func TestSuite(t *testing.T) {
	gunit.Run(new(F), t)
}

type F struct {
	*gunit.Fixture
}

// TestBasic adopts the code from readme.md into a simple test case
func (t *F) TestBasic() {
	expensiveCalls := 0

	// Function tracks how many times its been called
	expensive := func() (interface{}, error) {
		expensiveCalls++
		return expensiveCalls, nil
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	// First call SHOULD NOT be cached
	result, err, cached := cache.Memoize("key1", expensive)
	t.So(err, ShouldBeNil)
	t.So(result.(int), ShouldEqual, 1)
	t.So(cached, ShouldBeFalse)

	// Second call on same key SHOULD be cached
	result, err, cached = cache.Memoize("key1", expensive)
	t.So(err, ShouldBeNil)
	t.So(result.(int), ShouldEqual, 1)
	t.So(cached, ShouldBeTrue)

	// First call on a new key SHOULD NOT be cached
	result, err, cached = cache.Memoize("key2", expensive)
	t.So(err, ShouldBeNil)
	t.So(result.(int), ShouldEqual, 2)
	t.So(cached, ShouldBeFalse)
}

// TestFailure checks that failed function values are cached
func (t *F) TestFailure() {
	calls := 0
	functionReturningError := func() (interface{}, error) {
		calls += 1
		return calls, errors.New("Try again")
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	// First call should fail, and be cached
	result, err, cached := cache.Memoize("key1", functionReturningError)
	t.So(err.Error(), ShouldEqual, "Try again")
	t.So(result.(int), ShouldEqual, 1)
	t.So(cached, ShouldBeFalse)

	// Read from cache
	result, err, cached = cache.Memoize("key1", functionReturningError)
	t.So(err.Error(), ShouldEqual, "Try again")
	t.So(result.(int), ShouldEqual, 1)
	t.So(cached, ShouldBeTrue)
}
