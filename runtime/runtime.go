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

func New() *Runtime {
	r := &Runtime{}
	r.M = sync.Map{}
	r.C = make(chan func(), 1)
	r.patches = map[Action]ActionFunc{
		DELAY: r.delay,
	}
	return r
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

func (r *Runtime) Release(fn string) {
	r.M.Range(func(key, value interface{}) bool {
		if strings.EqualFold(fn, key.(string)) {
			if v, ok := r.M.LoadAndDelete(key); ok {
				v.(*Guard).Unpatch()
			}
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
	millsecs, ok := point.Val.(uint)
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

	stub := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) { return })
	replacement := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		time.Sleep(time.Millisecond * time.Duration(millsecs))
		guard.Unpatch()
		defer guard.Restore()

		g := Patch(stub, symbol.Value)
		defer g.Unpatch()
		return stub.Call(args)
	})

	guard = PatchIndirect(symbol.Value, replacement)
	return guard, nil
}
