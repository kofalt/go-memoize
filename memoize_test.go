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
	expensive := func() (any, error) {
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

// TestFailure checks that failed function values are not cached
func (t *F) TestFailure() {
	calls := 0

	// This function will fail IFF it has not been called before.
	twoForTheMoney := func() (any, error) {
		calls++

		if calls == 1 {
			return calls, errors.New("Try again")
		} else {
			return calls, nil
		}
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	// First call should fail, and not be cached
	result, err, cached := cache.Memoize("key1", twoForTheMoney)
	t.So(err, ShouldNotBeNil)
	t.So(result.(int), ShouldEqual, 1)
	t.So(cached, ShouldBeFalse)

	// Second call should succeed, and not be cached
	result, err, cached = cache.Memoize("key1", twoForTheMoney)
	t.So(err, ShouldBeNil)
	t.So(result.(int), ShouldEqual, 2)
	t.So(cached, ShouldBeFalse)

	// Third call should succeed, and be cached
	result, err, cached = cache.Memoize("key1", twoForTheMoney)
	t.So(err, ShouldBeNil)
	t.So(result.(int), ShouldEqual, 2)
	t.So(cached, ShouldBeTrue)
}

// TestBasicGenerics adopts the code from readme.md into a simple test case
// but using generics.
func (t *F) TestBasicGenerics() {
	expensiveCalls := 0

	// Function tracks how many times its been called
	expensive := func() (int, error) {
		expensiveCalls++
		return expensiveCalls, nil
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	// First call SHOULD NOT be cached
	result, err, cached := Call(cache, "key1", expensive)
	t.So(err, ShouldBeNil)
	t.So(result, ShouldEqual, 1)
	t.So(cached, ShouldBeFalse)

	// Second call on same key SHOULD be cached
	result, err, cached = Call(cache, "key1", expensive)
	t.So(err, ShouldBeNil)
	t.So(result, ShouldEqual, 1)
	t.So(cached, ShouldBeTrue)

	// First call on a new key SHOULD NOT be cached
	result, err, cached = Call(cache, "key2", expensive)
	t.So(err, ShouldBeNil)
	t.So(result, ShouldEqual, 2)
	t.So(cached, ShouldBeFalse)
}

// TestFailureGenerics checks that failed function values are not cached
// when using generics.
func (t *F) TestFailureGenerics() {
	calls := 0

	// This function will fail IFF it has not been called before.
	twoForTheMoney := func() (int, error) {
		calls++

		if calls == 1 {
			return calls, errors.New("Try again")
		} else {
			return calls, nil
		}
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	// First call should fail, and not be cached
	result, err, cached := Call(cache, "key1", twoForTheMoney)
	t.So(err, ShouldNotBeNil)
	t.So(result, ShouldEqual, 1)
	t.So(cached, ShouldBeFalse)

	// Second call should succeed, and not be cached
	result, err, cached = Call(cache, "key1", twoForTheMoney)
	t.So(err, ShouldBeNil)
	t.So(result, ShouldEqual, 2)
	t.So(cached, ShouldBeFalse)

	// Third call should succeed, and be cached
	result, err, cached = Call(cache, "key1", twoForTheMoney)
	t.So(err, ShouldBeNil)
	t.So(result, ShouldEqual, 2)
	t.So(cached, ShouldBeTrue)
}
