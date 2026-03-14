package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Filters  Filters  `mapstructure:"filters"`
	Scoring  Scoring  `mapstructure:"scoring"`
	Database Database `mapstructure:"database"`
	LogLevel string   `mapstructure:"log_level"`
}

type Filters struct {
	MinRAMGB          int     `mapstructure:"min_ram_gb"`
	MinCPUCores       int     `mapstructure:"min_cpu_cores"`
	MinDrives         int     `mapstructure:"min_drives"`
	MinDriveSizeGB    int     `mapstructure:"min_drive_size_gb"`
	MaxPriceEUR       float64 `mapstructure:"max_price_eur"`
	DatacenterPrefix  string  `mapstructure:"datacenter_prefix"`
}

type Scoring struct {
	CPUWeight      float64 `mapstructure:"cpu_weight"`
	RAMWeight      float64 `mapstructure:"ram_weight"`
	StorageWeight  float64 `mapstructure:"storage_weight"`
	NVMeWeight     float64 `mapstructure:"nvme_weight"`
	CPUGenWeight   float64 `mapstructure:"cpu_gen_weight"`
	LocalityWeight float64 `mapstructure:"locality_weight"`
}

type Database struct {
	Path string `mapstructure:"path"`
}

func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetDefault("filters.min_ram_gb", 64)
	v.SetDefault("filters.min_cpu_cores", 8)
	v.SetDefault("filters.min_drives", 2)
	v.SetDefault("filters.min_drive_size_gb", 512)
	v.SetDefault("filters.max_price_eur", 90)
	v.SetDefault("filters.datacenter_prefix", "HEL1")

	v.SetDefault("scoring.cpu_weight", 0.30)
	v.SetDefault("scoring.ram_weight", 0.25)
	v.SetDefault("scoring.storage_weight", 0.20)
	v.SetDefault("scoring.nvme_weight", 0.10)
	v.SetDefault("scoring.cpu_gen_weight", 0.10)
	v.SetDefault("scoring.locality_weight", 0.05)

	v.SetDefault("database.path", "foundry-scout.db")
	v.SetDefault("log_level", "info")

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}
