//go:build windows

package serverpicker

import (
	"encoding/binary"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The IP Helper echo API rather than a raw ICMP socket: raw sockets need
// Administrator on Windows, and this page has to work before the user elevates.
// It is what .NET's System.Net.NetworkInformation.Ping uses.
var (
	modiphlpapi          = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreateFile   = modiphlpapi.NewProc("IcmpCreateFile")
	procIcmpCloseHandle  = modiphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho     = modiphlpapi.NewProc("IcmpSendEcho")
	echoPayload          = []byte("TcNo-Acc-Switcher server picker")
	statusSuccess uint32 = 0
)

// ipOptionInformation mirrors IP_OPTION_INFORMATION.
type ipOptionInformation struct {
	TTL         uint8
	TOS         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

// icmpEchoReply mirrors ICMP_ECHO_REPLY. Field order and types are load-bearing:
// the kernel writes this layout into our buffer.
type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

// icmpEcho sends one echo request and reports the round trip in milliseconds.
func icmpEcho(ip string, timeout time.Duration) (int, bool) {
	addr := net.ParseIP(ip)
	if addr == nil {
		return 0, false
	}
	v4 := addr.To4()
	if v4 == nil {
		return 0, false
	}
	// IPAddr is network byte order, i.e. the four octets in memory order, which
	// is what a little-endian load of the slice produces.
	dest := binary.LittleEndian.Uint32(v4)

	h, _, _ := procIcmpCreateFile.Call()
	if h == 0 || h == uintptr(windows.InvalidHandle) {
		return 0, false
	}
	defer procIcmpCloseHandle.Call(h)

	// The API requires room for the reply header, the echoed payload and 8 bytes
	// of trailing ICMP error data.
	replySize := int(unsafe.Sizeof(icmpEchoReply{})) + len(echoPayload) + 8
	reply := make([]byte, replySize)

	ms := timeout.Milliseconds()
	if ms <= 0 {
		ms = 1
	}
	n, _, _ := procIcmpSendEcho.Call(
		h,
		uintptr(dest),
		uintptr(unsafe.Pointer(&echoPayload[0])),
		uintptr(uint16(len(echoPayload))),
		0,
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(uint32(replySize)),
		uintptr(uint32(ms)),
	)
	if n == 0 {
		return 0, false
	}
	r := (*icmpEchoReply)(unsafe.Pointer(&reply[0]))
	if r.Status != statusSuccess {
		return 0, false
	}
	return int(r.RoundTripTime), true
}
