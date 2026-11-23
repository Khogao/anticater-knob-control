package main

import (
	"fmt"
	"os"
	"testing"
	"unsafe"
)

func TestJobExtendedLimitSize(t *testing.T) {
	if s := unsafe.Sizeof(jobExtendedLimit{}); s != 144 {
		t.Fatalf("JOBOBJECT_EXTENDED_LIMIT_INFORMATION is %d bytes, want 144", s)
	}
}

// A second takeMutex on the same name in the same process must fail, as it
// would in a second process.
func TestTakeMutexOnce(t *testing.T) {
	name := fmt.Sprintf(`Local\AnticaterKnobTest%d`, os.Getpid())
	if !takeMutex(name) {
		t.Fatal("first take failed")
	}
	if takeMutex(name) {
		t.Fatal("second take succeeded; the lock does not hold")
	}
}
