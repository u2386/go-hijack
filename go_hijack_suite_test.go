package gohijack

import (
	"context"
	"debug/dwarf"
	"debug/elf"
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

var (
	doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
)

//go:noinline
func this_is_for_test(i int) string { return fmt.Sprint(i) }

func TestGoHijack(t *testing.T) {
	_ = this_is_for_test(0)

	RegisterFailHandler(Fail)
	RunSpecs(t, "GoHijack Suite")
}

var _ = Describe("Test Read Dwarf Tree", func() {
	Context("Read Symbol", func() {
		var (
			err   error
			dw    *dwarf.Data
			trees map[string]*godwarf.Tree
			syms  []elf.Symbol
		)

		Context("Open ELF", func() {
			BeforeEach(func() {
				pid := os.Getpid()
				ef, _ := elf.Open(fmt.Sprintf("/proc/%d/exe", pid))
				dw, _ = ef.DWARF()
				syms, _ = ef.Symbols()

				trees, err = DwarfTree(dw)
			})

			It("should find `this_is_for_test`", func() {
				Expect(err).To(BeNil())

				var (
					name string
					sym  elf.Symbol
				)
				for name = range trees {
					if strings.HasSuffix(name, "this_is_for_test") {
						break
					}
				}
				Expect(name).ShouldNot(BeEmpty())

				for _, sym = range syms {
					if strings.HasSuffix(sym.Name, "this_is_for_test") {
						break
					}
				}
				Expect(sym.Name).ShouldNot(BeEmpty())
				Expect(sym.Value).ShouldNot(BeZero())
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
			u := &uds{
				addr: UDSAddress,
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

func doomer() time.Time { return doom }

var _ = Describe("Test Patch", func() {
	Context("Test Time Patch by Pointer", func() {
		var (
			g   *Guard
			sym elf.Symbol
		)

		BeforeEach(func() {
			pid := os.Getpid()
			ef, _ := elf.Open(fmt.Sprintf("/proc/%d/exe", pid))
			syms, _ := ef.Symbols()
			for _, sym = range syms {
				if strings.HasSuffix(sym.Name, "time.Now") {
					break
				}
			}

			g = PatchIndirect(doomer, sym.Value)
		})

		AfterEach(func() {
			g.Unpatch()
		})

		It("should patch ok", func() {
			Expect(g).ShouldNot(BeNil())
			Expect(doomer()).ShouldNot(BeEquivalentTo(doom))

			g.Unpatch()
			Expect(doomer()).Should(BeEquivalentTo(doom))

			g.Restore()
			Expect(doomer()).ShouldNot(BeEquivalentTo(doom))
		})
	})

	Context("Test Time Patch Indirect by Pointer", func() {
		var (
			g    *Guard
			sym  elf.Symbol
			doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
		)

		BeforeEach(func() {
			pid := os.Getpid()
			ef, _ := elf.Open(fmt.Sprintf("/proc/%d/exe", pid))
			syms, _ := ef.Symbols()
			for _, sym = range syms {
				if strings.HasSuffix(sym.Name, "time.Now") {
					break
				}
			}

			g = PatchIndirect(sym.Value, func() time.Time {
				return doom
			})
		})

		AfterEach(func() {
			g.Unpatch()
		})

		It("should patch ok", func() {
			Expect(g).ShouldNot(BeNil())
			Expect(time.Now()).Should(BeEquivalentTo(doom))

			g.Unpatch()
			Expect(time.Now()).ShouldNot(BeEquivalentTo(doom))

			g.Restore()
			Expect(time.Now()).Should(BeEquivalentTo(doom))
		})
	})

	Context("Test Time Patch Indirect", func() {
		var (
			g    *Guard
			doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
		)

		BeforeEach(func() {
			g = PatchIndirect(time.Now, func() time.Time {
				return doom
			})
		})

		AfterEach(func() {
			g.Unpatch()
		})

		It("should patch ok", func() {
			Expect(g).ShouldNot(BeNil())
			Expect(time.Now()).Should(BeEquivalentTo(doom))

			g.Unpatch()
			Expect(time.Now()).ShouldNot(BeEquivalentTo(doom))

			g.Restore()
			Expect(time.Now()).Should(BeEquivalentTo(doom))
		})
	})
})

var _ = Describe("Test Make Func", func() {
	Context("Make Function", func() {
		var (
			ft  reflect.Type
			err error
		)

		BeforeEach(func() {
			pid := os.Getpid()
			ef, _ := elf.Open(fmt.Sprintf("/proc/%d/exe", pid))
			dw, _ := ef.DWARF()
			trees, _ := DwarfTree(dw)

			for name, tree := range trees {
				if strings.HasSuffix(name, "this_is_for_test") {
					ft, err = MakeFunc(tree, dw)
					break
				}
			}
		})

		It("should be called successfully", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ft).ShouldNot(BeZero())

			fn := reflect.MakeFunc(ft, func(args []reflect.Value) (results []reflect.Value) {
				return []reflect.Value{reflect.ValueOf("1024")}
			}).Interface().(func(int) string)
			Expect(fn(1)).Should(BeEquivalentTo("1024"))
		})
	})
})

var _ = Describe("Test Parser", func() {
	Context("Test Simple Parser", func() {
		var (
			point *HijackPoint
		)

		BeforeEach(func() {
			parser := SimpleParser()
			point = parser.Parse("func:this_is_for_test,action:delay,val:10")
		})

		It("should parse successfully", func() {
			Expect(point).ShouldNot(BeNil())
			Expect(point.Func).To(BeEquivalentTo("this_is_for_test"))
			Expect(point.Action).To(BeEquivalentTo("delay"))
			Expect(point.Val).To(BeEquivalentTo("10"))
		})
	})
})
