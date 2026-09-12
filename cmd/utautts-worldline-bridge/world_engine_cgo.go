//go:build (linux || darwin) && cgo

package main

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

func cString(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}
	return string(buffer)
}

func cBytes[T any](values []T) unsafe.Pointer {
	if len(values) == 0 {
		return nil
	}
	return C.CBytes(unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*int(unsafe.Sizeof(values[0]))))
}

func freePointers(pointers []unsafe.Pointer) {
	for _, pointer := range pointers {
		if pointer != nil {
			C.free(pointer)
		}
	}
}
