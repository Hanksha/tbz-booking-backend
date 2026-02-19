package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	bk "github.com/hanksha/tbz-booking-system-backend/booking"
	"github.com/hanksha/tbz-booking-system-backend/discord"
)

type BookingService interface {
	GetActiveBookings(ctx context.Context) ([]bk.Booking, error)
	FindBookingByID(ctx context.Context, id string) (bk.Booking, error)
	FindBookingsPerUsername(ctx context.Context, username string) ([]bk.Booking, error)
	CreateBooking(ctx context.Context, booking bk.Booking) (bk.Booking, error)
	ModifyBooking(ctx context.Context, updated bk.Booking, user discord.DiscordUser) error
	AcceptBooking(ctx context.Context, id string) error
	RefuseBooking(ctx context.Context, id, reason string) error
	CancelBooking(ctx context.Context, id string, user discord.DiscordUser) error
	GetBookingCountPerGame(ctx context.Context) ([]bk.GameBookingCount, error)
	GetBookingCountPerGameInPeriod(ctx context.Context, start, end time.Time) ([]bk.GameBookingCount, error)
	GetBookingCountPerWeekDay(ctx context.Context) ([]bk.WeekDayBookingCount, error)
}

type BookingHandler struct {
	service BookingService
}

func NewBookingHandler(service BookingService) *BookingHandler {
	return &BookingHandler{service: service}
}

func (h *BookingHandler) Register(rg *gin.RouterGroup) {
	adminOnly := AdminOnly()
	rg.GET("", h.ListActive)
	rg.GET("/booking/:id", h.GetByID)
	rg.POST("", h.Create)
	rg.PUT("/:id/accept", adminOnly, h.Accept)
	rg.PUT("/:id/refuse", adminOnly, h.Refuse)
	rg.PUT("/:id/cancel", h.Cancel)
	rg.PUT("/:id/modify", h.Modify)

	rg.GET("/stats/game", h.GetGameStats)
	rg.GET("/stats/game/period", h.GetGameStatsPerPeriod)
	rg.GET("/stats/day", h.GetGameStatsPerDay)

	rg.GET("/:username", h.GetByUsername)
}

func (h *BookingHandler) ListActive(c *gin.Context) {
	if bookings, err := h.service.GetActiveBookings(c.Request.Context()); err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve bookings",
		})
	} else {
		c.IndentedJSON(http.StatusOK, bookings)
	}
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	booking, err := h.service.FindBookingByID(c.Request.Context(), id)

	if err != nil {
		c.Error(err)
		if errors.Is(err, bk.ErrBookingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to fetch booking",
		})
		return
	}

	c.IndentedJSON(http.StatusOK, booking)
}

func (h *BookingHandler) GetByUsername(c *gin.Context) {
	username := c.Param("username")
	bookings, err := h.service.FindBookingsPerUsername(c.Request.Context(), username)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get bookings",
		})
		return
	}

	c.IndentedJSON(http.StatusOK, bookings)
}

func (h *BookingHandler) Create(c *gin.Context) {
	var booking bk.Booking

	if err := c.BindJSON(&booking); err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to parse JSON body",
		})
		return
	}

	inserted, err := h.service.CreateBooking(c.Request.Context(), booking)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to create booking",
		})
		return
	}

	c.JSON(http.StatusCreated, inserted)
}

func (h *BookingHandler) Accept(c *gin.Context) {
	id := c.Param("id")

	err := h.service.AcceptBooking(c.Request.Context(), id)

	if err != nil {
		c.Error(err)
		if errors.Is(err, bk.ErrBookingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})
		} else if errors.Is(err, bk.ErrInvalidBookingState) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid booking state",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to accept booking",
			})
		}

		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "booking accepted"})
}

func (h *BookingHandler) Refuse(c *gin.Context) {
	id := c.Param("id")
	reason := c.Query("reason")

	err := h.service.RefuseBooking(c.Request.Context(), id, reason)

	if err != nil {
		c.Error(err)
		if errors.Is(err, bk.ErrBookingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})
		} else if errors.Is(err, bk.ErrInvalidBookingState) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid booking state",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to refuse booking",
			})
		}

		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "booking refused"})
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	user := c.MustGet("user").(discord.DiscordUser)

	err := h.service.CancelBooking(c.Request.Context(), id, user)

	if err != nil {
		c.Error(err)
		if errors.Is(err, bk.ErrBookingNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "booking not found",
			})
		} else if errors.Is(err, bk.ErrInvalidBookingState) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid booking state",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to cancel booking",
			})
		}

		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "booking canceled"})
}

func (h *BookingHandler) Modify(c *gin.Context) {
	user := c.MustGet("user").(discord.DiscordUser)
	booking := bk.Booking{}
	id := c.Param("id")

	err := c.BindJSON(&booking)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse JSON body"})
		return
	}

	booking.ID = id

	err = h.service.ModifyBooking(c.Request.Context(), booking, user)

	if err != nil {
		c.Error(err)

		if errors.Is(err, bk.ErrNotAllowed) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not allowed to modify this booking"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to modify booking"})
		}

		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "booking modified"})
}

func (h *BookingHandler) GetGameStats(c *gin.Context) {
	stats, err := h.service.GetBookingCountPerGame(c.Request.Context())

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.IndentedJSON(http.StatusOK, stats)
}

func (h *BookingHandler) GetGameStatsPerPeriod(c *gin.Context) {
	startQuery := c.Query("startPeriod")
	endQuery := c.Query("endPeriod")

	startTime, err := time.Parse(time.DateOnly, startQuery)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse startPeriod"})
		return
	}

	endTime, err := time.Parse(time.DateOnly, endQuery)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse endPeriod"})
		return
	}

	stats, err := h.service.GetBookingCountPerGameInPeriod(c.Request.Context(), startTime, endTime)

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.IndentedJSON(http.StatusOK, stats)
}

func (h *BookingHandler) GetGameStatsPerDay(c *gin.Context) {
	stats, err := h.service.GetBookingCountPerWeekDay(c.Request.Context())

	if err != nil {
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get stats"})
		return
	}

	c.IndentedJSON(http.StatusOK, stats)
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.MustGet("user").(discord.DiscordUser)

		if !user.Admin {
			c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
			c.Abort()
			return
		}
	}
}
