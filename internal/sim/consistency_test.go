package sim_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/riza/wasphole/internal/config"
	"github.com/riza/wasphole/internal/sim"
)

func testCfg(t *testing.T, mode, seed string) *config.Config {
	t.Helper()
	return &config.Config{
		Instance: config.InstanceConfig{Mode: mode, Seed: seed},
		AI:       config.AIConfig{CacheDir: t.TempDir()},
	}
}

func TestLinuxCrossConsistency(t *testing.T) {
	s, err := sim.New(testCfg(t, "linux", "test-seed-42"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	hostname, _ := s.ReadFile("/etc/hostname")
	if hostname != s.Hostname+"\n" {
		t.Errorf("/etc/hostname=%q, want %q", hostname, s.Hostname+"\n")
	}

	shellHostname, _ := s.ExecShell("hostname")
	if shellHostname != s.Hostname+"\n" {
		t.Errorf("ExecShell(hostname)=%q, want %q", shellHostname, s.Hostname+"\n")
	}

	authLog, _ := s.ReadFile("/var/log/auth.log")
	if !strings.Contains(authLog, s.Hostname) {
		t.Errorf("/var/log/auth.log does not contain hostname %q", s.Hostname)
	}
}

func TestKernelVersionConsistency(t *testing.T) {
	s, err := sim.New(testCfg(t, "linux", "kernel-check"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	procVersion, _ := s.ReadFile("/proc/version")
	if !strings.Contains(procVersion, s.Linux.KernelVersion) {
		t.Errorf("/proc/version missing KernelVersion %q", s.Linux.KernelVersion)
	}

	osRelease, _ := s.ReadFile("/etc/os-release")
	if !strings.Contains(osRelease, s.Linux.OSVersionID) {
		t.Errorf("/etc/os-release missing OSVersionID %q", s.Linux.OSVersionID)
	}
}

func TestUptimeMonotonic(t *testing.T) {
	s, err := sim.New(testCfg(t, "linux", "uptime-test"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	v1, _ := s.ReadFile("/proc/uptime")
	time.Sleep(110 * time.Millisecond)
	v2, _ := s.ReadFile("/proc/uptime")

	secs1, err1 := strconv.ParseFloat(strings.Fields(v1)[0], 64)
	secs2, err2 := strconv.ParseFloat(strings.Fields(v2)[0], 64)
	if err1 != nil || err2 != nil {
		t.Fatalf("failed to parse uptime: %v / %v", err1, err2)
	}
	if secs2 <= secs1 {
		t.Errorf("uptime did not increase: %.2f → %.2f", secs1, secs2)
	}
}

func TestSameSeedSameState(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Instance: config.InstanceConfig{Mode: "linux", Seed: "seed-xyz"},
		AI:       config.AIConfig{CacheDir: dir},
	}

	s1, err1 := sim.New(cfg)
	s2, err2 := sim.New(cfg)
	if err1 != nil || err2 != nil {
		t.Fatalf("New() errors: %v / %v", err1, err2)
	}

	if s1.Hostname != s2.Hostname {
		t.Errorf("Hostname differs: %q vs %q", s1.Hostname, s2.Hostname)
	}
	if s1.Linux.KernelVersion != s2.Linux.KernelVersion {
		t.Errorf("KernelVersion differs: %q vs %q", s1.Linux.KernelVersion, s2.Linux.KernelVersion)
	}
	if s1.MachineID != s2.MachineID {
		t.Errorf("MachineID differs: %q vs %q", s1.MachineID, s2.MachineID)
	}
}

func TestWindowsCrossConsistency(t *testing.T) {
	s, err := sim.New(testCfg(t, "windows", "win-test-seed"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	hostCmd, _ := s.RunCommand("hostname")
	if hostCmd != s.Hostname+"\r\n" {
		t.Errorf("RunCommand(hostname)=%q, want %q", hostCmd, s.Hostname+"\r\n")
	}

	wmiCS, _ := s.QueryWMI("Win32_ComputerSystem")
	if !strings.Contains(wmiCS, s.Hostname) {
		t.Errorf("Win32_ComputerSystem missing Hostname %q", s.Hostname)
	}

	wmiOS, _ := s.QueryWMI("Win32_OperatingSystem")
	if !strings.Contains(wmiOS, s.Hostname) {
		t.Errorf("Win32_OperatingSystem missing Hostname %q", s.Hostname)
	}
	if !strings.Contains(wmiOS, s.Windows.LastBootUpTime) {
		t.Errorf("Win32_OperatingSystem missing LastBootUpTime %q", s.Windows.LastBootUpTime)
	}

	expectedSID := s.Windows.MachineSID + "-" + strconv.Itoa(s.Windows.PrimaryUser.RID)
	if s.Windows.PrimaryUser.SID != expectedSID {
		t.Errorf("PrimaryUser.SID=%q, expected %q", s.Windows.PrimaryUser.SID, expectedSID)
	}
}

func TestShadowPermissionDenied(t *testing.T) {
	s, err := sim.New(testCfg(t, "linux", "shadow-test"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_, err = s.ReadFile("/etc/shadow")
	if err == nil {
		t.Fatal("ReadFile(/etc/shadow) should return error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected 'permission denied', got: %v", err)
	}
}

func TestAuthLogPreBootTime(t *testing.T) {
	s, err := sim.New(testCfg(t, "linux", "authlog-test"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	authLog, err := s.ReadFile("/var/log/auth.log")
	if err != nil {
		t.Fatalf("ReadFile(/var/log/auth.log) error: %v", err)
	}
	if len(authLog) == 0 {
		t.Fatal("auth.log is empty")
	}
	if !strings.Contains(authLog, s.Hostname) {
		t.Errorf("auth.log does not contain hostname %q", s.Hostname)
	}
}
