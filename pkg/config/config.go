package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv" 

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

    // Дополнительное использование переменных окружения, если они заданы
    if port := os.Getenv("PORT"); port != "" {
        // Преобразуем строку в int
        if p, err := strconv.Atoi(port); err == nil {
            cfg.Port = p
        } else {
            return nil, fmt.Errorf("invalid PORT value: %v", err)
        }
    }

    if backends := os.Getenv("BACKENDS"); backends != "" {
        cfg.Backends = append(cfg.Backends, backends)
    }

    return &cfg, nil
}
