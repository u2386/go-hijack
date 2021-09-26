package gohijack

import (
	"debug/dwarf"
	"debug/elf"
	"fmt"
	"os"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	dwarfreader "github.com/go-delve/delve/pkg/dwarf/reader"
)

const (
	UDSAddress = "/tmp/gohijack.sock"
)

type hijack struct {
	table map[string]*godwarf.Tree
}

func (r *hijack) Run() {}

func Open(path string) (*dwarf.Data, error) {
	e, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	return e.DWARF()
}

func FuncTable(dw *dwarf.Data) (map[string]*godwarf.Tree, error) {
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

func Hijack() error {
	dw, err := Open(fmt.Sprintf("/proc/%d/exe", os.Getpid()))
	if err != nil {
		return err
	}
	tab, err := FuncTable(dw)
	if err != nil {
		return err
	}

	r := &hijack{
		table: tab,
	}
	go r.Run()
	return nil
}