package cmemory

import (
	"bytes"
	"strings"
	"testing"
)

func TestMemory(t *testing.T) {
	mem, err := Alloc(256)
	if err != nil {
		t.Fatal(err)
	}
	if !isConsistent(mem.Cbuf) {
		t.Error("Alloc() failed to allocate C block")
	}

	testData := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testData[i] = byte(i)
	}
	bytesWritten, err := mem.Write(testData)
	if err != nil {
		t.Error("Write() returned EOF too early")
	}
	if bytesWritten != 256 {
		t.Error("Write() did not write the correct number of bytes")
	}
	testData = make([]byte, 1)
	bytesWritten, err = mem.Write(testData)
	if err == nil {
		t.Error("Write() went past the end of the buffer")
	}
	if !isConsistent(mem.Cbuf) {
		t.Error("Write() caused an inconsistency in the C block")
	}

	newCursor, err := mem.Seek(0, 0)
	if err != nil {
		t.Error("Seek() failed")
	}
	if newCursor != 0 {
		t.Error("Seek() went to wrong location")
	}

	testData = make([]byte, 256)
	bytesRead, err := mem.Read(testData)
	if err != nil {
		t.Error("Read() returned EOF too early")
	}
	if bytesRead != 256 {
		t.Error("Read() did not read the correct number of bytes")
	}
	for index, data := range testData {
		if byte(index) != data {
			t.Error("Read() did not return the correct data")
		}
	}
	testData = make([]byte, 1)
	bytesRead, err = mem.Read(testData)
	if err == nil {
		t.Error("Read() went past the end of the buffer")
	}
	if !isConsistent(mem.Cbuf) {
		t.Error("Read() caused an inconsistency in the C block")
	}
}

func TestLeaks(t *testing.T) {
	StartInstrumentation()
	_, err := Alloc(256)
	if err != nil {
		t.Error("Alloc() failed to allocate C block")
	}
	stats := MemoryAnalysis()
	if stats.CurAllocations != 1 {
		t.Error("MemoryAnalysis() gave the wrong number of allocations")
	}
	if stats.CurBytesAllocated != 256 {
		t.Error("MemoryAnalysis() gave the wrong number of current bytes allocated")
	}
	if stats.TotalBytesAllocated != 256 {
		t.Error("MemoryAnalysis() gave the wrong number of total bytes allocated")
	}
	if stats.BytesFreed != 0 {
		t.Error("MemoryAnalysis() gave the wrong number of bytes freed")
	}

	buffer := bytes.NewBuffer(make([]byte, 0))
	err = MemoryDump(buffer)
	if err != nil {
		t.Error("MemoryDump() failed")
	}
	bufferString := buffer.String()
	if !strings.HasPrefix(bufferString, "heap profile: 1: 256 [1: 256] @ heapprofile\n1: 256 [1: 256] @") {
		t.Error("MemoryDump() returned incorrect results")
	}
}
