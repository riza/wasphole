package sim

import (
	"fmt"
	"strings"
	"time"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

type windowsVersion struct {
	OSName      string
	OSVersion   string
	BuildNumber string
}

var windowsVersions = []windowsVersion{
	{"Microsoft Windows Server 2022 Datacenter", "10.0.20348", "20348"},
	{"Microsoft Windows Server 2022 Standard", "10.0.20348", "20348"},
	{"Microsoft Windows Server 2019 Datacenter", "10.0.17763", "17763"},
	{"Microsoft Windows Server 2019 Standard", "10.0.17763", "17763"},
	{"Microsoft Windows Server 2016 Datacenter", "10.0.14393", "14393"},
	{"Microsoft Windows 11 Pro", "10.0.22631", "22631"},
	{"Microsoft Windows 10 Pro", "10.0.19045", "19045"},
}

var domainSuffixes = []string{"corp", "ad", "internal", "lan", "local"}

func generateWindowsState(fake *gofakeit.Faker, s *SystemState) *WindowsState {
	ver := windowsVersions[fake.IntRange(0, len(windowsVersions)-1)]

	// Generate a realistic domain from a fake company name
	company := strings.ToLower(fake.Company())
	company = strings.ReplaceAll(company, " ", "")
	company = strings.ReplaceAll(company, ",", "")
	company = strings.ReplaceAll(company, ".", "")
	company = strings.ReplaceAll(company, "-", "")
	if len(company) > 10 {
		company = company[:10]
	}
	suffix := domainSuffixes[fake.IntRange(0, len(domainSuffixes)-1)]
	domain := company + "." + suffix
	domainShort := strings.ToUpper(company)
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
		OSName:         ver.OSName,
		OSVersion:      ver.OSVersion,
		BuildNumber:    ver.BuildNumber,
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

	// ── Identity / host ──────────────────────────────────────────
	case lower == "hostname":
		return s.Hostname + "\r\n", nil

	case lower == "whoami":
		return fmt.Sprintf("%s\\%s\r\n", p.Domain, p.Username), nil

	case lower == "whoami /all", lower == "whoami/all":
		return whoamiAll(p), nil

	case lower == "ver":
		return fmt.Sprintf("\r\nMicrosoft Windows [Version %s]\r\n", w.OSVersion), nil

	// ── Network ──────────────────────────────────────────────────
	case strings.HasPrefix(lower, "ipconfig"):
		if strings.Contains(lower, "/all") {
			return ipconfigAll(w), nil
		}
		return ipconfigOutput(w), nil

	case strings.HasPrefix(lower, "netstat"):
		return netstatOutput(s), nil

	case strings.HasPrefix(lower, "ping"):
		host := strings.TrimSpace(strings.TrimPrefix(lower, "ping"))
		host = strings.Fields(host)[0]
		return fmt.Sprintf("\r\nPinging %s with 32 bytes of data:\r\nReply from %s: bytes=32 time=1ms TTL=128\r\nReply from %s: bytes=32 time<1ms TTL=128\r\n\r\nPing statistics for %s:\r\n    Packets: Sent = 2, Received = 2, Lost = 0 (0%% loss),\r\n", host, host, host, host), nil

	case strings.HasPrefix(lower, "arp"):
		return arpOutput(w), nil

	case strings.HasPrefix(lower, "nslookup"):
		return fmt.Sprintf("Server:  dc01.%s\r\nAddress:  10.0.0.53\r\n\r\nName:    %s\r\nAddress: %s\r\n", w.Domain, s.Hostname+"."+w.Domain, w.Network.IP), nil

	// ── Filesystem ───────────────────────────────────────────────
	case lower == "dir" || lower == "ls":
		return dirOutput(s, "")

	case strings.HasPrefix(lower, "dir ") || strings.HasPrefix(lower, "ls "):
		arg := strings.TrimSpace(trimmed[4:])
		return dirOutput(s, arg)

	case lower == "cd" || lower == "pwd":
		return fmt.Sprintf("C:\\Users\\%s\r\n", p.Username), nil

	case strings.HasPrefix(lower, "type ") || strings.HasPrefix(lower, "cat "):
		path := strings.TrimSpace(trimmed[5:])
		// delegate to ReadFile for known paths; otherwise simulate common files
		content, err := s.readWindowsFile(path)
		if err != nil {
			return "", fmt.Errorf("The system cannot find the file specified.")
		}
		return content, nil

	// ── Users / groups ───────────────────────────────────────────
	case lower == "net user":
		return netUserList(w), nil

	case strings.HasPrefix(lower, "net user "):
		username := strings.TrimSpace(lower[9:])
		return netUserDetail(w, username), nil

	case lower == "net localgroup":
		return netLocalgroupList(w), nil

	case strings.HasPrefix(lower, "net localgroup "):
		group := strings.TrimSpace(trimmed[15:])
		return netLocalgroupDetail(w, group), nil

	case lower == "net use":
		return "New connections will be remembered.\r\n\r\nThere are no entries in the list.\r\n", nil

	case lower == "net share":
		return netShareOutput(s), nil

	// ── Processes / services ─────────────────────────────────────
	case strings.HasPrefix(lower, "tasklist"):
		return s.GetProcessInfo(), nil

	case strings.HasPrefix(lower, "sc query"):
		return scQueryOutput(), nil

	case strings.HasPrefix(lower, "sc qc "):
		svc := strings.TrimSpace(trimmed[6:])
		return scQC(svc), nil

	// ── System info ──────────────────────────────────────────────
	case strings.HasPrefix(lower, "systeminfo"):
		return sysinfoOutput(s), nil

	case lower == "set":
		return setOutput(s), nil

	case strings.HasPrefix(lower, "echo "):
		arg := strings.TrimSpace(trimmed[5:])
		return echoExpand(s, arg) + "\r\n", nil

	// ── PowerShell cmdlets ───────────────────────────────────────
	case strings.HasPrefix(lower, "get-process"):
		return s.GetProcessInfo(), nil

	case strings.HasPrefix(lower, "get-service"):
		return psGetService(), nil

	case strings.HasPrefix(lower, "get-localuser"):
		return psGetLocalUser(w), nil

	case strings.HasPrefix(lower, "get-computerinfo"):
		return psGetComputerInfo(s), nil

	case strings.HasPrefix(lower, "get-netipaddress"):
		return psGetNetIPAddress(w), nil

	case lower == "get-date":
		return fmt.Sprintf("\r\n%-20s\r\n---------\r\n%s\r\n", "Thursday, January 1, 2026 12:00:00 AM", time.Now().Format("Monday, January 2, 2006 3:04:05 PM")), nil

	case lower == "$psversiontable" || lower == "$psversiontable | fl":
		return "PSVersion                      7.4.0\r\nPSEdition                      Core\r\nPSCompatibleVersions           {1.0, 2.0, 3.0, 4.0, 5.0, 5.1, 6.0, 7.0, 7.4.0}\r\nOS                             Microsoft Windows 10.0.20348\r\nPlatform                       Win32NT\r\n", nil

	case strings.HasPrefix(lower, "$env:"):
		varName := strings.ToUpper(strings.TrimPrefix(lower, "$env:"))
		return echoExpand(s, "%"+varName+"%") + "\r\n", nil

	// ── Registry ─────────────────────────────────────────────────
	case strings.HasPrefix(lower, "reg query"):
		key := strings.TrimSpace(trimmed[9:])
		return s.ReadRegistry(key)

	default:
		return "", fmt.Errorf("'%s' is not recognized as an internal or external command, operable program or batch file.", trimmed)
	}
}

