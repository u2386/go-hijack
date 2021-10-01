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

func debug(format string, args ...interface{}) {
	if DEBUG != "" {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

var DEBUG = ""

type hijack struct {
	dwarftrees map[string]*godwarf.Tree
	symbols    map[string]elf.Symbol
}

func (r *hijack) Run() {}

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

func Hijack() error {
	ef, err := elf.Open(fmt.Sprintf("/proc/%d/exe", os.Getpid()))
	if err != nil {
		return err
	}

	r := &hijack{
		dwarftrees: make(map[string]*godwarf.Tree),
		symbols:    make(map[string]elf.Symbol),
	}

	syms, err := ef.Symbols()
	if err != nil {
		return err
	}
	for _, sym := range syms {
		r.symbols[sym.Name] = sym
	}

	dw, err := ef.DWARF()
	if err != nil {
		return err
	}
	r.dwarftrees, err = DwarfTree(dw)
	if err != nil {
		return err
	}

	go r.Run()
	return nil
}
