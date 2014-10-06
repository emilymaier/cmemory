# cmemory

cmemory is a Go library to help write cgo code. It provides a wrapper for C memory blocks that provides I/O functions and frees unused memory. It also provides heap profiling for all C-allocated memory.

[![GoDoc](https://godoc.org/github.com/emilymaier/cmemory?status.png)](https://godoc.org/github.com/emilymaier/cmemory)

## Installation

```bash
go get -u github.com/emilymaier/cmemory
```

cmemory does not have any Go dependencies. It has not been tested against libc other than glibc.

## Components

### Buffer

The Memory struct provides an interface to memory that has been allocated on the C heap. It can allocate memory itself, or it can take control of a block that's already been allocated. When the Memory object is garbage collected, it frees the C memory that it references. It implements the io.Reader, io.Writer, and io.Seeker interfaces allowing easy reads and writes to the memory.

```go
// Generates a C block of 256 bytes.
buffer, err := cmemory.Alloc(256)
```

### Profiling

cmemory implements the C memory allocation functions, allowing all C memory allocation to be profiled without changing any other code. When it is instrumenting memory, it keeps track of the number and size of allocations, when they are freed, as well as the stack trace of the code that created them.

The profiling tool provides several ways to get information about C memory usage. It can print a valgrind-like output of allocated blocks, though it cannot distinguish between reachable and unreachable blocks. It can also create a pprof-compatible output of all the blocks currently and formerly allocated.

```go
cmemory.StartInstrumentation()
// C allocations, either in cgo or cmemory buffers, are tracked.
stats := cmemory.MemoryAnalysis()
stats.Print(os.Stdout)
```

## Testing

The tests need to be built with "-tags test" in order to work, as they rely on helper functions in cmemory only built for testing.

## Usage

The example/ directory contains a larger example of using cmemory to find memory leaks in C code.
