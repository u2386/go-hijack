package gohijack

import (
	"context"
	"debug/dwarf"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:noinline
func this_is_for_test(string) int { return 0 }

func TestGoHijack(t *testing.T) {
	_ = this_is_for_test("")

	RegisterFailHandler(Fail)
	RunSpecs(t, "GoHijack Suite")
}

var _ = Describe("Test FuncTable", func() {
	Context("Read Symbol", func() {
		var (
			err   error
			dw    *dwarf.Data
			table map[string]*godwarf.Tree
		)

		Context("Open ELF", func() {
			BeforeEach(func() {
				pid := os.Getpid()
				dw, err = Open(fmt.Sprintf("/proc/%d/exe", pid))
			})

			It("should open successful", func() {
				Expect(err).To(BeNil())
			})

			Context("Read Function Table", func() {
				BeforeEach(func() {
					table, err = FuncTable(dw)
				})

				It("should find `this_is_for_test`", func() {
					Expect(err).To(BeNil())

					var name string
					for name = range table {
						if strings.HasSuffix(name, "this_is_for_test") {
							break
						}
					}
					Expect(name).ShouldNot(BeEquivalentTo(""))
				})
			})
		})
	})
})

var _ = Describe("Test UDS Listener", func() {
	Context("Listen on uds", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
			err    error
		)

		BeforeEach(func() {
			u := &uds{UDSAddress}
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

var _ = Describe("Test Patch", func() {
	Context("Test Time Patch", func ()  {
		var (
			g *Guard
			err error
			doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
		)

		BeforeEach(func ()  {
			g = PatchIndirect(reflect.ValueOf(time.Now), reflect.ValueOf(func() time.Time {
				return doom
			}))
		})

		AfterEach(func ()  {
			g.Unpatch()
		})

		It("should patch ok", func ()  {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(time.Now()).Should(BeEquivalentTo(doom))

			g.Unpatch()
			Expect(time.Now()).ShouldNot(BeEquivalentTo(doom))

			g.Restore()
			Expect(time.Now()).Should(BeEquivalentTo(doom))
		})
	})
})