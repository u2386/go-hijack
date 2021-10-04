package gohijack

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/u2386/go-hijack/runtime"
)

//go:noinline
func this_is_for_test(i int) string { return fmt.Sprint(i) }

func TestGoHijack(t *testing.T) {
	_ = this_is_for_test(0)

	RegisterFailHandler(Fail)
	RunSpecs(t, "GoHijack Suite")
}

var _ = Describe("Test UDS Listener", func() {
	Context("Listen on uds", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
			err    error
		)

		BeforeEach(func() {
			u := &uds{
				Addr: UDSAddress,
			}
			ctx, cancel = context.WithCancel(context.Background())

			ch := make(chan error, 1)
			go func() {
				select {
				case <-ctx.Done():
				case ch <- u.Run(ctx):
				}
			}()

			select {
			case err = <-ch:
			case <-time.After(500 * time.Millisecond):
			}
		})

		AfterEach(func() {
			if _, err := os.Stat(UDSAddress); err == nil {
				if err := os.RemoveAll(UDSAddress); err != nil {
					fmt.Fprintf(os.Stderr, "unexcepted error:%s", err)
				}
			}
		})

		It("creates a sock", func() {
			Expect(err).Should(BeNil())

			_, err = os.Stat(UDSAddress)
			Expect(err).Should(BeNil())

			cancel()
			time.Sleep(500 * time.Millisecond)

			_, err = os.Stat(UDSAddress)
			Expect(err).Should(HaveOccurred())
		})

	})
})

var _ = Describe("Test Parser", func() {
	Context("Test Json Parser", func() {
		var (
			point runtime.HijackPoint
		)

		BeforeEach(func() {
			parser := JsonParser()
			m := parser.Parse(`
			{
				"func":"this_is_for_test",
				"action":"delay",
				"val": 10
			}
			`)
			mapstructure.Decode(m, &point)
		})

		It("should parse successfully", func() {
			Expect(point).ShouldNot(BeNil())
			Expect(point.Func).To(BeEquivalentTo("this_is_for_test"))
			Expect(point.Action).To(BeEquivalentTo("delay"))
		})
	})
})