// ── Command output helpers ────────────────────────────────────────────────────

func dirOutput(s *SystemState, path string) (string, error) {
	p := s.Windows.PrimaryUser
	userHome := fmt.Sprintf(`C:\Users\%s`, p.Username)

	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(path), "/", `\`))
	// strip flags like /s /b /w
	for _, flag := range []string{"/S", "/B", "/W", "/A", "/O", "/P"} {
		normalized = strings.ReplaceAll(normalized, flag, "")
	}
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		normalized = userHome
	}

	type entry struct {
		isDir bool
		name  string
		size  string
	}

	var entries []entry
	label := "Windows"
	serial := "A3F2-09B1"

	switch {
	case normalized == `C:\` || normalized == "C:":
		entries = []entry{
			{true, "PerfLogs", ""},
			{true, "Program Files", ""},
			{true, "Program Files (x86)", ""},
			{true, "Users", ""},
			{true, "Windows", ""},
		}
	case normalized == `C:\USERS`:
		entries = []entry{
			{true, ".", ""},
			{true, "..", ""},
			{true, "Administrator", ""},
			{true, p.Username, ""},
			{true, "Public", ""},
		}
	case normalized == strings.ToUpper(userHome):
		entries = []entry{
			{true, ".", ""},
			{true, "..", ""},
			{true, "AppData", ""},
			{true, "Desktop", ""},
			{true, "Documents", ""},
			{true, "Downloads", ""},
			{true, ".ssh", ""},
			{false, ".bash_history", "1,024"},
			{false, "notes.txt", "512"},
		}
	case normalized == strings.ToUpper(userHome+`\.SSH`):
		entries = []entry{
			{true, ".", ""},
			{true, "..", ""},
			{false, "id_rsa", "1,674"},
			{false, "id_rsa.pub", "394"},
			{false, "known_hosts", "884"},
			{false, "config", "256"},
		}
	case strings.HasPrefix(normalized, `C:\WINDOWS`):
		entries = []entry{
			{true, ".", ""},
			{true, "..", ""},
			{true, "System32", ""},
			{true, "SysWOW64", ""},
			{true, "Temp", ""},
			{true, "Logs", ""},
		}
		label = "System"
	case strings.HasPrefix(normalized, `C:\PROGRAM FILES`):
		entries = []entry{
			{true, ".", ""},
			{true, "..", ""},
			{true, "Common Files", ""},
			{true, "Internet Explorer", ""},
			{true, "Windows Defender", ""},
			{true, "app", ""},
		}
	default:
		return "", fmt.Errorf("The system cannot find the path specified.")
	}

	now := time.Now()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(" Volume in drive C is %s\r\n Volume Serial Number is %s\r\n\r\n", label, serial))
	sb.WriteString(fmt.Sprintf(" Directory of %s\r\n\r\n", normalized))

	dirs, files := 0, 0
	for _, e := range entries {
		ts := now.Add(-24 * time.Hour).Format("01/02/2006  03:04 PM")
		if e.isDir {
			sb.WriteString(fmt.Sprintf("%-20s    <DIR>          %s\r\n", ts, e.name))
			dirs++
		} else {
			sb.WriteString(fmt.Sprintf("%-20s %16s %s\r\n", ts, e.size, e.name))
			files++
		}
	}
	sb.WriteString(fmt.Sprintf("%16d File(s)              0 bytes\r\n", files))
	sb.WriteString(fmt.Sprintf("%16d Dir(s)  45,231,616,000 bytes free\r\n", dirs))
	return sb.String(), nil
}

func (s *SystemState) readWindowsFile(path string) (string, error) {
	upper := strings.ToUpper(strings.ReplaceAll(path, "/", `\`))
	p := s.Windows.PrimaryUser
	switch {
	case strings.HasSuffix(upper, "NOTES.TXT"):
		return fmt.Sprintf("Server: %s\r\nOwner: %s\\%s\r\nSetup date: 2025-03-15\r\n", s.Hostname, p.Domain, p.Username), nil
	case strings.HasSuffix(upper, ".BASH_HISTORY"):
		return "ipconfig\r\nwhoami\r\nnet user\r\ndir C:\\\r\n", nil
	default:
		return "", fmt.Errorf("not found")
	}
}

func ipconfigAll(w *WindowsState) string {
	n := w.Network
	return fmt.Sprintf(
		"Windows IP Configuration\r\n\r\n   Host Name . . . . . . . . . . . . : %s\r\n   Primary Dns Suffix  . . . . . . . : %s\r\n   Node Type . . . . . . . . . . . . : Hybrid\r\n   IP Routing Enabled. . . . . . . . : No\r\n\r\nEthernet adapter %s:\r\n   Connection-specific DNS Suffix  . : %s\r\n   Description . . . . . . . . . . . : vmxnet3 Ethernet Adapter\r\n   Physical Address. . . . . . . . . : %s\r\n   DHCP Enabled. . . . . . . . . . . : No\r\n   IPv4 Address. . . . . . . . . . . : %s\r\n   Subnet Mask . . . . . . . . . . . : %s\r\n   Default Gateway . . . . . . . . . : %s\r\n   DNS Servers . . . . . . . . . . . : %s\r\n",
		strings.ToLower(w.ComputerName), w.Domain,
		n.AdapterName, n.DNSSuffix, n.MAC, n.IP, n.Subnet, n.Gateway, n.DNS,
	)
}

func netstatOutput(s *SystemState) string {
	ip := s.Windows.Network.IP
	return fmt.Sprintf(
		"\r\nActive Connections\r\n\r\n  Proto  Local Address          Foreign Address        State\r\n"+
			"  TCP    %s:445        0.0.0.0:0              LISTENING\r\n"+
			"  TCP    %s:3389       0.0.0.0:0              LISTENING\r\n"+
			"  TCP    %s:8080       0.0.0.0:0              LISTENING\r\n"+
			"  TCP    %s:49152      %s:52341     ESTABLISHED\r\n"+
			"  TCP    %s:49153      10.0.0.53:53           TIME_WAIT\r\n",
		ip, ip, ip, ip, s.Windows.Network.Gateway, ip,
	)
}

func arpOutput(w *WindowsState) string {
	n := w.Network
	gMAC := "00-50-56-e0-00-01"
	return fmt.Sprintf(
		"\r\nInterface: %s --- 0x3\r\n  Internet Address      Physical Address      Type\r\n  %s          %s   dynamic\r\n  %s         %s   dynamic\r\n",
		n.IP, n.Gateway, gMAC, n.IP, n.MAC,
	)
}

func netUserList(w *WindowsState) string {
	return fmt.Sprintf(
		"\r\nUser accounts for \\\\%s\r\n\r\n-------------------------------------------------------------------------------\r\nAdministrator            DefaultAccount           Guest\r\n%s                   WDAGUtilityAccount\r\nThe command completed successfully.\r\n",
		w.ComputerName, w.PrimaryUser.Username,
	)
}

func netUserDetail(w *WindowsState, username string) string {
	p := w.PrimaryUser
	if username != strings.ToLower(p.Username) && username != "administrator" {
		return fmt.Sprintf("The user name could not be found.\r\n\r\nMore help is available by typing NET HELPMSG 2221.\r\n")
	}
	name := p.Username
	if username == "administrator" {
		name = "Administrator"
	}
	return fmt.Sprintf(
		"User name                    %s\r\nFull Name                    %s\r\nComment                      Built-in account for administering the computer/domain\r\nUser's comment\r\nCountry/region code          000 (System Default)\r\nAccount active               Yes\r\nAccount expires              Never\r\n\r\nPassword last set            3/15/2025 9:00:00 AM\r\nPassword expires             Never\r\nPassword changeable          3/15/2025 9:00:00 AM\r\nPassword required            Yes\r\nUser may change password     Yes\r\n\r\nWorkstations allowed         All\r\nLogon script\r\nUser profile\r\nHome directory\r\nLast logon                   %s\r\n\r\nLogon hours allowed          All\r\n\r\nLocal Group Memberships      *%s\r\nGlobal Group memberships     *Domain Users\r\nThe command completed successfully.\r\n",
		name, name, time.Now().Format("1/2/2006 3:04:05 PM"), strings.Join(p.Groups, " *"),
	)
}

func netLocalgroupList(w *WindowsState) string {
	return fmt.Sprintf(
		"\r\nAliases for \\\\%s\r\n\r\n-------------------------------------------------------------------------------\r\n*Administrators\r\n*Backup Operators\r\n*Guests\r\n*Network Configuration Operators\r\n*Remote Desktop Users\r\n*Remote Management Users\r\n*Users\r\nThe command completed successfully.\r\n",
		w.ComputerName,
	)
}

func netLocalgroupDetail(w *WindowsState, group string) string {
	p := w.PrimaryUser
	members := ""
	if strings.EqualFold(group, "administrators") {
		members = fmt.Sprintf("%s\\Administrator\r\n%s\\%s\r\n", w.ComputerName, p.Domain, p.Username)
	} else if strings.EqualFold(group, "users") || strings.EqualFold(group, "\"users\"") {
		members = fmt.Sprintf("%s\\Domain Users\r\n%s\\%s\r\n", p.Domain, p.Domain, p.Username)
	}
	return fmt.Sprintf(
		"\r\nAlias name     %s\r\nComment\r\n\r\nMembers\r\n\r\n-------------------------------------------------------------------------------\r\n%sThe command completed successfully.\r\n",
		group, members,
	)
}

func netShareOutput(s *SystemState) string {
	return fmt.Sprintf(
		"Share name   Resource                        Remark\r\n\r\n-------------------------------------------------------------------------------\r\nC$           C:\\                              Default share\r\nIPC$                                          Remote IPC\r\nADMIN$       C:\\Windows                       Remote Admin\r\nThe command completed successfully.\r\n",
	)
}

func scQueryOutput() string {
	services := []struct{ name, display, state string }{
		{"wuauserv", "Windows Update", "RUNNING"},
		{"winrm", "Windows Remote Management", "RUNNING"},
		{"w32tm", "Windows Time", "RUNNING"},
		{"spooler", "Print Spooler", "RUNNING"},
		{"schedule", "Task Scheduler", "RUNNING"},
		{"eventlog", "Windows Event Log", "RUNNING"},
		{"lmhosts", "TCP/IP NetBIOS Helper", "STOPPED"},
	}
	var sb strings.Builder
	for _, svc := range services {
		sb.WriteString(fmt.Sprintf("\r\nSERVICE_NAME: %s\r\n", svc.name))
		sb.WriteString(fmt.Sprintf("DISPLAY_NAME: %s\r\n", svc.display))
		sb.WriteString(fmt.Sprintf("        TYPE               : 20  WIN32_SHARE_PROCESS\r\n"))
		state := "4  RUNNING"
		if svc.state == "STOPPED" {
			state = "1  STOPPED"
		}
		sb.WriteString(fmt.Sprintf("        STATE              : %s\r\n", state))
	}
	return sb.String()
}

func scQC(svc string) string {
	return fmt.Sprintf(
		"[SC] QueryServiceConfig SUCCESS\r\n\r\nSERVICE_NAME: %s\r\n        TYPE               : 20  WIN32_SHARE_PROCESS\r\n        START_TYPE         : 2   AUTO_START\r\n        ERROR_CONTROL      : 1   NORMAL\r\n        BINARY_PATH_NAME   : C:\\Windows\\system32\\svchost.exe -k netsvcs -p\r\n        LOAD_ORDER_GROUP\r\n        TAG                : 0\r\n        DISPLAY_NAME       : %s\r\n        DEPENDENCIES\r\n        SERVICE_START_NAME : LocalSystem\r\n",
		svc, svc,
	)
}

func setOutput(s *SystemState) string {
	w := s.Windows
	p := w.PrimaryUser
	return fmt.Sprintf(
		"ALLUSERSPROFILE=C:\\ProgramData\r\nAPPDATA=C:\\Users\\%s\\AppData\\Roaming\r\nCOMPUTERNAME=%s\r\nComSpec=C:\\Windows\\system32\\cmd.exe\r\nHOMEPATH=\\Users\\%s\r\nLOCALAPPDATA=C:\\Users\\%s\\AppData\\Local\r\nLOGONSERVER=\\\\DC01\r\nNUMBER_OF_PROCESSORS=4\r\nOS=Windows_NT\r\nPath=C:\\Windows\\system32;C:\\Windows;C:\\Windows\\System32\\Wbem;C:\\Program Files\\app\\bin\r\nPATHEXT=.COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC\r\nPROCESSOR_ARCHITECTURE=AMD64\r\nPROGRAMFILES=C:\\Program Files\r\nSYSTEMDRIVE=C:\r\nSYSTEMROOT=C:\\Windows\r\nTEMP=C:\\Windows\\Temp\r\nTMP=C:\\Windows\\Temp\r\nUSERDOMAIN=%s\r\nUSERNAME=%s\r\nUSERPROFILE=C:\\Users\\%s\r\nwindir=C:\\Windows\r\n",
		p.Username, w.ComputerName, p.Username, p.Username, p.Domain, p.Username, p.Username,
	)
}

func echoExpand(s *SystemState, arg string) string {
	w := s.Windows
	p := w.PrimaryUser
	r := strings.NewReplacer(
		"%USERNAME%", p.Username,
		"%USERDOMAIN%", p.Domain,
		"%COMPUTERNAME%", w.ComputerName,
		"%SYSTEMROOT%", `C:\Windows`,
		"%SYSTEMDRIVE%", "C:",
		"%TEMP%", `C:\Windows\Temp`,
		"%TMP%", `C:\Windows\Temp`,
		"%APPDATA%", fmt.Sprintf(`C:\Users\%s\AppData\Roaming`, p.Username),
		"%USERPROFILE%", fmt.Sprintf(`C:\Users\%s`, p.Username),
		"%PROCESSOR_ARCHITECTURE%", "AMD64",
		"%OS%", "Windows_NT",
		"%PROGRAMFILES%", `C:\Program Files`,
		"%PATH%", `C:\Windows\system32;C:\Windows;C:\Program Files\app\bin`,
	)
	return r.Replace(arg)
}

func psGetService() string {
	var sb strings.Builder
	sb.WriteString("\r\nStatus   Name               DisplayName\r\n")
	sb.WriteString("------   ----               -----------\r\n")
	services := []struct{ status, name, display string }{
		{"Running", "wuauserv", "Windows Update"},
		{"Running", "WinRM", "Windows Remote Management (WS-Manag..."},
		{"Running", "Schedule", "Task Scheduler"},
		{"Running", "EventLog", "Windows Event Log"},
		{"Running", "Spooler", "Print Spooler"},
		{"Stopped", "lmhosts", "TCP/IP NetBIOS Helper"},
	}
	for _, s := range services {
		sb.WriteString(fmt.Sprintf("%-8s %-18s %s\r\n", s.status, s.name, s.display))
	}
	return sb.String()
}

func psGetLocalUser(w *WindowsState) string {
	p := w.PrimaryUser
	return fmt.Sprintf(
		"\r\nName               Enabled Description\r\n----               ------- -----------\r\nAdministrator      True    Built-in account for administering\r\nDefaultAccount     False   A user account managed by the system.\r\nGuest              False   Built-in account for guest access\r\n%s             True    Service deployment account\r\n",
		p.Username,
	)
}

func psGetComputerInfo(s *SystemState) string {
	w := s.Windows
	return fmt.Sprintf(
		"WindowsProductName      : %s\r\nWindowsVersion          : %s\r\nWindowsBuildLabEx       : %s.amd64fre\r\nCsName                  : %s\r\nCsDomain                : %s\r\nCsManufacturer          : VMware, Inc.\r\nCsModel                 : VMware Virtual Platform\r\nCsNumberOfLogicalProcessors : 4\r\nCsTotalPhysicalMemory   : 8589934592\r\n",
		w.OSName, w.OSVersion, w.BuildNumber, s.Hostname, w.Domain,
	)
}

func psGetNetIPAddress(w *WindowsState) string {
	n := w.Network
	return fmt.Sprintf(
		"\r\nIPAddress         : %s\r\nInterfaceAlias    : %s\r\nInterfaceIndex    : 3\r\nIPVersion         : IPv4\r\nPrefixLength      : 24\r\nType              : Unicast\r\n",
		n.IP, n.AdapterName,
	)
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
