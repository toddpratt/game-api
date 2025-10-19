package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

type Config struct {
	Port           string
	JWTSecret      []byte
	AllowedOrigins string
}

func Load() *Config {
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
	}
	jwtSecretHex := os.Getenv("JWT_SECRET")
	if jwtSecretHex != "" {
		secret, err := hex.DecodeString(jwtSecretHex)
		if err != nil {
			log.Fatal("Invalid JWT_SECRET format. Must be hex-encoded.")
		}
		cfg.JWTSecret = secret
		log.Println("Loaded JWT secret from environment")
	} else {
		cfg.JWTSecret = generateSecret()
		log.Printf("WARNING: No JWT_SECRET set. Generated temporary secret: %s", hex.EncodeToString(cfg.JWTSecret))
		log.Println("Set JWT_SECRET environment variable for production!")
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func generateSecret() []byte {
	secret := make([]byte, 32)
	rand.Read(secret)
	return secret
}
