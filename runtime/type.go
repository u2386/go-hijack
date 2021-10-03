package runtime

import (
	"debug/dwarf"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
)

var (
	ErrUnsupportedType = errors.New("unsupported type")
	typeCache          = make(map[dwarf.Offset]godwarf.Type)
)

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
