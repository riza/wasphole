package sim

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"text/template"
	"time"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

var kernelVersions = []string{
	"5.15.0-91-generic",
	"5.15.0-105-generic",
	"6.5.0-45-generic",
}

func generateLinuxState(fake *gofakeit.Faker, s *SystemState) *LinuxState {
	kernelIdx := fake.IntRange(0, len(kernelVersions)-1)
	kernel := kernelVersions[kernelIdx]

	octets := [4]int{10, fake.IntRange(1, 5), fake.IntRange(1, 3), fake.IntRange(10, 200)}
	ip := fmt.Sprintf("%d.%d.%d.%d", octets[0], octets[1], octets[2], octets[3])
	gateway := fmt.Sprintf("%d.%d.%d.1", octets[0], octets[1], octets[2])
	mac := fake.MacAddress()

	memTotal := fake.IntRange(4, 32) * 1024 * 1024

	// Build user list
	svcUsers := []LinuxUser{
		{Username: "svc-app", Gecos: "Application Service", Home: "/opt/app", Shell: "/usr/sbin/nologin", UID: 1000, GID: 1000, IsSystem: true},
	}
	appUser := LinuxUser{
		Username: fake.Username(),
		Gecos:    "App User",
		Home:     "/home/" + fake.Username(),
		Shell:    "/bin/bash",
		UID:      1001,
		GID:      1001,
		IsSystem: false,
	}
	// Re-generate consistently
	appUsername := strings.ToLower(fake.FirstName())
	appUser = LinuxUser{
		Username: appUsername,
		Gecos:    "Application User",
		Home:     "/home/" + appUsername,
		Shell:    "/bin/bash",
		UID:      1001,
		GID:      1001,
		IsSystem: false,
	}

	allUsers := []LinuxUser{
		{Username: "root", Gecos: "root", Home: "/root", Shell: "/bin/bash", UID: 0, GID: 0, IsSystem: true},
		{Username: "daemon", Gecos: "daemon", Home: "/usr/sbin", Shell: "/usr/sbin/nologin", UID: 1, GID: 1, IsSystem: true},
		{Username: "bin", Gecos: "bin", Home: "/bin", Shell: "/usr/sbin/nologin", UID: 2, GID: 2, IsSystem: true},
		{Username: "www-data", Gecos: "www-data", Home: "/var/www", Shell: "/usr/sbin/nologin", UID: 33, GID: 33, IsSystem: true},
		svcUsers[0],
		appUser,
	}

	return &LinuxState{
		KernelVersion: kernel,
		OSName:        "Ubuntu",
		OSVersion:     "22.04.3 LTS (Jammy Jellyfish)",
		OSVersionID:   "22.04",
		OSCodename:    "Jammy Jellyfish",
		GCCVersion:    "gcc (Ubuntu 11.4.0-1ubuntu1~22.04) 11.4.0",
		BuildDate:     "Tue Nov 14 13:30:08 UTC 2023",
		Users:         allUsers,
		PrimaryUser:   appUser,
		Network: LinuxNetwork{
			Interface: "eth0",
			IP:        ip,
			MAC:       mac,
			Gateway:   gateway,
			DNS:       []string{"8.8.8.8", "8.8.4.4"},
		},
		MemTotalKB:     memTotal,
		MemFreeKB:      memTotal / 8,
		MemAvailKB:     memTotal / 4,
		SwapTotalKB:    memTotal / 2,
		ListeningPorts: []int{22, 80, 443, 5432},
	}
}

var passwdTmpl = template.Must(template.New("passwd").Parse(
	`root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
bin:x:2:2:bin:/bin:/usr/sbin/nologin
www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin
{{- range .}}
{{.Username}}:x:{{.UID}}:{{.GID}}:{{.Gecos}}:{{.Home}}:{{.Shell}}
{{- end}}
`))

