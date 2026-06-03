package sim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
)

// PersistedState holds values that must survive process restarts.
type PersistedState struct {
	BootTime  time.Time `json:"boot_time"`
	MachineID string    `json:"machine_id"`
}

// loadOrCreatePersisted reads an existing sim_state.json or generates fresh values.
// On first run (or any parse error), a new BootTime and MachineID are generated and saved.
func loadOrCreatePersisted(path string, fake *gofakeit.Faker) (*PersistedState, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var p PersistedState
		if jsonErr := json.Unmarshal(data, &p); jsonErr == nil && !p.BootTime.IsZero() && p.MachineID != "" {
			return &p, nil
		}
	}

	p := &PersistedState{
		BootTime:  generateBootTime(fake),
		MachineID: uuid.New().String(),
	}

	out, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, out, 0600); err != nil {
		return nil, err
	}

	return p, nil
}

func generateBootTime(fake *gofakeit.Faker) time.Time {
	days := fake.IntRange(7, 60)
	hours := fake.IntRange(0, 23)
	return time.Now().
		AddDate(0, 0, -days).
		Add(-time.Duration(hours) * time.Hour).
		Truncate(time.Second)
}
