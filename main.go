package main

import (
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unsafe"
	_ "unsafe"

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

var (
	ErrUnsupportedType = errors.New("unsupported type")
	typeCache          = make(map[dwarf.Offset]godwarf.Type)
)

//go:linkname entryToTreeInternal github.com/go-delve/delve/pkg/dwarf/godwarf.entryToTreeInternal
func entryToTreeInternal(entry *dwarf.Entry) *Tree

//go:linkname loadTreeChildren github.com/go-delve/delve/pkg/dwarf/godwarf.loadTreeChildren
func loadTreeChildren(e *dwarf.Entry, rdr *dwarf.Reader) ([]*Tree, error)

func LoadTree(off dwarf.Offset, dw *dwarf.Data) (*Tree, error) {
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
	return r, nil
}

func stringOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	return reflect.TypeOf(""), nil
}

func intOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	return reflect.TypeOf(0), nil
}

func structOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	t := typ.(*godwarf.StructType)
	var fields []reflect.StructField
	for _, field := range t.Field {
		rt, err := MakeType(field.Type, dw)
		if err != nil {
			return nil, err
		}
		fields = append(fields, reflect.StructField{
			Name: strings.Title(field.Name),
			Type: rt,
		})
	}
	return reflect.StructOf(fields), nil
}

func MakeType(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	var tree *godwarf.Tree
	{
		t, err := LoadTree(typ.Common().Offset, dw)
		if err != nil {
			return nil, err
		}
		tree = (*godwarf.Tree)(unsafe.Pointer(t))
		tree.Children = *(*[]*godwarf.Tree)(unsafe.Pointer(&t.Children))
	}

	switch t := typ.(type) {
	case *godwarf.TypedefType:
		return MakeType(t.Type, dw)

	case *godwarf.PtrType:
		rt, err := MakeType(t.Type, dw)
		if err != nil {
			return nil, err
		}
		return reflect.PtrTo(rt), nil

	case *godwarf.StructType:
		return structOf(t, dw)

	case *godwarf.StringType:
		return stringOf(t, dw)

	case *godwarf.IntType:
		return intOf(t, dw)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, t.String())
	}
}

func MakeFunc(tree *godwarf.Tree, dw *dwarf.Data) (reflect.Type, error) {
	var (
		in []reflect.Type
		out []reflect.Type
	)

	for _, node := range tree.Children {
		if node.Tag != dwarf.TagFormalParameter {
			continue
		}

		typ, err := node.Type(dw, int(node.Offset), typeCache)
		if err != nil {
			return nil, err
		}

		param, err := MakeType(typ, dw)
		if err != nil {
			return nil, err
		}

		if node.Entry.Val(dwarf.AttrVarParam).(bool) {
			out = append(out, param)
			continue
		}
		in = append(in, param)
	}
	return reflect.FuncOf(in, out, false), nil
}

func main() {
	if len(os.Args[1]) == 0 {
		fmt.Fprintf(os.Stderr, "binary expected\n")
		os.Exit(1)
	}
	out := os.Stdout

	var dw *dwarf.Data
	{
		e, err := elf.Open(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "get elf failed:%+v\n", err)
			panic(err)
		}
		dw, err = e.DWARF()
		if err != nil {
			fmt.Fprintf(os.Stderr, "get dwarf failed:%+v\n", err)
			panic(err)
		}
	}

	reader := dwarfreader.New(dw)
	for entry, err := reader.Next(); entry != nil; entry, err = reader.Next() {
		if err != nil {
			fmt.Fprintf(out, "iterate entries error:%#v\n", err)
			return
		}

		if entry.Tag != dwarf.TagSubprogram {
			continue
		}

		var tree *godwarf.Tree
		{
			t, err := LoadTree(entry.Offset, dw)
			if err != nil {
				fmt.Fprintf(out, "load tree error:%#v\n", err)
				return
			}
			tree = (*godwarf.Tree)(unsafe.Pointer(t))
			tree.Children = *(*[]*godwarf.Tree)(unsafe.Pointer(&t.Children))
		}

		name := tree.Entry.Val(dwarf.AttrName)
		if name != "main.(*simpleStruct).String" {
			continue
		}

		typ, err := MakeFunc(tree, dw)
		if err != nil {
			fmt.Fprintf(out, "make function error:%#v\n", err)
			return
		}

		f := reflect.MakeFunc(typ, func(args []reflect.Value) (results []reflect.Value) {
			fmt.Println(args)
			return []reflect.Value{reflect.ValueOf("good")}
		}).Interface().(func(*struct{Name string; Age int}) string)

		fmt.Println(f(nil))
	}
}
