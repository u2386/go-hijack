package runtime

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

	"bou.ke/monkey"
	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
)

//go:noinline
func this_is_for_test(i int) string { return fmt.Sprint(i) }

var _ = this_is_for_test(0)

//go:noinline
func doomer() time.Time { return doom }

func TestRuntime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runtime Suite")
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

var _ = Describe("Test Patch", func() {
	Context("Test Time Patch by Pointer", func() {
		var (
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
		})

		Context("Test Patch Native Func by Pointer", func() {
			var (
				g *Guard
			)

			BeforeEach(func() {
				g = Patch(doomer, sym.Value)
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

		Context("Test Patch reflect.MakeFunc by Pointer", func() {
			var (
				stub reflect.Value
				g    *Guard
			)
			BeforeEach(func() {
				t := reflect.FuncOf([]reflect.Type{}, []reflect.Type{reflect.TypeOf(time.Time{})}, false)
				stub = reflect.MakeFunc(t, nil)
				g = Patch(stub.Pointer(), sym.Value)
			})

			AfterEach(func() {
				g.Unpatch()
			})

			It("should patch ok", func() {
				Expect(g).ShouldNot(BeNil())

				rv := stub.Call(nil)
				_, ok := rv[0].Interface().(time.Time)
				Expect(ok).Should(BeTrue())
			})
		})

		Context("Test Patch Function Pointer by reflect.MakeFunc", func() {
			var (
				g    *Guard
			)
			BeforeEach(func() {
				t := reflect.FuncOf([]reflect.Type{}, []reflect.Type{reflect.TypeOf(time.Time{})}, false)
				stub := reflect.MakeFunc(t, func(args []reflect.Value) (results []reflect.Value) {
					return []reflect.Value{reflect.ValueOf(doom)}
				}).Interface()
				g = Patch(sym.Value, stub)
			})

			AfterEach(func() {
				g.Unpatch()
			})

			It("should patch ok", func() {
				Expect(g).ShouldNot(BeNil())

				t := time.Now()
				Expect(t).Should(BeEquivalentTo(doom))
			})
		})

		Context("Test Patch Function Pointer by Native Function", func() {
			var (
				g    *Guard
				doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
			)

			BeforeEach(func() {
				g = Patch(sym.Value, func() time.Time {
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

		Context("Test Patch Native Function By Function", func() {
			var (
				g    *Guard
				doom = time.Date(2012, time.December, 21, 0, 0, 0, 0, time.UTC)
			)

			BeforeEach(func() {
				g = Patch(time.Now, func() time.Time {
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

var _ = Describe("Test Hijack Runtime", func() {
	Context("Test Run", func() {
		var (
			r *Runtime
		)

		BeforeEach(func() {
			r = New()

			ctx, cancel := context.WithCancel(context.Background())
			go r.Run(ctx)
			r.C <- func() {}
			time.Sleep(time.Second)
			cancel()
		})

		It("should exit", func() {
			time.Sleep(time.Millisecond)
			Expect(r.C).To(BeClosed())
		})
	})

	Context("Test Hijack", func() {
		var (
			r       *Runtime
			ctx     context.Context
			cancel  context.CancelFunc
			patched bool
			err     error
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			r = New()
			r.patches["u2386"] = func(*HijackPoint) (*Guard, error) { patched = true; return nil, nil }
			go r.Run(ctx)
			err = r.Hijack(&HijackPoint{Action: "u2386"})
		})

		AfterEach(func() {
			cancel()
		})

		It("should patch successfully", func() {
			Expect(err).To(BeNil())
			Expect(patched).To(BeTrue())
		})

		It("should return error", func() {
			Expect(r.Hijack(&HijackPoint{Action: "unknown"})).To(BeEquivalentTo(ErrUnsupportAction))
		})
	})

	Context("Test Release Hijack Point", func() {
		var (
			r         *Runtime
			ctx       context.Context
			cancel    context.CancelFunc
			pg        *monkey.PatchGuard
			unpatched bool
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			var g *Guard
			pg = monkey.PatchInstanceMethod(reflect.TypeOf(g), "Unpatch", func(*Guard) { unpatched = true })

			r = New()
			r.M.Store("u2386", g)
			go r.Run(ctx)

		})

		AfterEach(func() {
			pg.Unpatch()
			cancel()
		})

		It("should release successfully", func() {
			r.Release("unknown")
			Expect(unpatched).To(BeFalse())

			r.Release("u2386")
			Expect(unpatched).To(BeTrue())
		})
	})
})
