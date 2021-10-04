package runtime

import (
	"context"
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

type (
	HijackPoint struct {
		Func   string
		Action Action
		Val    interface{}
	}

	Runtime struct {
		M          sync.Map
		C          chan func()
		patches    map[Action]ActionFunc
		dwarftrees map[string]*godwarf.Tree
		symbols    map[string]elf.Symbol
		dwarf      *dwarf.Data
	}

	Action     string
	ActionFunc func(*HijackPoint) (*Guard, error)
)

const (
	DELAY Action = "delay"
)

var (
	DEBUG              = ""
	ErrUnsupportAction = errors.New("unsupport action")
	ErrPointNotFound   = errors.New("function point not found")
	ErrPatchedAlready  = errors.New("patched already")
)

func debug(format string, args ...interface{}) {
	if DEBUG != "" {
		fmt.Fprintf(os.Stderr, "GOHIJACK: "+format+"\n", args...)
	}
}

func New(pid int) (*Runtime, error) {
	r := &Runtime{}
	r.M = sync.Map{}
	r.C = make(chan func(), 1)
	r.symbols = make(map[string]elf.Symbol)
	r.patches = map[Action]ActionFunc{
		DELAY: r.delay,
	}

	ef, err := elf.Open(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return nil, err
	}

	syms, err := ef.Symbols()
	if err != nil {
		return nil, err
	}
	for _, sym := range syms {
		r.symbols[sym.Name] = sym
	}

	r.dwarf, err = ef.DWARF()
	if err != nil {
		return nil, err
	}
	r.dwarftrees, err = DwarfTree(r.dwarf)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Runtime) Run(ctx context.Context) {
	go func() {
		runtime.LockOSThread()
		defer close(r.C)

		for {
			select {
			case <-ctx.Done():
				return
			case fn := <-r.C:
				fn()
			}
		}
	}()
}

func (r *Runtime) Funcs() []string {
	var ns []string
	for sym := range r.symbols {
		ns = append(ns, sym)
	}
	return ns
}

func (r *Runtime) Points() []string {
	var ns []string
	r.M.Range(func(key, value interface{}) bool {
		ns = append(ns, key.(string))
		return true
	})
	return ns
}

func (r *Runtime) Release(fn string) {
	r.M.Range(func(key, value interface{}) bool {
		if strings.EqualFold(fn, key.(string)) {
			value.(*Guard).Unpatch()
			r.M.Delete(key)
			return false
		}
		return true
	})
}

func (r *Runtime) Hijack(point *HijackPoint) error {
	if patch, ok := r.patches[point.Action]; ok {
		if _, ok := r.M.Load(point.Func); ok {
			return ErrPatchedAlready
		}

		c := make(chan error, 1)
		r.C <- func() { g, err := patch(point); r.M.Store(point.Func, g); c <- err }
		return <-c
	}
	return ErrUnsupportAction
}

func (r *Runtime) delay(point *HijackPoint) (*Guard, error) {
	millsecs, ok := point.Val.(int)
	if !ok {
		return nil, ErrUnsupportAction
	}

	var (
		node   *godwarf.Tree
		symbol elf.Symbol
		guard  *Guard
	)
	if node, ok = r.dwarftrees[point.Func]; !ok {
		return nil, ErrPointNotFound
	}
	if symbol, ok = r.symbols[point.Func]; !ok {
		return nil, ErrPointNotFound
	}

	typ, err := MakeFunc(node, r.dwarf)
	if err != nil {
		return nil, err
	}

	stub := reflect.MakeFunc(typ, nil)
	replacement := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		time.Sleep(time.Millisecond * time.Duration(millsecs))
		guard.Unpatch()
		defer guard.Restore()

		g := Patch(stub.Pointer(), symbol.Value)
		defer g.Unpatch()
		return stub.Call(args)
	})

	guard = Patch(symbol.Value, replacement.Interface())
	return guard, nil
}
