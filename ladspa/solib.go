package ladspa

// #cgo LDFLAGS: -ldl
// #include <stdlib.h>
// #include <dlfcn.h>
import "C"

import (
	"fmt"
	"unsafe"
)

type SoLib struct {
	p unsafe.Pointer
}

func NewSoLib(lib string) (*SoLib, error) {
	libname := C.CString(lib)
	defer C.free(unsafe.Pointer(libname))
	p := C.dlopen(libname, C.RTLD_LAZY)
	if p == nil {
		return nil, dlerror()
	}
	return &SoLib{p}, nil
}

func (l *SoLib) Symbol(name string) (unsafe.Pointer, error) {
	sym := C.CString(name)
	defer C.free(unsafe.Pointer(sym))
	C.dlerror()
	if p := C.dlsym(l.p, sym); p != nil {
		return p, nil
	}
	return nil, dlerror()
}

func (l *SoLib) Close() error {
	C.dlerror()
	C.dlclose(l.p)
	return dlerror()
}

func dlerror() error {
	if s := C.dlerror(); s != nil {
		return fmt.Errorf("%s", C.GoString(s))
	}
	return nil
}
