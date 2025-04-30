package config

import (
    "io/ioutil"

    "gopkg.in/yaml.v2"
)

type Config struct {
    Port     int      `yaml:"port"`
    Backends []string `yaml:"backends"`
    RateLimit struct {
        Capacity   int `yaml:"capacity"`
        RefillRate int `yaml:"refill_rate"`
    } `yaml:"rate_limit"`
}

func Load(path string) (*Config, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}