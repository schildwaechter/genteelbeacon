package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
)

var buildEpoch string = "0"

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

func main() {
	app := fiber.New()

	// standard Kubernetes endpoints
	app.Get(healthcheck.DefaultLivenessEndpoint, healthcheck.NewHealthChecker())
	app.Get(healthcheck.DefaultReadinessEndpoint, healthcheck.NewHealthChecker())
	app.Get(healthcheck.DefaultStartupEndpoint, healthcheck.NewHealthChecker())
	app.Get("/healthz", healthcheck.NewHealthChecker())

	// our names
	useName := getEnv("USENAME", "Genteel Beacon")
	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	// Define a route for the root path '/'
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Build time: " + buildEpoch + ", »" + useName + "« running on " + nodeName + " 🙋")
	})

	// Start the server on the specified port
	runPort := getEnv("RUNPORT", "1333")
	log.Fatal(app.Listen(":" + runPort))
}
