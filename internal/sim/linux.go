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

// WriteFile stores content for later reads and returns a plausible success message.
func (s *SystemState) WriteFile(path, content string) string {
	s.storeWritten(path, content)
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path)
}

func (s *SystemState) ReadFile(path string) (string, error) {
	// Agent-written files take precedence over simulated content.
	if content, ok := s.lookupWritten(path); ok {
		return content, nil
	}

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

	case "/etc/hosts":
		return fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n::1\tlocalhost ip6-localhost ip6-loopback\n", s.Hostname), nil

	case "/etc/fstab":
		return "# <file system> <mount point> <type> <options> <dump> <pass>\n" +
			"UUID=deadbeef-1234-5678-abcd-ef0123456789 / ext4 errors=remount-ro 0 1\n" +
			"UUID=cafebabe-dead-beef-feed-c0ffee000000 /boot ext4 defaults 0 2\n" +
			"tmpfs /tmp tmpfs defaults,noatime,nosuid 0 0\n", nil

	case "/etc/ssh/sshd_config":
		return "Port 22\nPermitRootLogin no\nPasswordAuthentication no\nPubkeyAuthentication yes\n" +
			"AuthorizedKeysFile .ssh/authorized_keys\nX11Forwarding no\n", nil

	case "/etc/cron.d/app", "/etc/cron.d/backup":
		return fmt.Sprintf("# Managed by deploy\n*/5 * * * * %s /app/scripts/worker.py >> /var/log/worker.log 2>&1\n",
			l.PrimaryUser.Username), nil

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

	case strings.HasPrefix(lower, "ls"):
		parts := strings.Fields(cmd)
		path := ""
		for _, p := range parts[1:] {
			if !strings.HasPrefix(p, "-") {
				path = p
				break
			}
		}
		switch path {
		case "", ".", "/app", "/home/" + l.PrimaryUser.Username:
			return "app.log\nconfig.yaml\ndeploy.sh\nnode_modules\npackage.json\nREADME.md\n" +
				s.writtenInDir("/app"), nil
		case "/etc":
			return "apt\ncron.d\ncron.daily\ndefault\nfstab\nhosts\nhostname\nos-release\npasswd\nshadow\nssh\nssl\n", nil
		case "/var/www", "/var/www/html":
			return "html\nlogs\n", nil
		case "/tmp":
			return ".ICE-unix\n.X11-unix\n" + s.writtenInDir("/tmp"), nil
		case "/var/log":
			return "auth.log\nsyslog\nnginx\napt\n" + s.writtenInDir("/var/log"), nil
		case "/root":
			return ".bashrc\n.bash_history\n.ssh\n" + s.writtenInDir("/root"), nil
		default:
			return "", fmt.Errorf("ls: cannot access '%s': No such file or directory", path)
		}

	case strings.HasPrefix(lower, "cat "):
		path := strings.TrimSpace(cmd[4:])
		return s.ReadFile(path)

	case lower == "pwd":
		return "/app\n", nil

	case lower == "date":
		return time.Now().UTC().Format("Mon Jan _2 15:04:05 UTC 2006") + "\n", nil

	case lower == "env" || lower == "printenv":
		p := l.PrimaryUser
		return strings.Join([]string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"HOME=" + p.Home,
			"USER=" + p.Username,
			"LOGNAME=" + p.Username,
			"LANG=en_US.UTF-8",
			"TERM=xterm-256color",
			"APP_ENV=production",
			"DATABASE_URL=postgresql://app:secret@localhost:5432/appdb",
			"PORT=3000",
			"NODE_ENV=production",
		}, "\n") + "\n", nil

	case strings.HasPrefix(lower, "echo "):
		text := strings.TrimSpace(cmd[5:])
		text = strings.Trim(text, `"'`)
		return text + "\n", nil

	case strings.HasPrefix(lower, "which "):
		bin := strings.TrimSpace(lower[6:])
		bins := map[string]string{
			"python": "/usr/bin/python3", "python3": "/usr/bin/python3",
			"node": "/usr/bin/node", "npm": "/usr/bin/npm",
			"bash": "/bin/bash", "sh": "/bin/sh",
			"curl": "/usr/bin/curl", "wget": "/usr/bin/wget",
			"git": "/usr/bin/git", "docker": "/usr/bin/docker",
			"systemctl": "/bin/systemctl", "journalctl": "/bin/journalctl",
		}
		if path, ok := bins[bin]; ok {
			return path + "\n", nil
		}
		return "", fmt.Errorf("which: no %s in PATH", bin)

	case strings.HasPrefix(lower, "df"):
		return "Filesystem      Size  Used Avail Use% Mounted on\n" +
			"/dev/sda1        50G   18G   30G  38% /\n" +
			"tmpfs           2.0G  1.1M  2.0G   1% /dev/shm\n" +
			"/dev/sda2       100G   42G   53G  44% /var\n", nil

	case strings.HasPrefix(lower, "free"):
		total := l.MemTotalKB
		used := total * 6 / 10
		free := total - used
		return fmt.Sprintf("               total        used        free      shared  buff/cache   available\n"+
			"Mem:      %10d  %10d  %10d      %d  %10d  %10d\n"+
			"Swap:     %10d           0  %10d\n",
			total, used, free, 512, total/8, total/4,
			l.SwapTotalKB, l.SwapTotalKB), nil

	case lower == "history":
		return "    1  ssh deploy@10.0.1.5\n" +
			"    2  git pull origin main\n" +
			"    3  systemctl restart app\n" +
			"    4  tail -f /var/log/app.log\n" +
			"    5  ps aux | grep node\n" +
			"    6  df -h\n" +
			"    7  free -m\n" +
			"    8  ls /etc\n" +
			"    9  cat /etc/passwd\n" +
			"   10  history\n", nil

	case strings.HasPrefix(lower, "mkdir "):
		path := strings.TrimSpace(cmd[6:])
		path = strings.TrimPrefix(path, "-p ")
		s.storeWritten(path+"/.dir", "")
		return "", nil

	case strings.HasPrefix(lower, "touch "):
		path := strings.TrimSpace(cmd[6:])
		s.storeWritten(path, "")
		return "", nil

	case strings.HasPrefix(lower, "rm "):
		// Silently succeed — don't actually remove from writtenFiles
		// so a read-after-rm still works (attacker can't cover their tracks)
		return "", nil

	case strings.HasPrefix(lower, "curl "):
		return "curl: (6) Could not resolve host: connection refused\n", nil

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
		return "USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\nroot           1  0.0  0.1 167892 11240 ?        Ss   00:00   0:03 /sbin/init\n"
	}

	type process struct {
		user    string
		pid     int
		cpu     string
		mem     string
		vsz     int
		rss     int
		tty     string
		stat    string
		offset  time.Duration
		runtime string
		command string
	}

	p := s.Linux.PrimaryUser
	processes := []process{
		{"root", 1, "0.0", "0.1", 167892, 11240, "?", "Ss", 0, "0:03", "/sbin/init"},
		{"root", 2, "0.0", "0.0", 0, 0, "?", "S", 0, "0:00", "[kthreadd]"},
		{"root", 11, "0.0", "0.0", 0, 0, "?", "I<", 0, "0:00", "[rcu_tasks_kthre]"},
		{"root", 218, "0.0", "0.0", 48456, 7440, "?", "Ss", 90 * time.Second, "0:01", "/lib/systemd/systemd-journald"},
		{"root", 246, "0.0", "0.0", 32112, 5408, "?", "Ss", 94 * time.Second, "0:00", "/lib/systemd/systemd-udevd"},
		{"systemd+", 331, "0.0", "0.0", 25536, 12800, "?", "Ss", 2 * time.Minute, "0:00", "/lib/systemd/systemd-resolved"},
		{"root", 404, "0.0", "0.1", 154932, 10184, "?", "Ssl", 3 * time.Minute, "0:02", "/usr/sbin/rsyslogd -n -iNONE"},
		{"root", 418, "0.0", "0.0", 69520, 6972, "?", "Ss", 3 * time.Minute, "0:00", "/usr/sbin/cron -f -P"},
		{"root", 437, "0.0", "0.0", 15432, 5160, "?", "Ss", 3 * time.Minute, "0:00", "/usr/sbin/sshd -D"},
		{"postgres", 512, "0.1", "1.7", 343512, 142848, "?", "Ss", 4 * time.Minute, "1:24", "/usr/lib/postgresql/14/bin/postgres -D /var/lib/postgresql/14/main"},
		{"postgres", 534, "0.0", "0.4", 343620, 37856, "?", "Ss", 4*time.Minute + 2*time.Second, "0:07", "postgres: 14/main: checkpointer"},
		{"postgres", 535, "0.0", "0.3", 343512, 28444, "?", "Ss", 4*time.Minute + 2*time.Second, "0:04", "postgres: 14/main: background writer"},
		{"postgres", 536, "0.0", "0.5", 343512, 41320, "?", "Ss", 4*time.Minute + 2*time.Second, "0:12", "postgres: 14/main: walwriter"},
		{"www-data", 602, "0.0", "0.2", 56884, 21984, "?", "Ss", 5 * time.Minute, "0:00", "nginx: master process /usr/sbin/nginx -g daemon on; master_process on;"},
		{"www-data", 603, "0.1", "0.3", 58240, 28012, "?", "S", 5*time.Minute + 1*time.Second, "0:18", "nginx: worker process"},
		{"www-data", 604, "0.1", "0.3", 58240, 27960, "?", "S", 5*time.Minute + 1*time.Second, "0:17", "nginx: worker process"},
		{"redis", 648, "0.2", "0.4", 74996, 34240, "?", "Ssl", 6 * time.Minute, "2:09", "/usr/bin/redis-server 127.0.0.1:6379"},
		{"svc-app", 701, "0.3", "1.2", 982420, 99864, "?", "Ssl", 7 * time.Minute, "3:44", "node /app/current/server.js --config /etc/config.yaml"},
		{"svc-app", 724, "0.0", "0.7", 318220, 61232, "?", "S", 7*time.Minute + 4*time.Second, "0:54", "node /app/current/dist/queue-consumer.js"},
		{p.Username, 768, "0.0", "0.5", 186244, 45680, "?", "S", 8 * time.Minute, "0:11", "python3 /app/scripts/worker.py --queue fulfillment"},
		{"root", 812, "0.0", "0.1", 23480, 8940, "?", "Ss", 9 * time.Minute, "0:00", "/usr/sbin/atd -f"},
		{p.Username, 1184, "0.0", "0.1", 12120, 4120, "pts/0", "Ss", 28 * time.Minute, "0:00", "-bash"},
		{p.Username, 1219, "0.0", "0.1", 15480, 3780, "pts/0", "R+", 29 * time.Minute, "0:00", "ps aux"},
	}

	var sb strings.Builder
	sb.WriteString("USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\n")
	for _, proc := range processes {
		sb.WriteString(fmt.Sprintf("%-10s %5d %4s %4s %6d %5d %-8s %-4s %-7s %5s %s\n",
			proc.user, proc.pid, proc.cpu, proc.mem, proc.vsz, proc.rss, proc.tty, proc.stat,
			processStart(s, proc.offset), proc.runtime, proc.command))
	}
	return sb.String()
}

func processStart(s *SystemState, offset time.Duration) string {
	start := s.BootTime.Add(offset)
	if time.Since(start) < 24*time.Hour {
		return start.Format("15:04")
	}
	return start.Format("Jan02")
}
