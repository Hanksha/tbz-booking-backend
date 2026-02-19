package main

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"os"

	"github.com/hanksha/tbz-booking-system-backend/api"
	bk "github.com/hanksha/tbz-booking-system-backend/booking"
	"github.com/hanksha/tbz-booking-system-backend/discord"
	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

//go:embed database/setup.sql
var setupSQL string

func main() {
	logger := slog.Default().With("component", "main")

	err := godotenv.Load()

	if err != nil {
		logger.Error("Error loading .env file", "err", err)
	}

	// postgres://postgres:password@localhost:5432/petprojects
	logger.Info("connecting to PostgreSQL database")
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))

	if err != nil {
		logger.Error("Unable to connect to database", "err", err)
		os.Exit(1)
	}

	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), setupSQL)
	if err != nil {
		logger.Error("failed to initialize tables", "err", err)
		os.Exit(1)
	} else {
		logger.Info("initialized database tables")
	}

	discordClient := discord.NewClient(
		os.Getenv("DISCORD_BOT_TOKEN"),
		os.Getenv("DISCORD_CLIENT_ID"),
		os.Getenv("DISCORD_CLIENT_SECRET"),
		os.Getenv("DISCORD_REDIRECT_URI"),
		os.Getenv("DISCORD_SERVER_ID"),
	)

	bookingRepo := bk.NewRepository(conn)
	bookingService := bk.NewService(bookingRepo, discordClient, os.Getenv("DISCORD_CHANNEL_ID"))

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	adminRoleID := os.Getenv("DISCORD_ADMIN_ROLE_ID")

	// DISCORD API

	discordRouter := r.Group("/api/discord")
	discordHandler := api.NewDiscordHandler(discordClient, adminRoleID)

	discordHandler.Register(discordRouter)

	// BOOKING API

	bookingRouter := r.Group("/api/v1/bookings")
	bookingRouter.Use(api.DiscordAuth(discordClient, adminRoleID))
	bookingHandler := api.NewBookingHandler(bookingService)

	bookingHandler.Register(bookingRouter)

	r.Run(":9090")
}
