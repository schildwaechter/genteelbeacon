package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	slogfiber "github.com/samber/slog-fiber"
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
	app.Use(requestid.New())

	_, jsonLogging := os.LookupEnv("JSONLOGGING")
	var logger *slog.Logger
	if jsonLogging {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	//slog.SetDefault(logger)
	app.Use(slogfiber.New(logger))
	app.Use(recover.New())

	app.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/livez",
		ReadinessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		ReadinessEndpoint: "/readyz",
	}))

	// our names
	useName := getEnv("USENAME", "Genteel Beacon")
	nodeName, err := os.Hostname()
	if err != nil {
		nodeName = "unknown host"
	}

	// Define a route for the root path '/'
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Build time: " + buildEpoch + ", »" + useName + "« running on " + nodeName + " 🙋 " + slogfiber.GetRequestIDFromContext(c.Context()) + " 🏁")
	})

	// Start the server on the specified port
	runPort := getEnv("RUNPORT", "1333")
	log.Fatal(app.Listen(":" + runPort))
}
