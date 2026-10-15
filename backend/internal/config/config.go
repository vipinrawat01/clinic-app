package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config is deliberately small — this app has no multi-tenant concept
// (see migration 000001's doc), no per-company RLS, no agent-service
// dependency. Just a database, a port, a JWT secret for staff logins, a
// shared service key the main chatbot platform's Action nodes
// authenticate with (see middleware.RequireServiceAuth), and the
// reverse connection — a real sk_live_ API key from the main platform's
// own Developers page, used by internal/transport to send a WhatsApp
// message to a transport coordinator through the clinic's connected
// WhatsApp number (see the main platform's MessagingHandler.SendWhatsApp).
type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	ServiceKey     string
	AllowedOrigins []string

	MainPlatformBaseURL        string
	MainPlatformAPIKey         string
	MainPlatformAgentID        string
	TransportCoordinatorNumber string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Port:           getOr("PORT", "8200"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		ServiceKey:     os.Getenv("SERVICE_KEY"),
		AllowedOrigins: splitCSV(getOr("ALLOWED_ORIGINS", "http://localhost:8300")),

		MainPlatformBaseURL:        getOr("MAIN_PLATFORM_BASE_URL", "http://localhost:8090"),
		MainPlatformAPIKey:         os.Getenv("MAIN_PLATFORM_API_KEY"),
		MainPlatformAgentID:        os.Getenv("MAIN_PLATFORM_AGENT_ID"),
		TransportCoordinatorNumber: os.Getenv("TRANSPORT_COORDINATOR_NUMBER"),
	}
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
