package sim

import (
	"fmt"
	"strings"
	"time"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

var windowsDomains = []string{"corp.internal", "contoso.local", "ad.company.com"}

func generateWindowsState(fake *gofakeit.Faker, s *SystemState) *WindowsState {
	domain := windowsDomains[fake.IntRange(0, len(windowsDomains)-1)]
	domainShort := strings.ToUpper(strings.Split(domain, ".")[0])
	machineSID := generateMachineSID(fake)

	const rid = 1001
	primaryUser := WindowsUser{
		Username:  "svc-deploy",
		Domain:    domainShort,
		RID:       rid,
		SID:       fmt.Sprintf("%s-%d", machineSID, rid),
		Groups:    []string{domainShort + `\Domain Users`, `BUILTIN\Users`},
		Privilege: "SeChangeNotifyPrivilege\nSeIncreaseWorkingSetPrivilege",
	}

	ipOctets := [4]int{10, fake.IntRange(1, 5), fake.IntRange(1, 3), fake.IntRange(10, 200)}
	ip := fmt.Sprintf("%d.%d.%d.%d", ipOctets[0], ipOctets[1], ipOctets[2], ipOctets[3])
	gateway := fmt.Sprintf("%d.%d.%d.1", ipOctets[0], ipOctets[1], ipOctets[2])

	bootStr := wmiBootTime(s.BootTime)

	return &WindowsState{
		ComputerName:   s.Hostname,
		OSName:         "Microsoft Windows Server 2022 Datacenter",
		OSVersion:      "10.0.20348",
		BuildNumber:    "20348",
		Domain:         domain,
		MachineSID:     machineSID,
		LastBootUpTime: bootStr,
		PrimaryUser:    primaryUser,
		Network: WindowsNetwork{
			AdapterName: "Ethernet0",
			IP:          ip,
			MAC:         fake.MacAddress(),
			Subnet:      "255.255.255.0",
			Gateway:     gateway,
			DNS:         "10.0.0.53",
			DNSSuffix:   domain,
		},
	}
}

func generateMachineSID(fake *gofakeit.Faker) string {
	return fmt.Sprintf("S-1-5-21-%d-%d-%d",
		fake.Uint32(), fake.Uint32(), fake.Uint32())
}

func wmiBootTime(t time.Time) string {
	return t.UTC().Format("20060102150405") + ".000000+000"
}

func (s *SystemState) RunCommand(cmd string) (string, error) {
	if s.Windows == nil {
		return "", fmt.Errorf("'%s' is not recognized as an internal or external command, operable program or batch file.", cmd)
	}
	w := s.Windows
	p := w.PrimaryUser
	trimmed := strings.TrimSpace(cmd)
	lower := strings.ToLower(trimmed)

	switch {
	case lower == "hostname":
		return s.Hostname + "\r\n", nil

	case lower == "whoami":
		return fmt.Sprintf("%s\\%s\r\n", p.Domain, p.Username), nil

	case lower == "whoami /all":
		return whoamiAll(p), nil

	case strings.HasPrefix(lower, "ipconfig"):
		return ipconfigOutput(w), nil

	case strings.HasPrefix(lower, "systeminfo"):
		return sysinfoOutput(s), nil

	default:
		return "", fmt.Errorf("'%s' is not recognized as an internal or external command, operable program or batch file.", trimmed)
	}
}

func whoamiAll(p WindowsUser) string {
	var sb strings.Builder
	sb.WriteString("USER INFORMATION\r\n")
	sb.WriteString("----------------\r\n")
	sb.WriteString(fmt.Sprintf("User Name           SID\r\n"))
	sb.WriteString(fmt.Sprintf("=================== =============================================\r\n"))
	sb.WriteString(fmt.Sprintf("%s\\%s %s\r\n\r\n", p.Domain, p.Username, p.SID))

	sb.WriteString("GROUP INFORMATION\r\n")
	sb.WriteString("-----------------\r\n")
	sb.WriteString("Group Name                                 Type             SID          Attributes\r\n")
	sb.WriteString("========================================== ================ ============ ==================================================\r\n")
	for _, g := range p.Groups {
		sb.WriteString(fmt.Sprintf("%-42s Well-known group S-1-1-0      Mandatory group, Enabled by default, Enabled group\r\n", g))
	}
	sb.WriteString("\r\nPRIVILEGES INFORMATION\r\n")
	sb.WriteString("----------------------\r\n")
	sb.WriteString("Privilege Name                Description                          State\r\n")
	sb.WriteString("============================= ==================================== ========\r\n")
	sb.WriteString("SeChangeNotifyPrivilege       Bypass traverse checking             Enabled\r\n")
	sb.WriteString("SeIncreaseWorkingSetPrivilege Increase a process working set       Disabled\r\n")
	return sb.String()
}

func ipconfigOutput(w *WindowsState) string {
	n := w.Network
	return fmt.Sprintf(
		"Windows IP Configuration\r\n\r\nEthernet adapter %s:\r\n   Connection-specific DNS Suffix  . : %s\r\n   IPv4 Address. . . . . . . . . . . : %s\r\n   Subnet Mask . . . . . . . . . . . : %s\r\n   Default Gateway . . . . . . . . . : %s\r\n",
		n.AdapterName, n.DNSSuffix, n.IP, n.Subnet, n.Gateway,
	)
}

func sysinfoOutput(s *SystemState) string {
	w := s.Windows
	return fmt.Sprintf(
		"Host Name:                 %s\r\nOS Name:                   %s\r\nOS Version:                %s N/A Build %s\r\nOS Manufacturer:           Microsoft Corporation\r\nSystem Type:               x64-based PC\r\nDomain:                    %s\r\n",
		s.Hostname, w.OSName, w.OSVersion, w.BuildNumber, w.Domain,
	)
}

func (s *SystemState) ReadRegistry(key string) (string, error) {
	if s.Windows == nil {
		return "", fmt.Errorf("ERROR: The system was unable to find the specified registry key or value.")
	}
	w := s.Windows
	upper := strings.ToUpper(key)

	switch {
	case strings.Contains(upper, "CURRENTVERSION"):
		domainShort := strings.ToUpper(strings.Split(w.Domain, ".")[0])
		return fmt.Sprintf(
			"HKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\r\n    ProductName    REG_SZ    %s\r\n    CurrentBuild   REG_SZ    %s\r\n    RegisteredOrganization    REG_SZ    %s\r\n    InstallDate    REG_DWORD    0x5e1d4f80\r\n",
			w.OSName, w.BuildNumber, domainShort,
		), nil

	case strings.Contains(upper, "ENVIRONMENT"):
		return "HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment\r\n    Path    REG_EXPAND_SZ    %SystemRoot%\\system32;%SystemRoot%;C:\\Program Files\\app\\bin\r\n    TEMP    REG_EXPAND_SZ    %SystemRoot%\\Temp\r\n    ComSpec REG_EXPAND_SZ    %SystemRoot%\\system32\\cmd.exe\r\n", nil

	default:
		return "", fmt.Errorf("ERROR: The system was unable to find the specified registry key or value.")
	}
}

func (s *SystemState) QueryWMI(class string) (string, error) {
	if s.Windows == nil {
		return "", fmt.Errorf("Get-WmiObject : Not supported\r\nAt line:1 char:1\r\n")
	}
	w := s.Windows
	lower := strings.ToLower(class)

	switch lower {
	case "win32_computersystem":
		return fmt.Sprintf(
			"Domain          : %s\r\nManufacturer    : VMware, Inc.\r\nModel           : VMware Virtual Platform\r\nName            : %s\r\nTotalPhysicalMemory : 8589934592\r\nUserName        : %s\\%s\r\n",
			w.Domain, s.Hostname, w.PrimaryUser.Domain, w.PrimaryUser.Username,
		), nil

	case "win32_operatingsystem":
		return fmt.Sprintf(
			"Caption         : %s\r\nCSName          : %s\r\nVersion         : %s\r\nBuildNumber     : %s\r\nOSArchitecture  : 64-bit\r\nLastBootUpTime  : %s\r\n",
			w.OSName, s.Hostname, w.OSVersion, w.BuildNumber, w.LastBootUpTime,
		), nil

	default:
		return "", fmt.Errorf("Get-WmiObject : Not supported\r\nAt line:1 char:1\r\n")
	}
}

func (s *SystemState) GetProcessInfo() string {
	if s.Windows == nil {
		return "Image Name                     PID Session Name        Session#    Mem Usage Status          User Name                 CPU Time Window Title\r\n" +
			"========================= ======== ================ =========== ============ =============== ========================= ======== =========================\r\n" +
			"System                           4 Services                   0      8,284 K Unknown         NT AUTHORITY\\SYSTEM       0:01:12 N/A\r\n"
	}

	user := fmt.Sprintf("%s\\%s", s.Windows.PrimaryUser.Domain, s.Windows.PrimaryUser.Username)
	rows := []struct {
		image   string
		pid     int
		session string
		num     int
		mem     string
		status  string
		user    string
		cpu     string
		title   string
	}{
		{"System Idle Process", 0, "Services", 0, "8 K", "Unknown", "NT AUTHORITY\\SYSTEM", "42:18:09", "N/A"},
		{"System", 4, "Services", 0, "8,284 K", "Unknown", "NT AUTHORITY\\SYSTEM", "0:01:12", "N/A"},
		{"Registry", 108, "Services", 0, "34,112 K", "Unknown", "NT AUTHORITY\\SYSTEM", "0:00:03", "N/A"},
		{"smss.exe", 312, "Services", 0, "1,176 K", "Unknown", "NT AUTHORITY\\SYSTEM", "0:00:00", "N/A"},
		{"csrss.exe", 512, "Services", 0, "5,380 K", "Unknown", "NT AUTHORITY\\SYSTEM", "0:00:09", "N/A"},
		{"wininit.exe", 588, "Services", 0, "6,144 K", "Unknown", "NT AUTHORITY\\SYSTEM", "0:00:01", "N/A"},
		{"services.exe", 672, "Services", 0, "9,216 K", "Running", "NT AUTHORITY\\SYSTEM", "0:00:19", "N/A"},
		{"lsass.exe", 680, "Services", 0, "21,504 K", "Running", "NT AUTHORITY\\SYSTEM", "0:00:31", "N/A"},
		{"svchost.exe", 892, "Services", 0, "14,336 K", "Running", "NT AUTHORITY\\LOCAL SERVICE", "0:00:04", "N/A"},
		{"svchost.exe", 1024, "Services", 0, "35,840 K", "Running", "NT AUTHORITY\\NETWORK SERVICE", "0:00:38", "N/A"},
		{"WmiPrvSE.exe", 1392, "Services", 0, "22,188 K", "Running", "NT AUTHORITY\\NETWORK SERVICE", "0:00:07", "N/A"},
		{"spoolsv.exe", 1516, "Services", 0, "12,704 K", "Running", "NT AUTHORITY\\SYSTEM", "0:00:02", "N/A"},
		{"explorer.exe", 2048, "Console", 1, "78,912 K", "Running", user, "0:01:55", "Program Manager"},
		{"app.exe", 2412, "Services", 0, "142,336 K", "Running", user, "0:04:27", "N/A"},
		{"order-worker.exe", 2524, "Services", 0, "88,104 K", "Running", user, "0:02:18", "N/A"},
		{"sqlservr.exe", 2688, "Services", 0, "512,000 K", "Running", "NT SERVICE\\MSSQLSERVER", "0:12:49", "N/A"},
		{"conhost.exe", 3104, "Console", 1, "7,812 K", "Running", user, "0:00:00", "N/A"},
		{"powershell.exe", 3188, "Console", 1, "64,908 K", "Running", user, "0:00:03", "Administrator: Windows PowerShell"},
	}

	var sb strings.Builder
	sb.WriteString("Image Name                     PID Session Name        Session#    Mem Usage Status          User Name                 CPU Time Window Title\r\n")
	sb.WriteString("========================= ======== ================ =========== ============ =============== ========================= ======== =========================\r\n")
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf("%-25s %8d %-16s %11d %12s %-15s %-25s %8s %s\r\n",
			row.image, row.pid, row.session, row.num, row.mem, row.status, row.user, row.cpu, row.title))
	}
	return sb.String()
}

