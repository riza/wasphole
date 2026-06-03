package sim

import (
	"hash/fnv"
	"time"
)

// SystemState is the single source of truth for all simulated OS data.
// Every formatter receives *SystemState — cross-consistency is structural, not coordinated.
type SystemState struct {
	Mode      string
	Hostname  string
	BootTime  time.Time
	MachineID string
	Linux     *LinuxState
	Windows   *WindowsState
}

type LinuxState struct {
	KernelVersion string
	OSName        string
	OSVersion     string
	OSVersionID   string
	OSCodename    string
	GCCVersion    string
	BuildDate     string

	Users       []LinuxUser
	PrimaryUser LinuxUser
	Network     LinuxNetwork

	MemTotalKB  int
	MemFreeKB   int
	MemAvailKB  int
	SwapTotalKB int

	ListeningPorts []int
}

type LinuxUser struct {
	Username string
	Gecos    string
	Home     string
	Shell    string
	UID      int
	GID      int
	IsSystem bool
}

type LinuxNetwork struct {
	Interface string
	IP        string
	MAC       string
	Gateway   string
	DNS       []string
}

type WindowsState struct {
	ComputerName   string
	OSName         string
	OSVersion      string
	BuildNumber    string
	Domain         string
	MachineSID     string
	LastBootUpTime string

	PrimaryUser WindowsUser
	Network     WindowsNetwork
}

type WindowsUser struct {
	Username  string
	Domain    string
	SID       string
	Privilege string
	RID       int
	Groups    []string
}

type WindowsNetwork struct {
	AdapterName string
	IP          string
	MAC         string
	Subnet      string
	Gateway     string
	DNS         string
	DNSSuffix   string
}

// seedInt64 converts a string seed to int64 for deterministic gofakeit initialization.
// Empty string returns 0, which makes gofakeit use crypto/rand.
func seedInt64(s string) int64 {
	if s == "" {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}
