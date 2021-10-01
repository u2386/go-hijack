package gohijack

import (
	"errors"
	"reflect"
	"syscall"
	"unsafe"
)

var ErrTypeUnsupported = errors.New("type unsupported")

type (
	Guard struct {
		target      *reflect.Value
		replacement *reflect.Value
		original    []byte
		patched     []byte
	}

	value struct {
		_   uintptr
		ptr unsafe.Pointer
	}
)

func Patch(target, replacement reflect.Value) *Guard {
	from := target.Pointer()
	to := (uintptr)(getPtr(replacement))

	code := []byte{
		0x68, //push
		byte(to), byte(to >> 8), byte(to >> 16), byte(to >> 24),
		0xc7, 0x44, 0x24, // mov $value, 4%rsp
		0x04, // rsp + 4
		byte(to >> 32), byte(to >> 40), byte(to >> 48), byte(to >> 56),
		0xc3, // retn
	}

	f := RawMemoryAccess(from, len(code))
	original := make([]byte, len(f))
	copy(original, f)
	CopyToLocation(from, code)
	return &Guard{target: &target, replacement: &replacement, original: original, patched: code}
}

func PatchIndirect(target, replacement reflect.Value) *Guard {
	from := target.Pointer()
	to := (uintptr)(getPtr(replacement))

	code := []byte{
		0x48, 0xBA,
		byte(to),
		byte(to >> 8),
		byte(to >> 16),
		byte(to >> 24),
		byte(to >> 32),
		byte(to >> 40),
		byte(to >> 48),
		byte(to >> 56), // movabs rdx,to
		0xFF, 0x22,     // jmp QWORD PTR [rdx]
	}

	f := RawMemoryAccess(from, len(code))
	original := make([]byte, len(f))
	copy(original, f)
	CopyToLocation(from, code)
	return &Guard{target: &target, replacement: &replacement, original: original, patched: code}
}

func (g *Guard) Unpatch() {
	CopyToLocation(g.target.Pointer(), g.original)
}

func (g *Guard) Restore() {
	CopyToLocation(g.target.Pointer(), g.patched)
}

func RawMemoryAccess(p uintptr, length int) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: p,
		Len:  length,
		Cap:  length,
	}))
}

func CopyToLocation(location uintptr, data []byte) {
	f := RawMemoryAccess(location, len(data))

	MprotectCrossPage(location, len(data), syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC)
	copy(f, data[:])
	MprotectCrossPage(location, len(data), syscall.PROT_READ|syscall.PROT_EXEC)
}

func MprotectCrossPage(addr uintptr, length int, prot int) {
	pageSize := syscall.Getpagesize()
	for p := PageStart(addr); p < addr+uintptr(length); p += uintptr(pageSize) {
		page := RawMemoryAccess(p, pageSize)
		if err := syscall.Mprotect(page, prot); err != nil {
			panic(err)
		}
	}
}

func PageStart(ptr uintptr) uintptr {
	return ptr & ^(uintptr(syscall.Getpagesize() - 1))
}

func getPtr(v reflect.Value) uintptr {
	return (uintptr)((*value)(unsafe.Pointer(&v)).ptr)
}