// windowsEventLog returns a simulated Windows event log entry for the given eventID.
// Used by Phase 4/5 for AI-generated and canary-injected responses.
func windowsEventLog(s *SystemState, eventID int) string {
	w := s.Windows
	switch eventID {
	case 4624: // Security: successful logon
		return fmt.Sprintf(
			"Log Name:      Security\r\nSource:        Microsoft-Windows-Security-Auditing\r\nEvent ID:      4624\r\nTask Category: Logon\r\nLevel:         Information\r\nComputer:      %s\r\nDescription:\r\nAn account was successfully logged on.\r\n\r\nSubject:\r\n  Security ID:  SYSTEM\r\n  Account Name: %s$\r\n\r\nNew Logon:\r\n  Security ID:  %s\r\n  Account Name: %s\r\n  Account Domain: %s\r\n  Logon Type: 3\r\n",
			s.Hostname, s.Hostname,
			w.PrimaryUser.SID, w.PrimaryUser.Username, w.PrimaryUser.Domain,
		)
	case 7036: // System: service state change
		return fmt.Sprintf(
			"Log Name:      System\r\nSource:        Service Control Manager\r\nEvent ID:      7036\r\nLevel:         Information\r\nComputer:      %s\r\nDescription:\r\nThe Windows Update service entered the running state.\r\n",
			s.Hostname,
		)
	default:
		return ""
	}
}
