package shortcutsvdf

import "hash/crc32"

// The Exe value is hashed with its quote bytes included. A caller that strips
// them gets a different id, which orphans any artwork the user set.

// shortAppID is CRC-32/IEEE over Exe+AppName with the high bit forced. It names
// artwork under userdata/<id32>/config/grid/.
func shortAppID(exeQuoted, appName string) uint32 {
	return crc32.ChecksumIEEE([]byte(exeQuoted+appName)) | 0x80000000
}

// ShortcutAppID is what goes in the entry's "appid" field: the short id read as
// a signed 32-bit integer, so always negative.
func ShortcutAppID(exeQuoted, appName string) int32 {
	return int32(shortAppID(exeQuoted, appName))
}
