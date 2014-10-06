// Copyright © 2014 Emily Maier

package cmemory

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestAlloc(t *testing.T) {
	mem, err := Alloc(256)
	if err != nil {
		t.Error(err)
	}
	if !isConsistent(mem.Cbuf) {
		t.Error("Alloc() failed to allocate C block")
	}
}

func TestAllocFromSlice(t *testing.T) {
	testData := initTestData()
	mem, err := AllocFromSlice(testData)
	if err != nil {
		t.Error(err)
	}
	if !isConsistent(mem.Cbuf) {
		t.Error("AllocFromSlice() failed to allocate C block")
	}
	for _, data := range testData {
		readByte, _ := mem.ReadByte()
		if readByte != data {
			t.Error("AllocFromSlice() failed to initialize the buffer")
		}
	}
}

func TestWrapMemory(t *testing.T) {
	block := testMalloc(256)
	mem := WrapMemory(block, 256)
	if block != mem.Cbuf {
		t.Error("WrapMemory() failed to set buffer")
	}
}

func initTestData() []byte {
	testData := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testData[i] = byte(i)
	}
	return testData
}

func TestGrow(t *testing.T) {
	mem, _ := Alloc(256)
	testData := initTestData()
	mem.Write(testData)
	mem.Seek(0, 0)
	err := mem.Grow(512)
	if err != nil {
		t.Error("Grow() failed to allocate C block")
	}
	testData = make([]byte, 256)
	bytesRead, err := mem.Read(testData)
	if err != nil {
		t.Error("Grow() caused EOF to be returned too early")
	}
	if bytesRead != 256 {
		t.Error("Grow() caused the incorrect number of bytes to be read")
	}
	for index, data := range testData {
		if byte(index) != data {
			t.Error("Grow() caused the incorrect data to be returned")
		}
	}
}

func TestReadWrite(t *testing.T) {
	mem, _ := Alloc(256)
	testData := initTestData()
	bytesWritten, err := mem.Write(testData)
	if err != nil {
		t.Error("Write() returned EOF too early")
	}
	if bytesWritten != 256 {
		t.Error("Write() did not write the correct number of bytes")
	}
	_, err = mem.Write(testData)
	if err != io.EOF {
		t.Error("Write() failed to return EOF")
	}
	mem.Seek(0, 0)

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
	_, err = mem.Read(testData)
	if err != io.EOF {
		t.Error("Read() failed to return EOF")
	}

	mem, _ = Alloc(128)
	testData = initTestData()
	bytesWritten, err = mem.Write(testData)
	if err != nil {
		t.Error("Write() returned EOF too early")
	}
	if bytesWritten != 128 {
		t.Error("Write() did not write the correct number of bytes")
	}
	if mem.cursor != 128 {
		t.Error("Write() moved the cursor past the end of the file")
	}
	mem.Seek(0, 0)

	bytesRead, err = mem.Read(testData)
	if err != nil {
		t.Error("Read() returned EOF too early")
	}
	if bytesRead != 128 {
		t.Error("Read() did not read the correct number of bytes")
	}
	if mem.cursor != 128 {
		t.Error("Read() moved the cursor past the end of the file")
	}
}

func TestReadWriteByte(t *testing.T) {
	mem, _ := Alloc(256)
	testData := initTestData()
	for _, oneByte := range testData {
		err := mem.WriteByte(oneByte)
		if err != nil {
			t.Error("WriteByte() returned EOF too early")
		}
	}
	err := mem.WriteByte(0)
	if err != io.EOF {
		t.Error("WriteByte() failed to return EOF")
	}
	mem.Seek(0, 0)

	for i := 0; i < 256; i++ {
		testData, err := mem.ReadByte()
		if err != nil {
			t.Error("ReadByte() returned EOF too early")
		}
		if byte(i) != testData {
			t.Error("ReadByte() did not return the correct data")
		}
	}
	_, err = mem.ReadByte()
	if err != io.EOF {
		t.Error("ReadByte() failed to return EOF")
	}
}

func TestUnreadByte(t *testing.T) {
	mem, _ := Alloc(256)
	testData := initTestData()
	mem.Write(testData)
	mem.Seek(0, 0)
	mem.ReadByte()
	mem.ReadByte()
	mem.ReadByte()
	mem.ReadByte()
	err := mem.UnreadByte()
	if err != nil {
		t.Error("UnreadByte() returned EOF too early")
	}
	readData, err := mem.ReadByte()
	if err != nil {
		t.Error("UnreadByte() caused an early EOF")
	}
	if readData != 3 {
		t.Error("UnreadByte() caused incorrect data to be returned")
	}

	mem.Seek(0, 0)
	err = mem.UnreadByte()
	if err != io.EOF {
		t.Error("UnreadByte() failed to return EOF")
	}
}

func TestReadWriteAt(t *testing.T) {
	mem, _ := Alloc(256)
	testData := initTestData()
	bytesWritten, err := mem.WriteAt(testData, 128)
	if err != nil {
		t.Error("WriteAt() returned EOF too early")
	}
	if bytesWritten != 128 {
		t.Error("WriteAt() did not write the correct number of bytes")
	}
	_, err = mem.WriteAt(testData, 256)
	if err != io.EOF {
		t.Error("WriteAt() failed to return EOF")
	}

	testData = make([]byte, 128)
	count, err := mem.ReadAt(testData, 128)
	if err != nil {
		t.Error("ReadAt() returned EOF too early")
	}
	if count != 128 {
		t.Error("ReadAt() did not read the correct number of bytes")
	}
	for index, data := range testData {
		if byte(index) != data {
			t.Error("ReadAt() did not return the correct data")
		}
	}
	_, err = mem.ReadAt(testData, 256)
	if err != io.EOF {
		t.Error("ReadAt() failed to return EOF")
	}
}

