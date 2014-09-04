// +build test

package cmemory

/*
#include <mcheck.h>
#cgo LDFLAGS: -lmcheck
*/
import "C"

import "unsafe"

func isConsistent(buf unsafe.Pointer) bool {
	return C.mprobe(buf) == C.MCHECK_OK
}
