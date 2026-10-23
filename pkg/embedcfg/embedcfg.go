// Package embedcfg writes the settings of an installation into the program of
// the client.
//
// The dashboard hands out the program of the client, the program carries the
// name, the icon and the address of the installation. The client reads them when
// it starts, it does not ask the user for them.
package embedcfg

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// The mark of the settings at the end of the program.
const markerText = "\x00\x00BYOG-CFG\x00"

var marker = []byte(markerText)

// The size of the mark.
const markerSize = len(markerText)

// Append writes the settings into the program of the client.
//
// The settings are written at the end of the program, the program of Windows
// does not mind the data that follows it.
func Append(program []byte, settings []byte) []byte {
	result := make([]byte, 0, len(program)+markerSize+len(settings)+4)
	result = append(result, program...)
	result = append(result, marker...)
	result = append(result, settings...)
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(settings)))
	return append(result, length...)
}

// Extract reads the settings from the program of the client.
//
// The second value tells whether the program carries the settings.
func Extract(program []byte) ([]byte, bool) {
	if len(program) < markerSize+4 {
		return nil, false
	}
	end := len(program)
	lengthPosition := end - 4
	length := int(binary.LittleEndian.Uint32(program[lengthPosition : lengthPosition+4]))
	markPosition := lengthPosition - markerSize - length
	if markPosition < 0 {
		return nil, false
	}
	if !bytes.Equal(program[markPosition:markPosition+markerSize], marker) {
		return nil, false
	}
	return program[markPosition+markerSize : lengthPosition], true
}

// ErrNoSettings tells that the program of the client does not carry the
// settings of an installation.
var ErrNoSettings = errors.New("the program does not carry the settings of the installation")
