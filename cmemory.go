// Copyright © 2014 Emily Maier

/*
Tools for managing C memory allocations.

A working example is in the example/ directory. More information can be found in
the README.md file.
*/
package cmemory

/*
#include <stdlib.h>
#cgo LDFLAGS: -ldl

void start_instrumentation();
void stop_instrumentation();
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

var ErrInvalidWhence = errors.New("Invalid whence parameter")
var ErrNegativeOffset = errors.New("Attempted to seek to a negative offset")

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

// AllocFromSlice creates a new Memory struct from an already existing byte
// slice. The data in the slice is copied into the C heap.
func AllocFromSlice(data []byte) (*Memory, error) {
	newMemory := new(Memory)
	newMemory.Cbuf = C.malloc(C.size_t(len(data)))
	if newMemory.Cbuf == nil {
		return newMemory, errors.New("malloc() could not allocate memory")
	}
	runtime.SetFinalizer(newMemory, finalizeMemory)
	newMemory.Size = uint64(len(data))
	newMemory.gobuf = *(*[]byte)(unsafe.Pointer(&newMemory.Cbuf))
	copy(newMemory.gobuf, data)
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

// Grow increases the size of the buffer.
func (this *Memory) Grow(size uint64) error {
	this.Cbuf = C.realloc(this.Cbuf, C.size_t(size))
	if this.Cbuf == nil {
		return errors.New("realloc() could not allocate memory")
	}
	this.Size = size
	this.gobuf = *(*[]byte)(unsafe.Pointer(&this.Cbuf))
	return nil
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

// ReadByte implements the io.ByteReader interface to read a byte from the memory block.
func (this *Memory) ReadByte() (byte, error) {
	if this.cursor == this.Size {
		return 0, io.EOF
	}
	this.cursor += 1
	return this.gobuf[this.cursor-1], nil
}

// UnreadByte implements the io.ByteScanner interface to unread a byte from the memory block.
func (this *Memory) UnreadByte() error {
	if this.cursor == 0 {
		return io.EOF
	}
	this.cursor -= 1
	return nil
}

// ReadAt implements the io.ReaderAt interface to read from the memory block at an offset.
func (this *Memory) ReadAt(output []byte, offset int64) (int, error) {
	if offset >= int64(this.Size) {
		return 0, io.EOF
	}
	return copy(output, this.gobuf[offset:this.Size]), nil
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

// WriteByte implements the io.ByteWriter interface to write a byte to the memory block.
func (this *Memory) WriteByte(input byte) error {
	if this.cursor == this.Size {
		return io.EOF
	}
	this.gobuf[this.cursor] = input
	this.cursor += 1
	return nil
}

// WriteAt implements the io.WriterAt interface to write to the memory block at an offset.
func (this *Memory) WriteAt(input []byte, offset int64) (int, error) {
	if offset >= int64(this.Size) {
		return 0, io.EOF
	}
	return copy(this.gobuf[offset:this.Size], input), nil
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
		return int64(this.cursor), ErrInvalidWhence
	}
	if newCursor < 0 {
		return int64(this.cursor), ErrNegativeOffset
	}
	if newCursor > int64(this.Size) {
		newCursor = int64(this.Size)
	}
	this.cursor = uint64(newCursor)
	return int64(this.cursor), nil
}

// Close implements the io.Closer interface to free the memory block. Any method call on this object after Close() is undefined.
func (this *Memory) Close() error {
	C.free(this.Cbuf)
	this.Cbuf = nil
	return nil
}

type subBlock struct {
	address unsafe.Pointer
	size    uint64
}

type block struct {
	trace           string
	goStack         []uintptr
	subBlocks       map[unsafe.Pointer]subBlock
	allocationCount uint64
	bytesAllocated  uint64
}

func (this *block) size() uint64 {
	var ret uint64
	for _, subBlock := range this.subBlocks {
		ret += subBlock.size
	}
	return ret
}

func (this *block) print(output io.Writer) error {
	_, err := fmt.Fprintf(output, "%d block(s) of total size %d were allocated at:\n%s\n", len(this.subBlocks), this.size(), this.trace)
	return err
}

var blocks map[string]*block = make(map[string]*block)
var addresses map[unsafe.Pointer]*block = make(map[unsafe.Pointer]*block)
var allocationCount uint64
var bytesAllocated uint64
var bytesFreed uint64

// StartInstrumentation begins recording all C memory allocations and frees.
func StartInstrumentation() {
	C.start_instrumentation()
}

// Stops recording C memory allocations and frees.
func StopInstrumentation() {
	C.stop_instrumentation()
}

// Resets the C memory statistics. Not safe to use while instrumentation is in
// progress.
func ResetInstrumentation() {
	blocks = make(map[string]*block)
	addresses = make(map[unsafe.Pointer]*block)
	allocationCount = 0
	bytesAllocated = 0
	bytesFreed = 0
}

//export instrumentMalloc
func instrumentMalloc(address unsafe.Pointer, size C.size_t, cTrace unsafe.Pointer, cFrames C.int) {
	var trace string
	for cFrame := 2; cFrame < int(cFrames)-1; cFrame++ {
		trace += C.GoString((*(*[]*C.char)(unsafe.Pointer(&cTrace)))[cFrame])
		trace += "\n"
	}
	var skip int = 4
	var inC bool = true
	goStack := make([]uintptr, 0)
	for {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		var funcName string = runtime.FuncForPC(pc).Name()
		goStack = append(goStack, pc)
		if strings.HasPrefix(funcName, "runtime.") {
			if !inC {
				inC = true
				trace += "C code\n"
			}
			skip++
			continue
		} else if strings.Contains(funcName, "_Cfunc_") {
			skip++
			continue
		}
		inC = false
		trace += runtime.FuncForPC(pc).Name() + "\n"
		trace += fmt.Sprintf("\t%s:%d (0x%x)\n", file, line, pc)
		skip++
	}
	trace = strings.TrimSuffix(trace, "C code\n")
	if _, ok := blocks[trace]; !ok {
		blocks[trace] = new(block)
		blocks[trace].trace = trace
		blocks[trace].goStack = goStack
		blocks[trace].subBlocks = make(map[unsafe.Pointer]subBlock)
	}
	blocks[trace].subBlocks[address] = subBlock{address, uint64(size)}
	blocks[trace].allocationCount += 1
	blocks[trace].bytesAllocated += uint64(size)
	addresses[address] = blocks[trace]
	allocationCount += 1
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
	TotalAllocations    uint64
	TotalBytesAllocated uint64
	BytesFreed          uint64
}

// Print prints out the human-readable stats contained in the Stats struct to
// stdout.
func (this *Stats) Print(output io.Writer) error {
	_, err := fmt.Fprintf(output, "Current number of allocations: %d\n", this.CurAllocations)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Current number of bytes allocated: %d\n", this.CurBytesAllocated)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Total number of allocations: %d\n", this.TotalAllocations)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Total number of bytes allocated: %d\n", this.TotalBytesAllocated)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Number of bytes freed: %d\n", this.BytesFreed)
	if err != nil {
		return err
	}
	return nil
}

// MemoryAnalysis creates a new Stats struct from the current C heap
// information.
func MemoryAnalysis() Stats {
	ret := Stats{}
	for _, curBlock := range blocks {
		ret.CurAllocations += uint64(len(curBlock.subBlocks))
		ret.CurBytesAllocated += curBlock.size()
	}
	ret.TotalAllocations = allocationCount
	ret.TotalBytesAllocated = bytesAllocated
	ret.BytesFreed = bytesFreed
	return ret
}

// MemoryDump writes out a pprof-compatible profile of the C heap to the output
// parameter.
func MemoryDump(output io.Writer) error {
	stats := MemoryAnalysis()
	_, err := fmt.Fprintf(output, "heap profile: %d: %d [%d: %d] @ heapprofile\n", stats.CurAllocations, stats.CurBytesAllocated, stats.TotalAllocations, stats.TotalBytesAllocated)
	if err != nil {
		return err
	}
	for _, curBlock := range blocks {
		_, err := fmt.Fprintf(output, "%d: %d [%d: %d] @", len(curBlock.subBlocks), curBlock.size(), curBlock.allocationCount, curBlock.bytesAllocated)
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

// MemoryBlocks writes out the stack traces of the allocated C blocks to the
// output parameter.
func MemoryBlocks(output io.Writer) error {
	for _, curBlock := range blocks {
		err := curBlock.print(output)
		if err != nil {
			return err
		}
	}
	return nil
}
