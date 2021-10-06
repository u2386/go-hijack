package runtime

import (
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:noinline
func test_struct_argument(r *http.Request) { _ = fmt.Sprint(r) }

var _ = Describe("Test Make Type", func() {
	test_struct_argument(&http.Request{})

	find := func(trees map[string]*godwarf.Tree, s string) *godwarf.Tree {
		for k, v := range trees {
			if strings.HasSuffix(k, s) {
				return v
			}
		}
		return nil
	}

	Context("Test MakeType", func() {
		var (
			dw    *dwarf.Data
			trees map[string]*godwarf.Tree
		)

		BeforeEach(func() {
			pid := os.Getpid()
			ef, _ := elf.Open(fmt.Sprintf("/proc/%d/exe", pid))
			dw, _ = ef.DWARF()
			trees, _ = DwarfTree(dw)
		})

		It("tests struct argument function", func() {
			Skip("Recursive Struct")

			node := find(trees, "test_struct_argument")
			Expect(node).NotTo(BeNil())

			_, err := MakeFunc(node, dw)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
