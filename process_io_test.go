package main

import (
	"strings"
	"testing"
)

func TestValidateProcessTransferRejectsPartialReadAndWrite(t *testing.T) {
	for _, label := range []string{"读取进程内存", "写入进程内存"} {
		err := validateProcessTransfer(label, 24, 8)
		if err == nil || !strings.Contains(err.Error(), "8/24") {
			t.Fatalf("%s partial transfer should be explicit, got %v", label, err)
		}
	}
}

func TestValidateProcessTransferAcceptsExactAndZeroLength(t *testing.T) {
	for _, size := range []uintptr{0, 1, 24, 4096} {
		if err := validateProcessTransfer("读取进程内存", size, size); err != nil {
			t.Fatalf("exact transfer %d rejected: %v", size, err)
		}
	}
}
