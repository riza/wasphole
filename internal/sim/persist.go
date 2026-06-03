package sim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	gofakeit "github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
)

// persistSeed returns a seed for the persistence-only faker, isolated from the main faker.
// This ensures loadOrCreatePersisted never affects the main faker's call sequence.
func persistSeed(mainSeed string) int64 {
	return seedInt64(mainSeed + ":persist")
}

// PersistedState holds values that must survive process restarts.
type PersistedState struct {
	BootTime  time.Time `json:"boot_time"`
	MachineID string    `json:"machine_id"`
}

// loadOrCreatePersisted reads an existing sim_state.json or generates fresh values.
// On first run (or any parse error), a new BootTime and MachineID are generated and saved.
// Uses a separate faker (isolated from the main faker) so the main faker's call sequence
// is unaffected regardless of whether the file exists.
func loadOrCreatePersisted(path string, seed string) (*PersistedState, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var p PersistedState
		if jsonErr := json.Unmarshal(data, &p); jsonErr == nil && !p.BootTime.IsZero() && p.MachineID != "" {
			return &p, nil
		}
	}

	fake := gofakeit.New(persistSeed(seed))
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
