// Copyright 2018, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Packages bytes contains common functions to manipulate slice of bytes.
package bytes

import (
	"fmt"
)

//
// PrintHex will print each byte in slice as hexadecimal value into N column
// length.
//
func PrintHex(title string, data []byte, col int) {
	fmt.Print(title)
	for x := 0; x < len(data); x++ {
		if x%col == 0 {
			fmt.Printf("\n%4d -", x)
		}

		fmt.Printf(" %02X", data[x])
	}
	fmt.Println()
}

//
// ReadInt16 will convert two bytes from data start at `x` into int16 and
// return it.
//
func ReadInt16(data []byte, x uint) int16 {
	return int16(data[x])<<8 | int16(data[x+1])
}

//
// ReadInt32 will convert four bytes from data start at `x` into int32 and
// return it.
//
func ReadInt32(data []byte, x uint) int32 {
	return int32(data[x])<<24 | int32(data[x+1])<<16 | int32(data[x+2])<<8 | int32(data[x+3])
}

//
// ReadUint16 will convert two bytes from data start at `x` into uint16 and
// return it.
//
func ReadUint16(data []byte, x uint) uint16 {
	return uint16(data[x])<<8 | uint16(data[x+1])
}

//
// ReadUint32 will convert four bytes from data start at `x` into uint32 and
// return it.
//
func ReadUint32(data []byte, x uint) uint32 {
	return uint32(data[x])<<24 | uint32(data[x+1])<<16 | uint32(data[x+2])<<8 | uint32(data[x+3])
}

//
// WriteUint16 into slice of byte.
//
func WriteUint16(data *[]byte, x uint, v uint16) {
	(*data)[x] = byte(v >> 8)
	(*data)[x+1] = byte(v)
}

//
// AppendInt16 into slice of byte.
//
func AppendInt16(data *[]byte, v int16) {
	*data = append(*data, byte(v>>8))
	*data = append(*data, byte(v))
}

//
// AppendInt32 into slice of byte.
//
func AppendInt32(data *[]byte, v int32) {
	*data = append(*data, byte(v>>24))
	*data = append(*data, byte(v>>16))
	*data = append(*data, byte(v>>8))
	*data = append(*data, byte(v))
}

//
// AppendUint16 into slice of byte.
//
func AppendUint16(data *[]byte, v uint16) {
	*data = append(*data, byte(v>>8))
	*data = append(*data, byte(v))
}

//
// AppendUint32 into slice of byte.
//
func AppendUint32(data *[]byte, v uint32) {
	*data = append(*data, byte(v>>24))
	*data = append(*data, byte(v>>16))
	*data = append(*data, byte(v>>8))
	*data = append(*data, byte(v))
}