package runtime

import (
	"debug/dwarf"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unsafe"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

var (
	ErrUnsupportedType = errors.New("unsupported type")
	typeCache          = make(map[dwarf.Offset]godwarf.Type)
	FuncReturnRegexp   = regexp.MustCompile(`^func\(.*?\)(?P<Return>.+)$`)
)

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

func mapOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	t := typ.(*godwarf.MapType)

	kt, err := MakeType(t.KeyType, dw)
	if err != nil {
		return nil, err
	}
	vt, err := MakeType(t.ElemType, dw)
	if err != nil {
		return nil, err
	}
	return reflect.MapOf(kt, vt), nil
}

func sliceOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	t := typ.(*godwarf.SliceType)
	et, err := MakeType(t.ElemType, dw)
	if err != nil {
		return nil, err
	}
	return reflect.SliceOf(et), nil
}

func chanOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	t := typ.(*godwarf.ChanType)
	et, err := MakeType(t.ElemType, dw)
	if err != nil {
		return nil, err
	}
	return reflect.ChanOf(reflect.BothDir, et), nil
}

func funcOf(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	t := typ.(*godwarf.FuncType)

	// FIXME: We cannot distinguish parameters and return values from FuncType, so
	// we use regex to count function returns.
	// See issue: https://github.com/golang/go/issues/48812
	var count int
	if m := FuncReturnRegexp.FindStringSubmatch(t.Name); m != nil {
		count = len(strings.Split(m[1], ","))
	}

	pt := t.ParamType[:len(t.ParamType)-count]
	rt := t.ParamType[len(pt):]

	var (
		in  []reflect.Type
		out []reflect.Type
	)
	for _, param := range pt {
		v, err := MakeType(param, dw)
		if err != nil {
			return nil, err
		}
		in = append(in, v)
	}

	for _, ret := range rt {
		v, err := MakeType(ret.(*godwarf.PtrType).Type, dw)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}

	return reflect.FuncOf(in, out, false), nil
}

func MakeType(typ godwarf.Type, dw *dwarf.Data) (reflect.Type, error) {
	switch t := typ.(type) {
	case *godwarf.TypedefType:
		return MakeType(t.Type, dw)

	case *godwarf.PtrType:
		if t.Name == "unsafe.Pointer" {
			var ifunc interface{} = (func())(nil)
			return reflect.TypeOf(unsafe.Pointer(&ifunc)), nil
		}
		rt, err := MakeType(t.Type, dw)
		if err != nil {
			return nil, err
		}
		return reflect.PtrTo(rt), nil

	case *godwarf.StructType:
		return structOf(t, dw)

	case *godwarf.StringType:
		return reflect.TypeOf(""), nil

	case *godwarf.IntType:
		return reflect.TypeOf(0), nil

	case *godwarf.BoolType:
		return reflect.TypeOf(false), nil

	case *godwarf.MapType:
		return mapOf(t, dw)

	case *godwarf.SliceType:
		return sliceOf(t, dw)

	case *godwarf.UintType:
		return reflect.TypeOf(uint64(0)), nil

	case *godwarf.FuncType:
		return funcOf(t, dw)

	case *godwarf.InterfaceType:
		return reflect.TypeOf((*interface{})(nil)).Elem(), nil

	case *godwarf.ChanType:
		return chanOf(t, dw)

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, t.String())
	}
}

func MakeFunc(tree *godwarf.Tree, dw *dwarf.Data) (reflect.Type, error) {
	var (
		in  []reflect.Type
		out []reflect.Type

		args    []string
		returns []string
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
			returns = append(returns, fmt.Sprintf("%s %s", node.Entry.Val(dwarf.AttrName), typ.String()))
		} else {
			in = append(in, param)
			args = append(args, fmt.Sprintf("%s %s", node.Entry.Val(dwarf.AttrName), typ.String()))
		}
	}

	var sb strings.Builder
	sb.WriteString(tree.Entry.Val(dwarf.AttrName).(string) + "(" + strings.Join(args, ", ") + ")")
	if len(returns) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(returns, ", "))
		sb.WriteRune(')')
	}
	debug("%s", sb.String())

	return reflect.FuncOf(in, out, false), nil
}
