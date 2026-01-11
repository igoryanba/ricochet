package modes

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Loader handles parsing of mode configuration files
type Loader struct{}

// Load parses a YAML file into a Config struct
func (l *Loader) Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse modes config: %w", err)
	}

	return &cfg, nil
}