func (s *SystemState) ReadFile(path string) (string, error) {
	if s.Linux == nil {
		return "", fmt.Errorf("open %s: no such file or directory", path)
	}
	l := s.Linux

	switch path {
	case "/etc/hostname":
		return s.Hostname + "\n", nil

	case "/etc/machine-id":
		return s.MachineID + "\n", nil

	case "/etc/passwd":
		var appUsers []LinuxUser
		for _, u := range l.Users {
			if !u.IsSystem && u.UID >= 1001 {
				appUsers = append(appUsers, u)
			}
		}
		var buf bytes.Buffer
		if err := passwdTmpl.Execute(&buf, appUsers); err != nil {
			return "", err
		}
		return buf.String(), nil

	case "/etc/shadow":
		return "", fmt.Errorf("open /etc/shadow: permission denied")

	case "/proc/version":
		return fmt.Sprintf(
			"Linux version %s (buildd@lcy02-amd64-059) (%s, GNU ld (GNU Binutils for Ubuntu) 2.38) #101-Ubuntu SMP %s\n",
			l.KernelVersion, l.GCCVersion, l.BuildDate,
		), nil

	case "/etc/os-release":
		return fmt.Sprintf(
			`NAME="%s"
VERSION="%s"
ID=%s
ID_LIKE=debian
PRETTY_NAME="%s %s"
VERSION_ID="%s"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
VERSION_CODENAME=%s
`,
			l.OSName,
			l.OSVersion,
			strings.ToLower(l.OSName),
			l.OSName, l.OSVersion,
			l.OSVersionID,
			strings.ToLower(l.OSCodename),
		), nil

	case "/proc/uptime":
		up := time.Since(s.BootTime).Seconds()
		return fmt.Sprintf("%.2f %.2f\n", up, up*3.9), nil

	case "/proc/net/tcp":
		return procNetTCP(s), nil

	case "/proc/meminfo":
		return fmt.Sprintf(
			"MemTotal:       %8d kB\nMemFree:        %8d kB\nMemAvailable:   %8d kB\nBuffers:        %8d kB\nCached:         %8d kB\nSwapTotal:      %8d kB\nSwapFree:       %8d kB\n",
			l.MemTotalKB, l.MemFreeKB, l.MemAvailKB,
			l.MemTotalKB/32, l.MemTotalKB/8,
			l.SwapTotalKB, l.SwapTotalKB-l.MemTotalKB/16,
		), nil

	case "/proc/self/environ":
		p := l.PrimaryUser
		parts := []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"HOME=" + p.Home,
			"USER=" + p.Username,
			"LANG=en_US.UTF-8",
			"APP_ENV=production",
			"DATABASE_URL=postgresql://app:secret@localhost:5432/appdb",
			"PORT=3000",
		}
		return strings.Join(parts, "\x00") + "\x00", nil

	case "/proc/1/cmdline":
		return "/sbin/init\x00", nil

	case "/var/log/auth.log":
		return varLogAuthLog(s), nil

	default:
		return "", fmt.Errorf("open %s: no such file or directory", path)
	}
}

func procNetTCP(s *SystemState) string {
	var sb strings.Builder
	sb.WriteString("  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n")
	for i, port := range s.Linux.ListeningPorts {
		// 0.0.0.0 in little-endian = 00000000
		sb.WriteString(fmt.Sprintf("  %2d: 00000000:%s 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 %d 1 0000000000000000 100 0 0 10 0\n",
			i, portToHex(port), 10000+i))
	}
	return sb.String()
}

func portToHex(port int) string {
	return fmt.Sprintf("%04X", port)
}

func ipToHex(ip string) string {
	parsed := net.ParseIP(ip).To4()
	if parsed == nil {
		return "00000000"
	}
	// little-endian: reverse byte order
	return fmt.Sprintf("%02X%02X%02X%02X", parsed[3], parsed[2], parsed[1], parsed[0])
}

