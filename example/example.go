// Copyright © 2014 Emily Maier

/*
Commented example of how to use the cmemory tools.
*/
package main

/*
void c_main();
void c_untracked();
*/
import "C"

import "os"

import "github.com/emilymaier/cmemory"

func main() {
	cmemory.StartInstrumentation()
	// Once in C, all allocations and frees will be tracked.
	C.c_main()
	cmemory.StopInstrumentation()
	// No longer tracking C memory functions.
	C.c_untracked()
	stats := cmemory.MemoryAnalysis()
	stats.Print(os.Stdout)
	cmemory.MemoryBlocks(os.Stdout)
	f, _ := os.Create("heap")
	cmemory.MemoryDump(f)
	f.Close()
}
