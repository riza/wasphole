package sim

import (
	"fmt"
	"path/filepath"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/riza/wasphole/internal/config"
)

func New(cfg *config.Config) (*SystemState, error) {
	fake := gofakeit.New(seedInt64(cfg.Instance.Seed))

	statePath := filepath.Join(cfg.AI.CacheDir, "sim_state.json")
	persisted, err := loadOrCreatePersisted(statePath, cfg.Instance.Seed)
	if err != nil {
		return nil, err
	}

	s := &SystemState{
		Mode:      cfg.Instance.Mode,
		Hostname:  generateHostname(fake, cfg.Instance.Mode),
		BootTime:  persisted.BootTime,
		MachineID: persisted.MachineID,
	}

	switch cfg.Instance.Mode {
	case "linux":
		s.Linux = generateLinuxState(fake, s)
	case "windows":
		s.Windows = generateWindowsState(fake, s)
	}

	return s, nil
}

func generateHostname(fake *gofakeit.Faker, mode string) string {
	linuxPrefixes := []string{"prod-api", "web", "app", "db", "svc", "k8s-node"}
	winPrefixes := []string{"WIN-PROD", "SRV-APP", "DC01", "FILESERVER"}
	switch mode {
	case "linux":
		p := linuxPrefixes[fake.IntRange(0, len(linuxPrefixes)-1)]
		return fmt.Sprintf("%s-%02d", p, fake.IntRange(1, 9))
	default:
		p := winPrefixes[fake.IntRange(0, len(winPrefixes)-1)]
		return fmt.Sprintf("%s%02d", p, fake.IntRange(1, 9))
	}
}
