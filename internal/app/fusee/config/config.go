package config

import "github.com/BurntSushi/toml"

type Config struct {
	Mounts map[string]Mount
}

type Mount struct {
	Path          string
	ReadCommand   string
	NameSeparator string
	Mode          uint32
	ThreadCount   uint
	Cache         bool
	CacheSeconds  uint64
	Directory     Directory
	File          File
}

type Directory struct {
	ReadCommand   string
	NameSeparator string
	Mode          uint32
	Cache         bool
	CacheSeconds  uint64
}

type File struct {
	ReadCommand  string
	Mode         uint32
	Cache        bool
	CacheSeconds uint64
}

func NewConfig(path string) (Config, error) {
	config := Config{}
	_, parseError := toml.DecodeFile(path, &config)

	return config, parseError
}