func TestSeek(t *testing.T) {
	mem, _ := Alloc(256)
	testData := initTestData()
	mem.Write(testData)
	cursor, err := mem.Seek(100, 0)
	if err != nil {
		t.Error("Seek() failed")
	}
	if cursor != 100 {
		t.Error("Seek() went to the wrong location")
	}
	cursor, err = mem.Seek(-50, 1)
	if err != nil {
		t.Error("Seek() failed")
	}
	if cursor != 50 {
		t.Error("Seek() went to the wrong location")
	}
	cursor, err = mem.Seek(-100, 2)
	if err != nil {
		t.Error("Seek() failed")
	}
	if cursor != 156 {
		t.Error("Seek() went to the wrong location")
	}
	_, err = mem.Seek(0, 3)
	if err != ErrInvalidWhence {
		t.Error("Seek() failed to detect invalid whence parameter")
	}
	_, err = mem.Seek(-1, 0)
	if err != ErrNegativeOffset {
		t.Error("Seek() failed to detect negative offset paramter")
	}
	cursor, err = mem.Seek(1000, 0)
	if err != nil {
		t.Error("Seek() failed")
	}
	if cursor != 256 {
		t.Error("Seek() went past end of file")
	}
}

func TestClose(t *testing.T) {
	mem, _ := Alloc(256)
	mem.Close()
	if mem.Cbuf != nil {
		t.Error("Close() did not free the block")
	}
}

func TestInstrumentation(t *testing.T) {
	ResetInstrumentation()
	StartInstrumentation()
	block1, err := Alloc(256)
	if err != nil {
		t.Error("Alloc() failed to allocate C block")
	}
	// make sure block1 is used
	if block1 == nil {
		t.Fatal("How did you get here?")
	}
	stats := MemoryAnalysis()
	if stats.TotalBytesAllocated != 256 {
		t.Error("StartInstrumentation() failed to start instrumenting")
	}
	StopInstrumentation()
	block2, err := Alloc(256)
	if err != nil {
		t.Error("Alloc() failed to allocate C block")
	}
	// make sure block2 is used
	if block2 == nil {
		t.Fatal("How did you get here?")
	}
	stats = MemoryAnalysis()
	if stats.TotalBytesAllocated != 256 {
		t.Error("StopInstrumentation() failed to stop instrumenting")
	}
}

func TestLeaks(t *testing.T) {
	ResetInstrumentation()
	StartInstrumentation()
	block1, err := Alloc(256)
	if err != nil {
		t.Error("Alloc() failed to allocate C block")
	}
	block2, err := Alloc(128)
	if err != nil {
		t.Error("Alloc() failed to allocate C block")
	}
	block3, err := Alloc(32)
	if err != nil {
		t.Error("Alloc() failed to allocate C block")
	}
	err = block3.Grow(64)
	if err != nil {
		t.Error("Grow() failed to allocated C block")
	}
	finalizeMemory(block2)
	// make sure block1 is used
	if block1 == nil {
		t.Fatal("How did you get here?")
	}

	stats := MemoryAnalysis()
	if stats.CurAllocations != 2 {
		t.Error("MemoryAnalysis() gave the wrong number of current allocations")
	}
	if stats.CurBytesAllocated != 320 {
		t.Error("MemoryAnalysis() gave the wrong number of current bytes allocated")
	}
	if stats.TotalAllocations != 4 {
		t.Error("MemoryAnalysis() gave the wrong number of total allocations")
	}
	if stats.TotalBytesAllocated != 480 {
		t.Error("MemoryAnalysis() gave the wrong number of total bytes allocated")
	}
	if stats.BytesFreed != 160 {
		t.Error("MemoryAnalysis() gave the wrong number of bytes freed")
	}

	buffer := bytes.NewBuffer(make([]byte, 0))
	stats.Print(buffer)
	bufferString := buffer.String()
	if bufferString != "Current number of allocations: 2\nCurrent number of bytes allocated: 320\nTotal number of allocations: 4\nTotal number of bytes allocated: 480\nNumber of bytes freed: 160\n" {
		t.Error("Stats.Print() printed the wrong text")
	}

	buffer = bytes.NewBuffer(make([]byte, 0))
	err = MemoryDump(buffer)
	if err != nil {
		t.Error("MemoryDump() failed")
	}
	bufferString = buffer.String()
	if !strings.HasPrefix(bufferString, "heap profile: 2: 320 [4: 480] @ heapprofile\n1: 256 [1: 256] @") {
		t.Error("MemoryDump() returned incorrect results")
	}
	if !strings.Contains(bufferString, "0: 0 [1: 32] @") {
		t.Error("MemoryDump() returned incorrect results")
	}
	if !strings.Contains(bufferString, "1: 64 [1: 64] @") {
		t.Error("MemoryDump() returned incorrect results")
	}

	buffer = bytes.NewBuffer(make([]byte, 0))
	err = MemoryBlocks(buffer)
	if err != nil {
		t.Error("MemoryBlocks() failed")
	}
	bufferString = buffer.String()
	if !strings.Contains(bufferString, "1 block(s) of total size 256 were allocated at:") {
		t.Error("MemoryBlocks() printed incorrect results")
	}
	if !strings.Contains(bufferString, "1 block(s) of total size 64 were allocated at:") {
		t.Error("MemoryBlocks() printed incorrect results")
	}
}
