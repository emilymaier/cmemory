// +build test

package cmemory

// Functions used during testing. Since test files cannot use cgo, these have to
// be put in the main package. They are only included when building with the
// test flag.

/*
#include <mcheck.h>
#cgo LDFLAGS: -lmcheck
*/
import "C"

import "unsafe"

// Checks if the memory block is allocated and consistent, according to glibc's
// memory debugging.
func isConsistent(buf unsafe.Pointer) bool {
	return C.mprobe(buf) == C.MCHECK_OK
}

// Calls C's malloc() function. If the package is working correctly, it should
// be our malloc().
func testMalloc(size uint64) unsafe.Pointer {
	return C.malloc(C.size_t(size))
}
