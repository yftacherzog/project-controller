package logr_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/konflux-ci/project-controller/pkg/logr/muxr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

var _ = Describe("Logr", func() {
	var logger logr.Logger
	var outputs []fmt.Stringer
	var expectOutputsMatch func(lines ...string)
	var ImplementsLogrBehaviour func()

	BeforeEach(func() {
		format.UseStringerRepresentation = true
		expectOutputsMatch = func(lines ...string) {
			GinkgoHelper()
			Expect(outputs).Should(HaveEach(matchRegexpLines(lines...)))
		}
	})

	ImplementsLogrBehaviour = func() {
		It("Logs info messages", func() {
			logger.Info("Some info")

			expectOutputsMatch(` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="Some info"`)
		})
		It("Logs at different levels", func() {
			logger.Info("Some info")
			logger.V(1).Info("Less important info")

			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="Some info"`,
				` "ts"="[0-9 :\-\.]+" "level"=1 "msg"="Less important info"`,
			)
		})
		It("Can have nested names", func() {
			logger.Info("Some info")
			logger.WithName("foo1").Info("from foo1")
			l := logger.WithName("foo2")
			l.Info("from foo2")
			l.WithName("bar").Info("from foo2/bar")

			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="Some info"`,
				`foo1 "ts"="[0-9 :\-\.]+" "level"=0 "msg"="from foo1"`,
				`foo2 "ts"="[0-9 :\-\.]+" "level"=0 "msg"="from foo2"`,
				`foo2/bar "ts"="[0-9 :\-\.]+" "level"=0 "msg"="from foo2/bar"`,
			)
		})
		It("Can log arbitrary key/value pairs", func() {
			logger.Info("info with", "foo", "bar", "number", 1)
			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="info with" "foo"="bar" "number"=1`,
			)
		})
		It("Can have sub-instances with key/value pairs added", func() {
			l1 := logger.WithValues("foo", "bar")
			l2 := l1.WithValues("bal", "baz")
			l1.Info("one")
			l2.Info("two")
			l1.Info("three")
			l2.Info("four", "every", "body", "hit", "the floor")

			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="one" "foo"="bar"`,
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="two" "foo"="bar" "bal"="baz"`,
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="three" "foo"="bar"`,
				` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="four" "foo"="bar" "bal"="baz" "every"="body" "hit"="the floor"`,
			)
		})
		It("Can log errors with caller info", func() {
			logger.Error(errors.New("oops!"), "whoa!")
			caller := previousLineCaller()

			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "caller"=` + caller + ` "msg"="whoa!" "error"="oops!"`,
			)
		})
		It("Can skip logging caller frames witin a given depth", func() {
			errorHelper := func(em string) {
				logger.WithCallDepth(1).Error(errors.New(em), "gevalt!")
			}

			errorHelper("oy vey!")
			caller := previousLineCaller()

			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "caller"=` + caller + ` "msg"="gevalt!" "error"="oy vey!"`,
			)
		})
		It("Can provide a caller frame skipping marker fuction", func() {
			marker, l := logger.WithCallStackHelper()

			testHelper := func(em string) {
				marker()
				l.Error(errors.New(em), "hoo ha!")
			}

			testHelper("ma kara!")
			caller := previousLineCaller()

			expectOutputsMatch(
				` "ts"="[0-9 :\-\.]+" "caller"=` + caller + ` "msg"="hoo ha!" "error"="ma kara!"`,
			)
		})
	}

	Describe("Simple funcr-based logger", func() {
		BeforeEach(func() {
			var builder *strings.Builder
			logger, builder = stringsBuilderLogr()
			outputs = []fmt.Stringer{builder}
		})

		ImplementsLogrBehaviour()
	})

	Describe("Muxr - a multiplexing logger", func() {
		BeforeEach(func() {
			var b1, b2 *strings.Builder
			var l1, l2 logr.Logger
			l1, b1 = stringsBuilderLogr()
			l2, b2 = stringsBuilderLogr()
			outputs = []fmt.Stringer{b1, b2}

			logger = muxr.NewMuxLogger(l1, l2)
		})

		ImplementsLogrBehaviour()

		It("Supports loggers with different verbosity", func() {
			var b1, b2 *strings.Builder
			var l1, l2 logr.Logger
			l1, b1 = stringsBuilderLogrV(1)
			l2, b2 = stringsBuilderLogrV(0)
			outputs = []fmt.Stringer{b1, b2}

			logger = muxr.NewMuxLogger(l1, l2)

			logger.Info("l0 msg")
			logger.V(1).Info("l1 msg")

			Expect(outputs).To(HaveExactElements(
				matchRegexpLines(
					` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="l0 msg"`,
					` "ts"="[0-9 :\-\.]+" "level"=1 "msg"="l1 msg"`,
				),
				matchRegexpLines(
					` "ts"="[0-9 :\-\.]+" "level"=0 "msg"="l0 msg"`,
				),
			))
		})
	})
})

func stringsBuilderLogr() (logr.Logger, *strings.Builder) {
	return stringsBuilderLogrV(1)
}

func stringsBuilderLogrV(verbosity int) (logr.Logger, *strings.Builder) {
	builder := strings.Builder{}
	logger := funcr.New(func(prefix, args string) {
		builder.WriteString(prefix)
		builder.WriteString(" ")
		builder.WriteString(args)
		builder.WriteString("\n")
	}, funcr.Options{
		LogCaller:     funcr.Error,
		LogCallerFunc: false,
		LogTimestamp:  true,
		Verbosity:     verbosity,
	})
	return logger, &builder
}

func previousLineCaller() string {
	_, f, l, ok := runtime.Caller(1)
	caller := fmt.Sprintf(`{"file"="%s" "line"=%d}`, filepath.Base(f), l-1)
	Expect(ok).To(BeTrue())
	return caller
}

func matchRegexpLines(lines ...string) types.GomegaMatcher {
	return MatchRegexp(
		fmt.Sprintf("^%s\n$", strings.Join(lines, "\n")),
	)
}
