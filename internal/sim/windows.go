package sim

import (
	"fmt"

	gofakeit "github.com/brianvoe/gofakeit/v6"
)

func generateWindowsState(fake *gofakeit.Faker, s *SystemState) *WindowsState {
	return &WindowsState{
		ComputerName: s.Hostname,
		OSVersion:    "10.0.20348",
	}
}

func (s *SystemState) RunCommand(cmd string) (string, error) {
	return "", fmt.Errorf("'%s' is not recognized as an internal or external command", cmd)
}

func (s *SystemState) ReadRegistry(key string) (string, error) {
	return "", fmt.Errorf("ERROR: The system was unable to find the specified registry key or value.")
}

func (s *SystemState) QueryWMI(class string) (string, error) {
	return "", fmt.Errorf("Get-WmiObject : Not supported")
}

func (s *SystemState) GetProcessInfo() string {
	return "Image Name                     PID\nSystem                           4\n"
}
