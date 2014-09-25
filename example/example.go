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
	stats.Print(os.Stdout)
	cmemory.MemoryBlocks(os.Stdout)
	f, _ := os.Create("heap")
	cmemory.MemoryDump(f)
	f.Close()
}
