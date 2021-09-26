package gohijack

import (
	"debug/dwarf"
	"unsafe"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

type (
	Tree struct {
		godwarf.Entry
		typ      godwarf.Type
		Tag      dwarf.Tag
		Offset   dwarf.Offset
		Children []*Tree
	}
)

//go:linkname entryToTreeInternal github.com/go-delve/delve/pkg/dwarf/godwarf.entryToTreeInternal
func entryToTreeInternal(entry *dwarf.Entry) *Tree

//go:linkname loadTreeChildren github.com/go-delve/delve/pkg/dwarf/godwarf.loadTreeChildren
func loadTreeChildren(e *dwarf.Entry, rdr *dwarf.Reader) ([]*Tree, error)

func LoadTree(off dwarf.Offset, dw *dwarf.Data) (*godwarf.Tree, error) {
	rdr := dw.Reader()
	rdr.Seek(off)

	e, err := rdr.Next()
	if err != nil {
		return nil, err
	}
	r := entryToTreeInternal(e)
	r.Children, err = loadTreeChildren(e, rdr)
	if err != nil {
		return nil, err
	}

	tree := (*godwarf.Tree)(unsafe.Pointer(r))
	tree.Children = *(*[]*godwarf.Tree)(unsafe.Pointer(&r.Children))
	return tree, nil
}
