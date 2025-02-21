package config

import "os"

type Config struct {
	MongoURI string
	Database string
}

func Load() Config {
	return Config{
		MongoURI: getEnv("MONGO_URI", "mongodb://localhost:27017"),
		Database: getEnv("DB_NAME", "finviz"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
