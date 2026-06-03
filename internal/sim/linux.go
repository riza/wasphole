package sim

import (
	"fmt"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

func generateLinuxState(fake *gofakeit.Faker, s *SystemState) *LinuxState {
	return &LinuxState{
		KernelVersion:  "5.15.0-91-generic",
		ListeningPorts: []int{22, 80, 443},
	}
}

func (s *SystemState) ReadFile(path string) (string, error) {
	return "", fmt.Errorf("open %s: no such file or directory", path)
}

func (s *SystemState) ExecShell(cmd string) (string, error) {
	return "", fmt.Errorf("bash: %s: command not found", cmd)
}

func (s *SystemState) ListProcesses() string {
	return "PID USER CMD\n1 root /sbin/init\n"
}
