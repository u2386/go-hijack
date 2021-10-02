package gohijack

import (
	"debug/dwarf"
	"unsafe"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	dwarfreader "github.com/go-delve/delve/pkg/dwarf/reader"
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

func DwarfTree(dw *dwarf.Data) (map[string]*godwarf.Tree, error) {
	reader := dwarfreader.New(dw)

	ts := make(map[string]*godwarf.Tree)
	for entry, err := reader.Next(); entry != nil; entry, err = reader.Next() {
		if err != nil {
			return nil, err
		}

		if entry.Tag != dwarf.TagSubprogram {
			continue
		}

		tree, err := LoadTree(entry.Offset, dw)
		if err != nil {
			return nil, err
		}

		if name, ok := tree.Entry.Val(dwarf.AttrName).(string); ok {
			ts[name] = tree
		}
	}
	return ts, nil
}
