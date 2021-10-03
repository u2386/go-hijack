package gohijack

import (
	"context"
	"debug/elf"
	"fmt"
	"os"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	"github.com/u2386/go-hijack/runtime"
)

const (
	UDSAddress = "/tmp/gohijack.sock"
)

type (
	hijack struct {
		dwarftrees map[string]*godwarf.Tree
		symbols    map[string]elf.Symbol
	}
)

var DEBUG = ""

func debug(format string, args ...interface{}) {
	if DEBUG != "" {
		fmt.Fprintf(os.Stderr, "GOHIJACK: "+format+"\n", args...)
	}
}

func critical(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "GOHIJACK: "+format+"\n", args...)
}

func Hijack(ctx context.Context) error {
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
	r.dwarftrees, err = runtime.DwarfTree(dw)
	if err != nil {
		return err
	}
	return nil
}
