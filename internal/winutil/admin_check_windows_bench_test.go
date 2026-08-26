package winutil

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// firstPIDForImageNameLegacy is the benchmark baseline: a whole-machine process
// snapshot for one image name.
func firstPIDForImageNameLegacy(want string) (pid uint32, found bool, err error) {
	want = strings.TrimSpace(want)
	if want == "" {
		return 0, false, nil
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, false, err
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		if err == windows.ERROR_NO_MORE_FILES {
			return 0, false, nil
		}
		return 0, false, err
	}
	for {
		if strings.EqualFold(utf16FixedToString(pe.ExeFile[:]), want) {
			return pe.ProcessID, true, nil
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				return 0, false, nil
			}
			return 0, false, err
		}
	}
}

// benchExeNames mirrors the ExesToEnd lists in Platforms.json: the median
// platform names two executables, the largest six. None are running - the normal
// case, and the one that costs a full walk.
var benchExeNames = []string{
	"RiotClientServices.exe", "LeagueClient.exe", "VALORANT.exe",
	"RiotClientUx.exe", "RiotClientUxRender.exe", "RiotClientCrashHandler.exe",
}

// BenchmarkProcessLookupPerName compares one process-table snapshot per
// executable against a single shared snapshot.
func BenchmarkProcessLookupPerName(b *testing.B) {
	for _, n := range []int{2, 6} {
		names := benchExeNames[:n]
		b.Run(fmt.Sprintf("%dexes/PerName", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, name := range names {
					if _, _, err := firstPIDForImageNameLegacy(name); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
		b.Run(fmt.Sprintf("%dexes/OneSnapshot", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				pids, err := snapshotFirstPIDs()
				if err != nil {
					b.Fatal(err)
				}
				for _, name := range names {
					_ = pids[strings.ToLower(name)]
				}
			}
		})
	}
}
