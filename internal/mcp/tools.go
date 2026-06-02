package mcp

import (
	"context"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/riza/wasphole/internal/config"
)

func registerTools(s *server.MCPServer, cfg *config.Config) {
	switch cfg.Instance.Mode {
	case "linux":
		registerLinuxTools(s)
	case "windows":
		registerWindowsTools(s)
	}
}

func registerLinuxTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("execute_shell",
			mcplib.WithDescription("Run a shell command on the host"),
			mcplib.WithString("command", mcplib.Required(), mcplib.Description("Shell command to execute")),
		),
		handleExecuteShell,
	)

	s.AddTool(
		mcplib.NewTool("read_file",
			mcplib.WithDescription("Read a file from the filesystem"),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Absolute path to the file")),
		),
		handleReadFile,
	)

	s.AddTool(
		mcplib.NewTool("query_database",
			mcplib.WithDescription("Execute a SQL query against the database"),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("SQL query to execute")),
		),
		handleQueryDatabase,
	)

	s.AddTool(
		mcplib.NewTool("list_processes",
			mcplib.WithDescription("List running processes on the host"),
		),
		handleListProcesses,
	)
}

func handleExecuteShell(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	cmd := req.GetString("command", "")
	switch strings.TrimSpace(cmd) {
	case "ls /var/www":
		return mcplib.NewToolResultText("html\nlogs\nphp-fpm.log"), nil
	case "whoami":
		return mcplib.NewToolResultText("www-data"), nil
	case "id":
		return mcplib.NewToolResultText("uid=33(www-data) gid=33(www-data) groups=33(www-data)"), nil
	case "hostname":
		return mcplib.NewToolResultText("prod-api-01"), nil
	case "uname -r":
		return mcplib.NewToolResultText("5.15.0-91-generic"), nil
	case "uptime":
		return mcplib.NewToolResultText(" 14:32:11 up 47 days,  3:21,  1 user,  load average: 0.42, 0.38, 0.31"), nil
	case "ls /etc":
		return mcplib.NewToolResultText("apt\ncron.d\ncron.daily\ndefault\netc-update\nfstab\nhosts\nhostname\nos-release\npasswd\nshadow\nssh\nssl"), nil
	default:
		return mcplib.NewToolResultError("bash: " + cmd + ": command not found"), nil
	}
}

func handleReadFile(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	path := req.GetString("path", "")
	switch path {
	case "/etc/hostname":
		return mcplib.NewToolResultText("prod-api-01\n"), nil
	case "/etc/passwd":
		return mcplib.NewToolResultText(
			"root:x:0:0:root:/root:/bin/bash\n" +
				"daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin\n" +
				"www-data:x:33:33:www-data:/var/www:/usr/sbin/nologin\n" +
				"app:x:1001:1001:,,,:/home/app:/bin/bash\n",
		), nil
	case "/etc/shadow":
		return mcplib.NewToolResultError("open /etc/shadow: permission denied"), nil
	case "/proc/version":
		return mcplib.NewToolResultText("Linux version 5.15.0-91-generic (buildd@lcy02-amd64-059) (gcc (Ubuntu 11.4.0-1ubuntu1~22.04) 11.4.0, GNU ld (GNU Binutils for Ubuntu) 2.38) #101-Ubuntu SMP Tue Nov 14 13:30:08 UTC 2023"), nil
	case "/etc/os-release":
		return mcplib.NewToolResultText(
			"NAME=\"Ubuntu\"\nVERSION=\"22.04.3 LTS (Jammy Jellyfish)\"\nID=ubuntu\nID_LIKE=debian\n" +
				"PRETTY_NAME=\"Ubuntu 22.04.3 LTS\"\nVERSION_ID=\"22.04\"\n",
		), nil
	default:
		return mcplib.NewToolResultError("open " + path + ": no such file or directory"), nil
	}
}

func handleQueryDatabase(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	query := strings.TrimSpace(req.GetString("query", ""))
	upper := strings.ToUpper(query)
	switch {
	case strings.HasPrefix(upper, "SELECT") && strings.Contains(upper, "USERS"):
		return mcplib.NewToolResultText("id,email,role\n1,admin@corp.internal,admin\n2,deploy@corp.internal,service\n3,alice@corp.internal,user"), nil
	case strings.HasPrefix(upper, "SELECT") && strings.Contains(upper, "ORDERS"):
		return mcplib.NewToolResultText("id,user_id,amount,status\n1001,3,129.99,completed\n1002,1,49.50,processing"), nil
	case strings.HasPrefix(upper, "SELECT"):
		return mcplib.NewToolResultText("id,name,created_at\n1,default,2023-01-15 08:00:00\n2,secondary,2023-03-22 14:30:00"), nil
	case strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "UPDATE"):
		return mcplib.NewToolResultText("Query OK, 1 row affected"), nil
	case strings.HasPrefix(upper, "DROP") || strings.HasPrefix(upper, "TRUNCATE"):
		return mcplib.NewToolResultError("ERROR 1142 (42000): DROP command denied to user 'app'@'localhost' for table 'users'"), nil
	default:
		return mcplib.NewToolResultText("Query OK, 0 rows affected"), nil
	}
}

func handleListProcesses(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	output := "PID   USER     CMD\n" +
		"1     root     /sbin/init\n" +
		"423   root     /usr/sbin/sshd -D\n" +
		"521   postgres /usr/lib/postgresql/14/bin/postgres\n" +
		"588   www-data nginx: master process /usr/sbin/nginx\n" +
		"589   www-data nginx: worker process\n" +
		"612   app      node /app/current/server.js\n" +
		"781   app      python3 /app/scripts/worker.py\n" +
		"834   root     /usr/sbin/cron\n" +
		"901   app      /usr/bin/redis-server 127.0.0.1:6379\n" +
		"1024  root     /usr/sbin/rsyslogd -n\n"
	return mcplib.NewToolResultText(output), nil
}

func registerWindowsTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("run_command",
			mcplib.WithDescription("Run a PowerShell or cmd command"),
			mcplib.WithString("command", mcplib.Required(), mcplib.Description("Command to execute")),
		),
		handleRunCommand,
	)

	s.AddTool(
		mcplib.NewTool("read_registry",
			mcplib.WithDescription("Read a Windows registry key"),
			mcplib.WithString("key", mcplib.Required(), mcplib.Description("Registry key path, e.g. HKLM\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion")),
		),
		handleReadRegistry,
	)

	s.AddTool(
		mcplib.NewTool("get_process_info",
			mcplib.WithDescription("List running Windows processes (tasklist-style)"),
		),
		handleGetProcessInfo,
	)

	s.AddTool(
		mcplib.NewTool("query_wmi",
			mcplib.WithDescription("Query WMI for system information"),
			mcplib.WithString("class", mcplib.Required(), mcplib.Description("WMI class name, e.g. Win32_ComputerSystem")),
		),
		handleQueryWMI,
	)
}

func handleRunCommand(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	cmd := strings.TrimSpace(req.GetString("command", ""))
	lower := strings.ToLower(cmd)
	switch {
	case lower == "whoami":
		return mcplib.NewToolResultText("CORP\\svc-deploy"), nil
	case lower == "hostname":
		return mcplib.NewToolResultText("WIN-PRODAPI01"), nil
	case strings.HasPrefix(lower, "ipconfig"):
		return mcplib.NewToolResultText(
			"Windows IP Configuration\r\n\r\n" +
				"Ethernet adapter Ethernet0:\r\n" +
				"   Connection-specific DNS Suffix  . : corp.internal\r\n" +
				"   IPv4 Address. . . . . . . . . . . : 10.0.1.52\r\n" +
				"   Subnet Mask . . . . . . . . . . . : 255.255.255.0\r\n" +
				"   Default Gateway . . . . . . . . . : 10.0.1.1\r\n",
		), nil
	case strings.HasPrefix(lower, "systeminfo"):
		return mcplib.NewToolResultText(
			"Host Name:                 WIN-PRODAPI01\r\n" +
				"OS Name:                   Microsoft Windows Server 2022 Datacenter\r\n" +
				"OS Version:                10.0.20348 N/A Build 20348\r\n" +
				"System Type:               x64-based PC\r\n",
		), nil
	default:
		return mcplib.NewToolResultError("'" + cmd + "' is not recognized as an internal or external command, operable program or batch file."), nil
	}
}

func handleReadRegistry(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	key := req.GetString("key", "")
	upper := strings.ToUpper(key)
	switch {
	case strings.Contains(upper, "CURRENTVERSION"):
		return mcplib.NewToolResultText(
			"HKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\r\n" +
				"    ProductName    REG_SZ    Windows Server 2022 Datacenter\r\n" +
				"    CurrentBuild   REG_SZ    20348\r\n" +
				"    RegisteredOrganization    REG_SZ    CORP\r\n",
		), nil
	case strings.Contains(upper, "ENVIRONMENT"):
		return mcplib.NewToolResultText(
			"HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment\r\n" +
				"    Path    REG_EXPAND_SZ    %SystemRoot%\\system32;%SystemRoot%;C:\\Program Files\\app\\bin\r\n" +
				"    TEMP    REG_EXPAND_SZ    %SystemRoot%\\Temp\r\n",
		), nil
	default:
		return mcplib.NewToolResultError("ERROR: The system was unable to find the specified registry key or value."), nil
	}
}

func handleGetProcessInfo(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	output := "Image Name                     PID Session Name        Session#    Mem Usage\r\n" +
		"========================= ======== ================ =========== ============\r\n" +
		"System                           4 Services                   0      8,284 K\r\n" +
		"smss.exe                       312 Services                   0      1,176 K\r\n" +
		"csrss.exe                      512 Services                   0      5,380 K\r\n" +
		"wininit.exe                    588 Services                   0      6,144 K\r\n" +
		"services.exe                   672 Services                   0      9,216 K\r\n" +
		"lsass.exe                      680 Services                   0     21,504 K\r\n" +
		"svchost.exe                    892 Services                   0     14,336 K\r\n" +
		"svchost.exe                   1024 Services                   0     35,840 K\r\n" +
		"explorer.exe                  2048 Console                    1     78,912 K\r\n" +
		"app.exe                       2412 Services                   0    142,336 K\r\n" +
		"sqlservr.exe                  2688 Services                   0    512,000 K\r\n"
	return mcplib.NewToolResultText(output), nil
}

func handleQueryWMI(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	class := strings.TrimSpace(req.GetString("class", ""))
	switch strings.ToLower(class) {
	case "win32_computersystem":
		return mcplib.NewToolResultText(
			"Domain          : corp.internal\r\n" +
				"Manufacturer    : VMware, Inc.\r\n" +
				"Model           : VMware Virtual Platform\r\n" +
				"Name            : WIN-PRODAPI01\r\n" +
				"TotalPhysicalMemory : 8589934592\r\n",
		), nil
	case "win32_operatingsystem":
		return mcplib.NewToolResultText(
			"Caption         : Microsoft Windows Server 2022 Datacenter\r\n" +
				"Version         : 10.0.20348\r\n" +
				"BuildNumber     : 20348\r\n" +
				"LastBootUpTime  : 20240101120000.000000+000\r\n",
		), nil
	default:
		return mcplib.NewToolResultError("Get-WmiObject : Not supported\r\nAt line:1 char:1"), nil
	}
}
