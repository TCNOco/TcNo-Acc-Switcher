package streamer

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
)

const saltDomain = "tcno-streamer-avatar-v1"

// virtualMACPrefixes are OUIs handed out to hypervisors and tunnel drivers. The
// adapter set on a machine changes as VMs and VPNs come and go, so keying the salt
// on one of these would reshuffle every avatar the next time Docker starts.
var virtualMACPrefixes = []string{
	"00:05:69", "00:0c:29", "00:1c:14", "00:50:56", // VMware
	"08:00:27", "0a:00:27", // VirtualBox
	"00:15:5d", // Hyper-V
	"00:16:3e", // Xen
	"00:03:ff", // Microsoft Virtual PC
	"02:00:4c", // Npcap loopback
	"00:ff:00", // Windows TAP
}

// virtualNameHints match adapters that are software endpoints rather than hardware.
var virtualNameHints = []string{
	"virtual", "vmware", "hyper-v", "vethernet", "loopback", "pseudo",
	"tap", "tun", "vpn", "docker", "wsl", "zerotier", "tailscale",
	"hamachi", "radmin", "bluetooth", "wan miniport", "teredo", "isatap",
}

var salt struct {
	sync.Once
	value string
}

// MachineSalt returns a stable, machine-local hex string mixed into every avatar
// seed. Derived from the primary physical MAC address, so the same account name or
// SteamID64 produces a different avatar on a different computer — someone watching
// a stream cannot regenerate a viewer's avatar to confirm who they are.
//
// A NIC swap changes the salt and therefore the generated avatars. That is cosmetic:
// nothing is stored against it.
func MachineSalt() string {
	salt.Do(func() {
		h := sha256.New()
		h.Write([]byte(saltDomain))
		h.Write([]byte{0})
		if mac := primaryMAC(); mac != "" {
			h.Write([]byte(mac))
		} else if host, err := os.Hostname(); err == nil {
			// No usable adapter. Weaker, but still keeps seeds off other machines.
			h.Write([]byte("host:" + strings.ToLower(host)))
		}
		salt.value = hex.EncodeToString(h.Sum(nil))[:32]
	})
	return salt.value
}

// primaryMAC picks the lowest-sorting physical MAC so the choice does not depend on
// adapter enumeration order, link state, or which NIC happens to be connected.
func primaryMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var candidates []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		mac := strings.ToLower(iface.HardwareAddr.String())
		if len(mac) != 17 || mac == "00:00:00:00:00:00" {
			continue
		}
		if isVirtualAdapter(iface.Name, mac) {
			continue
		}
		candidates = append(candidates, mac)
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	return candidates[0]
}

func isVirtualAdapter(name, mac string) bool {
	lowerName := strings.ToLower(name)
	for _, hint := range virtualNameHints {
		if strings.Contains(lowerName, hint) {
			return true
		}
	}
	for _, prefix := range virtualMACPrefixes {
		if strings.HasPrefix(mac, prefix) {
			return true
		}
	}
	// Bit 0x02 of the first octet marks a locally administered address: generated,
	// not burned in. Windows' randomised Wi-Fi MACs land here and rotate.
	return len(mac) >= 2 && strings.ContainsRune("2367abef", rune(mac[1]))
}
