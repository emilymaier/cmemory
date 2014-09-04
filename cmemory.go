// Package cmemory contains tools for C memory allocation.
package cmemory

/*
#include <stdlib.h>
#cgo LDFLAGS: -ldl

void start_instrumentation();
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"unsafe"
)

// Memory contains a single block of memory allocated on the C heap.
type Memory struct {
	Cbuf   unsafe.Pointer
	Size   uint64
	gobuf  []byte
	cursor uint64
}

// Alloc creates a new Memory struct and allocates on the C heap for it.
func Alloc(size uint64) (*Memory, error) {
	newMemory := new(Memory)
	newMemory.Cbuf = C.malloc(C.size_t(size))
	if newMemory.Cbuf == nil {
		return newMemory, errors.New("malloc() could not allocate memory")
	}
	runtime.SetFinalizer(newMemory, finalizeMemory)
	newMemory.Size = size
	newMemory.gobuf = *(*[]byte)(unsafe.Pointer(&newMemory.Cbuf))
	return newMemory, nil
}

// WrapMemory creates a new Memory struct from an existing pointer to a C
// memory block and its size.
func WrapMemory(cbuf unsafe.Pointer, size uint64) *Memory {
	newMemory := new(Memory)
	newMemory.Cbuf = cbuf
	runtime.SetFinalizer(newMemory, finalizeMemory)
	newMemory.Size = size
	newMemory.gobuf = *(*[]byte)(unsafe.Pointer(&newMemory.Cbuf))
	return newMemory
}

func finalizeMemory(deadMemory *Memory) {
	C.free(deadMemory.Cbuf)
}

// Read implements the io.Reader interface to read from the memory block.
func (this *Memory) Read(output []byte) (int, error) {
	if this.cursor == this.Size {
		return 0, io.EOF
	}
	var newCursor uint64
	if this.cursor+uint64(len(output)) > this.Size {
		newCursor = this.Size
	} else {
		newCursor = this.cursor + uint64(len(output))
	}
	bytesRead := copy(output, this.gobuf[this.cursor:newCursor])
	this.cursor = newCursor
	return bytesRead, nil
}

// Write implements the io.Writer interface to write to the memory block.
func (this *Memory) Write(input []byte) (int, error) {
	if this.cursor == this.Size {
		return 0, io.EOF
	}
	var newCursor uint64
	if this.cursor+uint64(len(input)) > this.Size {
		newCursor = this.Size
	} else {
		newCursor = this.cursor + uint64(len(input))
	}
	bytesWritten := copy(this.gobuf[this.cursor:newCursor], input)
	this.cursor = newCursor
	return bytesWritten, nil
}

// Seek implements the io.Seeker interface to seek through the memory block.
func (this *Memory) Seek(offset int64, whence int) (int64, error) {
	var newCursor int64
	switch {
	case whence == 0:
		newCursor = offset
	case whence == 1:
		newCursor = int64(this.cursor) + offset
	case whence == 2:
		newCursor = int64(this.Size) + offset
	default:
		return int64(this.cursor), errors.New("Invalid whence parameter")
	}
	if newCursor < 0 {
		return int64(this.cursor), errors.New("Attempted to seek to a negative offset")
	}
	if newCursor > int64(this.Size) {
		newCursor = int64(this.Size)
	}
	this.cursor = uint64(newCursor)
	return int64(this.cursor), nil
}

type subBlock struct {
	address unsafe.Pointer
	size    uint64
}

type block struct {
	trace     string
	goStack   []uintptr
	subBlocks map[unsafe.Pointer]subBlock
}

func (this *block) size() uint64 {
	var ret uint64
	for _, subBlock := range this.subBlocks {
		ret += subBlock.size
	}
	return ret
}

func (this *block) print() {
	fmt.Printf("%d block(s) of total size %d were allocated at:\n%s\n", len(this.subBlocks), this.size(), this.trace)
}

var blocks map[string]*block = make(map[string]*block)
var addresses map[unsafe.Pointer]*block = make(map[unsafe.Pointer]*block)
var bytesAllocated uint64
var bytesFreed uint64

// StartInstrumentation begins recording all C memory allocations and frees.
func StartInstrumentation() {
	C.start_instrumentation()
}

//export instrumentMalloc
func instrumentMalloc(address unsafe.Pointer, size C.size_t, cTrace unsafe.Pointer, cFrames C.int) {
	var trace string
	for cFrame := 2; cFrame < int(cFrames)-2; cFrame++ {
		trace += C.GoString((*(*[]*C.char)(unsafe.Pointer(&cTrace)))[cFrame])
		trace += "\n"
	}
	var skip int = 1
	var inC bool = true
	goStack := make([]uintptr, 0)
	for {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		var funcName string = runtime.FuncForPC(pc).Name()
		if strings.HasPrefix(funcName, "runtime.") {
			if !inC {
				inC = true
				trace += "C code\n"
			}
			skip++
			continue
		}
		inC = false
		trace += runtime.FuncForPC(pc).Name() + "\n"
		trace += fmt.Sprintf("\t%s:%d (0x%x)\n", file, line, pc)
		goStack = append(goStack, pc)
		skip++
	}
	if curBlock, ok := blocks[trace]; ok {
		if subBlock, ok := curBlock.subBlocks[address]; ok {
			bytesFreed += subBlock.size
		}
	} else {
		blocks[trace] = new(block)
		blocks[trace].trace = trace
		blocks[trace].goStack = goStack
		blocks[trace].subBlocks = make(map[unsafe.Pointer]subBlock)
	}
	blocks[trace].subBlocks[address] = subBlock{address, uint64(size)}
	addresses[address] = blocks[trace]
	bytesAllocated += uint64(size)
}

//export instrumentFree
func instrumentFree(address unsafe.Pointer) {
	if block, ok := addresses[address]; ok {
		if subBlock, ok := block.subBlocks[address]; ok {
			bytesFreed += subBlock.size
			delete(addresses[address].subBlocks, address)
			return
		}
	}
}

// Stats contains information about C memory allocations that were recorded
// after StartInstrumentation.
type Stats struct {
	CurAllocations      uint64
	CurBytesAllocated   uint64
	TotalBytesAllocated uint64
	BytesFreed          uint64
}

// Print prints out the human-readable stats contained in the Stats struct to
// stdout.
func (this *Stats) Print() {
	fmt.Printf("Current number of allocations: %d\n", this.CurAllocations)
	fmt.Printf("Current number of bytes allocated: %d\n", this.CurBytesAllocated)
	fmt.Printf("Total number of bytes allocated: %d\n", this.TotalBytesAllocated)
	fmt.Printf("Number of bytes freed: %d\n", this.BytesFreed)
}

// MemoryAnalysis creates a new Stats struct from the current C heap
// information.
func MemoryAnalysis() Stats {
	ret := Stats{}
	for _, curBlock := range blocks {
		ret.CurAllocations += uint64(len(curBlock.subBlocks))
		ret.CurBytesAllocated += curBlock.size()
	}
	ret.TotalBytesAllocated = bytesAllocated
	ret.BytesFreed = bytesFreed
	return ret
}

// MemoryDump writes out a pprof-compatible profile of the C heap to the output
// parameter.
func MemoryDump(output io.Writer) error {
	stats := MemoryAnalysis()
	_, err := fmt.Fprintf(output, "heap profile: %d: %d [%d: %d] @ heapprofile\n", stats.CurAllocations, stats.CurBytesAllocated, stats.CurAllocations, stats.CurBytesAllocated)
	if err != nil {
		return err
	}
	for _, curBlock := range blocks {
		_, err := fmt.Fprintf(output, "%d: %d [%d: %d] @", len(curBlock.subBlocks), curBlock.size(), len(curBlock.subBlocks), curBlock.size())
		if err != nil {
			return err
		}
		for _, address := range curBlock.goStack {
			_, err := fmt.Fprintf(output, " 0x%x", address)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(output)
		if err != nil {
			return err
		}
	}
	return nil
}
