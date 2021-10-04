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
	"github.com/mitchellh/mapstructure"
)

type (
	Request map[string]interface{}

	HijackPoint struct {
		Func   string
		Action Action
	}

	DelayPoint struct {
		HijackPoint `mapstructure:",squash"`
		Val         int
	}

	PanicPoint struct {
		HijackPoint `mapstructure:",squash"`
		Val         string
	}

	SetPoint struct {
		HijackPoint `mapstructure:",squash"`
		Index       int
		Val         interface{}
	}

	ReturnPoint struct {
		HijackPoint `mapstructure:",squash"`
		Index       int
		Val         interface{}
	}

	Runtime struct {
		M          sync.Map
		C          chan func()
		patches    map[Action]ActionFunc
		dwarftrees map[string]*godwarf.Tree
		symbols    map[string]elf.Symbol
		dwarf      *dwarf.Data
	}

	patcher struct{}

	Action     string
	ActionFunc func(*Runtime, Request) (*Guard, error)
)

const (
	DELAY  Action = "delay"
	PANIC  Action = "panic"
	SET    Action = "set"
	RETURN Action = "return"
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

	pat := &patcher{}
	r.patches = map[Action]ActionFunc{
		DELAY:  pat.Delay,
		PANIC:  pat.Panic,
		SET:    pat.Set,
		RETURN: pat.Return,
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

func (r *Runtime) Hijack(m Request) error {
	var point HijackPoint
	mapstructure.Decode(m, &point)
	if patch, ok := r.patches[point.Action]; ok {
		if _, ok := r.M.Load(point.Func); ok {
			return ErrPatchedAlready
		}

		c := make(chan error, 1)
		r.C <- func() {
			if g, err := patch(r, m); err == nil {
				r.M.Store(point.Func, g)
				c <- nil
			} else {
				c <- err
			}
		}
		return <-c
	}
	return ErrUnsupportAction
}

func (*patcher) Delay(r *Runtime, m Request) (*Guard, error) {
	var point DelayPoint
	mapstructure.Decode(m, &point)

	var ok bool
	if point.Val <= 0 {
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
		time.Sleep(time.Millisecond * time.Duration(point.Val))
		guard.Unpatch()
		defer guard.Restore()

		g := Patch(stub.Pointer(), symbol.Value)
		defer g.Unpatch()
		return stub.Call(args)
	})

	guard = Patch(symbol.Value, replacement.Interface())
	return guard, nil
}

func (*patcher) Panic(r *Runtime, m Request) (*Guard, error) {
	var point PanicPoint
	mapstructure.Decode(m, &point)

	node, ok := r.dwarftrees[point.Func]
	if !ok {
		return nil, ErrPointNotFound
	}
	symbol, ok := r.symbols[point.Func]
	if !ok {
		return nil, ErrPointNotFound
	}

	typ, err := MakeFunc(node, r.dwarf)
	if err != nil {
		return nil, err
	}

	var guard *Guard
	replacement := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		panic(fmt.Sprintf("hijack:%s", point.Val))
	})

	guard = Patch(symbol.Value, replacement.Interface())
	return guard, nil
}

func (*patcher) Set(r *Runtime, m Request) (*Guard, error) {
	var point SetPoint
	mapstructure.Decode(m, &point)

	node, ok := r.dwarftrees[point.Func]
	if !ok {
		return nil, ErrPointNotFound
	}
	symbol, ok := r.symbols[point.Func]
	if !ok {
		return nil, ErrPointNotFound
	}

	typ, err := MakeFunc(node, r.dwarf)
	if err != nil {
		return nil, err
	}

	var guard *Guard
	stub := reflect.MakeFunc(typ, nil)
	replacement := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		args[point.Index] = reflect.ValueOf(point.Val)

		guard.Unpatch()
		defer guard.Restore()

		g := Patch(stub.Pointer(), symbol.Value)
		defer g.Unpatch()

		return stub.Call(args)
	})

	guard = Patch(symbol.Value, replacement.Interface())
	return guard, nil
}

func (*patcher) Return(r *Runtime, m Request) (*Guard, error) {
	var point SetPoint
	mapstructure.Decode(m, &point)

	node, ok := r.dwarftrees[point.Func]
	if !ok {
		return nil, ErrPointNotFound
	}
	symbol, ok := r.symbols[point.Func]
	if !ok {
		return nil, ErrPointNotFound
	}

	typ, err := MakeFunc(node, r.dwarf)
	if err != nil {
		return nil, err
	}

	var guard *Guard
	stub := reflect.MakeFunc(typ, nil)
	replacement := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
		guard.Unpatch()
		defer guard.Restore()

		g := Patch(stub.Pointer(), symbol.Value)
		defer g.Unpatch()

		results = stub.Call(args)
		results[point.Index] = reflect.ValueOf(point.Val)
		return
	})

	guard = Patch(symbol.Value, replacement.Interface())
	return guard, nil
}
