package memoize

import (
	"errors"
	"testing"
	"time"

	. "github.com/smartystreets/assertions"
	"github.com/smartystreets/gunit"
)

/*
	Testing goals, in order:

	1) automatically parallel on the terminal
	2) zero-ish overhead
	3) tolerable syntax

	For goal #1, Gunit seems to be the best-equipped to handle things, and there's some agreement on that:

	> I know that I'm the creator of GoConvey and all, but I've actually moved to gunit,
	> which uses t.Parallel() under the hood for every test case. - @mdwhatcott
	> https://github.com/smartystreets/goconvey/issues/360

	Test requirements:

	1) Each test works independent of any preexisting state, or lack thereof.
	2) Ideally tests can clean up after themselves, but this is not required.

	Please keep these goals and requirements in mind when modifying this package.
*/

/*

	A possible future goal:
	4) ability to both hit a testing infra, and/or replay locally.

	The best plan seems to be to incorporate go-vcr:
	https://github.com/dnaeon/go-vcr

	The implementation throws requests in YAML files, which... eh, let's try it maybe.
	There will have to be some setup trickery to transparently hit live or recorded.
	I think the vcr transport should handle that.
*/

// TestSuite fires off gunit.
//
// Gunit will look at various function name prefixes to determine behavior:
//
//   "Test": Well, it's a test.
//   "Skip": Skipped.
//   "Long": Skipped when `go test` is ran with the `short` flag.
//
//   "Setup":    Executed before each test.
//   "Teardown": Executed after  each test.
//
// Functions without these prefixes are ignored.
func TestSuite(t *testing.T) {
	gunit.Run(new(F), t)
}

// F is the default fixture, so-named for convenience.
type F struct {
	*gunit.Fixture
}

// Setup prepares the fixture. Runs once per test.
func (t *F) Setup() {

}

/*
// An example test:
func (t *F) TestExample() {
	t.So(42, ShouldEqual, 42)
	t.So("Hello, World!", ShouldContainSubstring, "World")
}
*/

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

// TestFailure checks that failed function values are not cached
func (t *F) TestFailure() {
	calls := 0

	// This function will fail IFF it has not been called before.
	twoForTheMoney := func() (interface{}, error) {
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
