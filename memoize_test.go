package memoize

import (
	"errors"
	"sync"
	"sync/atomic"
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

// TestBasicGenerics adopts the code from readme.md into a simple test case but using generics.
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

// TestFailureGenerics checks that failed function values are not cached when using generics.
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

// TestConcurrency runs 10,000 goroutines of ~10ms tasks and ensures at least 99.9% deduplication occurs.
func (t *F) TestConcurrency() {
	var counter atomic.Int64
	var wg sync.WaitGroup

	expensive := func() (int64, error) {
		time.Sleep(10 * time.Millisecond)
		return counter.Add(1), nil
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	wg.Add(1000)
	for range 1000 {
		go func() {
			Call(cache, "key1", expensive)
			wg.Done()
		}()
	}

	wg.Wait()
	t.So(counter.Load(), ShouldBeLessThanOrEqualTo, 10)
}

// TestTrivialConcurrency hammers 10,000 trivial goroutines and ensures at least 95% deduplication occurs.
func (t *F) TestTrivialConcurrency() {
	var counter atomic.Int64
	var wg sync.WaitGroup

	expensive := func() (int64, error) {
		return counter.Add(1), nil
	}

	cache := NewMemoizer(90*time.Second, 10*time.Minute)

	wg.Add(10000)
	for range 10000 {
		go func() {
			Call(cache, "key1", expensive)
			wg.Done()
		}()
	}

	wg.Wait()
	t.So(counter.Load(), ShouldBeLessThanOrEqualTo, 500)
}