func varLogAuthLog(s *SystemState) string {
	// All entries must predate BootTime
	var sb strings.Builder
	p := s.Linux.PrimaryUser
	hostname := s.Hostname

	entries := []struct {
		offset  time.Duration
		message string
	}{
		{30 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Accepted publickey for %s from 10.0.1.15 port 51234 ssh2", 2341, p.Username)},
		{29 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: pam_unix(sshd:session): session opened for user %s by (uid=0)", 2342, p.Username)},
		{28 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Failed password for root from 185.220.101.1 port 44321 ssh2", 2400)},
		{28*24*time.Hour + time.Minute, fmt.Sprintf("sshd[%d]: Failed password for root from 192.241.210.5 port 55123 ssh2", 2401)},
		{27 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Accepted publickey for %s from 10.0.1.20 port 61000 ssh2", 2500, p.Username)},
		{26 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: pam_unix(sshd:session): session closed for user %s", 2501, p.Username)},
		{20 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Failed password for invalid user admin from 45.33.32.156 port 33890 ssh2", 3100)},
		{15 * 24 * time.Hour, fmt.Sprintf("sudo: %s : TTY=pts/0 ; PWD=/home/%s ; USER=root ; COMMAND=/usr/bin/apt-get update", p.Username, p.Username)},
		{10 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Accepted publickey for %s from 10.0.1.15 port 52100 ssh2", 4200, p.Username)},
		{9 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: pam_unix(sshd:session): session opened for user %s by (uid=0)", 4201, p.Username)},
		{5 * 24 * time.Hour, fmt.Sprintf("sudo: %s : TTY=pts/1 ; PWD=/var/www ; USER=root ; COMMAND=/bin/systemctl restart nginx", p.Username)},
		{3 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Failed password for root from 185.220.101.33 port 45678 ssh2", 5000)},
		{2 * 24 * time.Hour, fmt.Sprintf("sshd[%d]: Accepted publickey for %s from 10.0.1.20 port 60001 ssh2", 5200, p.Username)},
		{36 * time.Hour, fmt.Sprintf("sshd[%d]: pam_unix(sshd:session): session closed for user %s", 5201, p.Username)},
		{2 * time.Hour, fmt.Sprintf("sshd[%d]: Accepted publickey for %s from 10.0.1.15 port 53000 ssh2", 6000, p.Username)},
	}

	for _, e := range entries {
		t := s.BootTime.Add(-e.offset)
		day := t.Day()
		dayStr := fmt.Sprintf("%2d", day)
		sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
			t.Format("Jan"), dayStr, t.Format("15:04:05"), hostname+" "+e.message))
	}
	return sb.String()
}

func (s *SystemState) ExecShell(cmd string) (string, error) {
	if s.Linux == nil {
		return "", fmt.Errorf("bash: %s: command not found", cmd)
	}
	l := s.Linux
	p := l.PrimaryUser
	cmd = strings.TrimSpace(cmd)
	lower := strings.ToLower(cmd)

	switch {
	case cmd == "hostname":
		return s.Hostname + "\n", nil

	case cmd == "whoami":
		return p.Username + "\n", nil

	case cmd == "id":
		return fmt.Sprintf("uid=%d(%s) gid=%d(%s) groups=%d(%s)\n",
			p.UID, p.Username, p.GID, p.Username, p.GID, p.Username), nil

	case cmd == "uname -r":
		return l.KernelVersion + "\n", nil

	case cmd == "uname -a":
		return fmt.Sprintf("Linux %s %s #101-Ubuntu SMP %s x86_64 x86_64 x86_64 GNU/Linux\n",
			s.Hostname, l.KernelVersion, l.BuildDate), nil

	case lower == "uptime":
		return uptimeString(s), nil

	case strings.HasPrefix(lower, "ps"):
		return s.ListProcesses(), nil

	case cmd == "ls /etc":
		return "apt\ncron.d\ncron.daily\ndefault\nfstab\nhosts\nhostname\nos-release\npasswd\nshadow\nssh\nssl\n", nil

	case cmd == "ls /var/www":
		return "html\nlogs\n", nil

	case strings.HasPrefix(lower, "ls"):
		// Extract path from command
		parts := strings.Fields(cmd)
		path := ""
		if len(parts) > 1 {
			path = parts[len(parts)-1]
		}
		return "", fmt.Errorf("ls: cannot access '%s': No such file or directory", path)

	default:
		return "", fmt.Errorf("bash: %s: command not found", cmd)
	}
}

func uptimeString(s *SystemState) string {
	up := time.Since(s.BootTime)
	days := int(up.Hours()) / 24
	hours := int(up.Hours()) % 24
	minutes := int(up.Minutes()) % 60
	now := time.Now().Format("15:04:05")
	return fmt.Sprintf(" %s up %d days, %2d:%02d,  1 user,  load average: 0.42, 0.38, 0.31\n",
		now, days, hours, minutes)
}

func (s *SystemState) ListProcesses() string {
	if s.Linux == nil {
		return "PID USER CMD\n1 root /sbin/init\n"
	}
	p := s.Linux.PrimaryUser
	return fmt.Sprintf("%-5s %-8s %s\n", "PID", "USER", "CMD") +
		fmt.Sprintf("%-5d %-8s %s\n", 1, "root", "/lib/systemd/systemd") +
		fmt.Sprintf("%-5d %-8s %s\n", 400, "root", "/usr/sbin/sshd -D") +
		fmt.Sprintf("%-5d %-8s %s\n", 500, "postgres", "/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main") +
		fmt.Sprintf("%-5d %-8s %s\n", 580, "www-data", "nginx: master process /usr/sbin/nginx -g daemon off;") +
		fmt.Sprintf("%-5d %-8s %s\n", 581, "www-data", "nginx: worker process") +
		fmt.Sprintf("%-5d %-8s %s\n", 610, p.Username, "node /app/current/server.js") +
		fmt.Sprintf("%-5d %-8s %s\n", 780, p.Username, "python3 /app/scripts/worker.py") +
		fmt.Sprintf("%-5d %-8s %s\n", 830, "root", "/usr/sbin/cron") +
		fmt.Sprintf("%-5d %-8s %s\n", 900, p.Username, "/usr/bin/redis-server 127.0.0.1:6379") +
		fmt.Sprintf("%-5d %-8s %s\n", 1020, "root", "/usr/sbin/rsyslogd -n")
}
