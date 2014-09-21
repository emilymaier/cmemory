// Package example contains a sample memory-leaking program with leaks detected by cmemory.
package main

/*
void c_main();
*/
import "C"

import "os"

import "cmemory"

func main() {
	cmemory.StartInstrumentation()
	C.c_main()
	stats := cmemory.MemoryAnalysis()
	stats.Print()
	cmemory.MemoryBlocks(os.Stdout)
	cmemory.MemoryDump(os.Stdout)
}
