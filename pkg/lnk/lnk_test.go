package lnk

import (
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShortcutBuildsTheFileOfTheShortcut makes sure that the file of the
// shortcut has the structure the system expects.
//
// The header of the file has a fixed size, the item list of the target follows
// it and the strings of the shortcut follow the item list.
func TestShortcutBuildsTheFileOfTheShortcut(t *testing.T) {
	shortcut := Shortcut{
		Name:             "BYOG Worker",
		TargetPath:       "powershell.exe",
		Arguments:        "-NoProfile",
		WorkingDirectory: "C:\\Windows\\System32",
		IconLocation:     "http://127.0.0.1:3000/api/byog/favicon.ico",
	}

	file := shortcut.Bytes()
	require.Greater(t, len(file), headerSize+len(targetItemList))

	// The header of the file.
	assert.Equal(t, byte(0x4C), file[0])
	assert.Equal(t, byte(0x01), file[4])
	assert.Equal(t, byte(0x46), file[19])

	// The flags of the link, every part of the shortcut has its flag.
	flags := uint32(file[0x14]) | uint32(file[0x15])<<8 | uint32(file[0x16])<<16 | uint32(file[0x17])<<24
	assert.Equal(t, uint32(flagHasLinkTargetIDList|flagHasName|flagHasRelativePath|
		flagHasWorkingDir|flagHasArguments|flagHasIconLocation|flagIsUnicode|flagRunAsInvoker), flags)

	// The item list of the target follows the header.
	assert.Equal(t, targetItemList, file[headerSize:headerSize+len(targetItemList)])

	// The strings of the shortcut follow the item list.
	name, nameLength := readString(t, file, headerSize+len(targetItemList))
	assert.Equal(t, "BYOG Worker", name)

	path, _ := readString(t, file, headerSize+len(targetItemList)+nameLength)
	assert.Equal(t, "powershell.exe", path)

	// The end of the file.
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x00}, file[len(file)-4:])
}

// TestFileNameRemovesTheInvalidCharacters makes sure that the name of the file
// of the shortcut can be used on every system.
func TestFileNameRemovesTheInvalidCharacters(t *testing.T) {
	assert.Equal(t, "BYOG Worker.lnk", FileName("BYOG Worker"))
	assert.Equal(t, "Test-Installation.lnk", FileName("Test/Installation"))
	assert.Equal(t, "BYOG.lnk", FileName("  BYOG  "))
}

// readString reads the string of the shortcut that starts at the given place.
//
// The string starts with the number of its characters, the characters follow it.
// The number of the bytes of the string is returned with it.
func readString(t *testing.T, file []byte, offset int) (string, int) {
	t.Helper()
	require.Less(t, offset+2, len(file))
	length := int(uint16(file[offset]) | uint16(file[offset+1])<<8)
	require.Less(t, offset+2+length*2, len(file))
	characters := make([]uint16, 0, length)
	for index := 0; index < length; index++ {
		position := offset + 2 + index*2
		characters = append(characters, uint16(file[position])|uint16(file[position+1])<<8)
	}
	return string(utf16.Decode(characters)), 2 + length*2
}
