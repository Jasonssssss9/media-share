package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"

	"github.com/Jasonssssss9/media-share/backend/internal/imagegen"
	"github.com/Jasonssssss9/media-share/backend/internal/search"
)

type Config struct {
	ElasticsearchURL      string
	ElasticsearchPassword string
	ElasticsearchIndex    string
	Address               string
	DatabaseURL           string
	JWTSecret             string
	AppOrigin             string
	MediaDir              string
	OpenAIAPIKey          string
	OpenAIImageModel      string
}

func Load() (Config, error) {
	apiPort, err := port("API_PORT", "8080")
	if err != nil {
		return Config{}, err
	}
	dbPort, err := port("POSTGRES_PORT", "5432")
	if err != nil {
		return Config{}, err
	}
	webPort, err := port("WEB_PORT", "5173")
	if err != nil {
		return Config{}, err
	}
	elasticPort, err := port("ELASTICSEARCH_PORT", "9200")
	if err != nil {
		return Config{}, err
	}
	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		return Config{}, fmt.Errorf("POSTGRES_PASSWORD is required; set it in .env")
	}

	databaseURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(value("POSTGRES_USER", "mediashare"), password),
		Host:     net.JoinHostPort(value("POSTGRES_HOST", "127.0.0.1"), dbPort),
		Path:     "/" + value("POSTGRES_DB", "mediashare"),
		RawQuery: "sslmode=disable",
	}

	return Config{
		ElasticsearchURL:      "http://" + net.JoinHostPort("127.0.0.1", elasticPort),
		ElasticsearchPassword: os.Getenv("ELASTIC_PASSWORD"),
		ElasticsearchIndex:    value("ELASTICSEARCH_INDEX", search.DefaultIndex),
		Address:               net.JoinHostPort("127.0.0.1", apiPort),
		DatabaseURL:           databaseURL.String(),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		AppOrigin:             value("APP_ORIGIN", "http://localhost:"+webPort),
		MediaDir:              value("MEDIA_DIR", "../var/media"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIImageModel:      value("OPENAI_IMAGE_MODEL", imagegen.DefaultModel),
	}, nil
}

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func port(name, fallback string) (string, error) {
	v := value(name, fallback)
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("%s must be a port between 1 and 65535", name)
	}
	return v, nil
}
