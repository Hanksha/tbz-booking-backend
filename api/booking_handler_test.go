package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanksha/tbz-booking-system-backend/api"
	mock_api "github.com/hanksha/tbz-booking-system-backend/api/mocks"
	bk "github.com/hanksha/tbz-booking-system-backend/booking"
	"github.com/hanksha/tbz-booking-system-backend/discord"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func setUserInContext(user discord.DiscordUser) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	}
}

func setupRouter(t *testing.T) (*gin.Engine, *gomock.Controller, *mock_api.MockBookingService) {
	t.Helper()
	ctrl := gomock.NewController(t)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mockService := mock_api.NewMockBookingService(ctrl)
	handler := api.NewBookingHandler(mockService)
	handler.Register(router.Group("/api/v1/bookings"))

	return router, ctrl, mockService
}

func setupRouterWithUser(t *testing.T, user discord.DiscordUser) (*gin.Engine, *gomock.Controller, *mock_api.MockBookingService) {
	t.Helper()
	ctrl := gomock.NewController(t)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mockService := mock_api.NewMockBookingService(ctrl)
	handler := api.NewBookingHandler(mockService)
	rg := router.Group("/api/v1/bookings")
	rg.Use(setUserInContext(user))
	handler.Register(rg)

	return router, ctrl, mockService
}

func TestGetAllActiveBookings(t *testing.T) {
	router, ctrl, mockService := setupRouter(t)
	defer ctrl.Finish()

	bookings := []bk.Booking{
		{
			ID:              "1",
			Game:            "Star Wars Shatterpoint",
			UserID:          "user1ID",
			Username:        "user1",
			Points:          10,
			Description:     "test description1",
			Status:          "pending",
			ReminderEnabled: true,
			DateTime:        time.Now(),
			Players:         []string{"user1", "player2"},
		},
		{
			ID:              "2",
			Game:            "Star Wars Legion",
			UserID:          "user1ID",
			Username:        "user1",
			Points:          10,
			Description:     "test description1",
			Status:          "accepted",
			ReminderEnabled: true,
			DateTime:        time.Now(),
			Players:         []string{"user1", "player2"},
		},
	}

	bookingsJson, _ := json.MarshalIndent(bookings, "", "    ")
	mockService.EXPECT().GetActiveBookings(gomock.Any()).Return(bookings, nil).Times(1)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/bookings", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, string(bookingsJson), w.Body.String())
}

func TestGetAllActiveBookings_Error(t *testing.T) {
	router, ctrl, mockService := setupRouter(t)
	defer ctrl.Finish()

	mockService.EXPECT().GetActiveBookings(gomock.Any()).Return(nil, assert.AnError).Times(1)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/bookings", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)
	assert.JSONEq(t, `{"error":"failed to retrieve bookings"}`, w.Body.String())
}

func TestGetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", Game: "SW"}
		bJson, _ := json.MarshalIndent(b, "", "    ")
		mockService.EXPECT().FindBookingByID(gomock.Any(), "123").Return(b, nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/booking/123", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, string(bJson), w.Body.String())
	})

	t.Run("not found", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		mockService.EXPECT().FindBookingByID(gomock.Any(), "123").Return(bk.Booking{}, bk.ErrBookingNotFound).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/booking/123", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 404, w.Code)
		assert.JSONEq(t, `{"error":"booking not found"}`, w.Body.String())
	})

	t.Run("repo error", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		mockService.EXPECT().FindBookingByID(gomock.Any(), "123").Return(bk.Booking{}, assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/booking/123", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to fetch booking"}`, w.Body.String())
	})
}

func TestGetByUsername(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		bookings := []bk.Booking{{ID: "1"}, {ID: "2"}}
		bookingsJson, _ := json.MarshalIndent(bookings, "", "    ")
		mockService.EXPECT().FindBookingsPerUsername(gomock.Any(), "john").Return(bookings, nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/john", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, string(bookingsJson), w.Body.String())
	})

	t.Run("repo error", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		mockService.EXPECT().FindBookingsPerUsername(gomock.Any(), "john").Return(nil, assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/john", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to get bookings"}`, w.Body.String())
	})
}

func TestCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		toCreate := bk.Booking{Game: "SW", Username: "john"}
		inserted := bk.Booking{ID: "123", Game: "SW", Username: "john"}
		insertedJson, _ := json.Marshal(inserted)
		body, _ := json.Marshal(toCreate)

		mockService.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).Return(inserted, nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/bookings", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 201, w.Code)
		assert.JSONEq(t, string(insertedJson), w.Body.String())
	})

	t.Run("bad json", func(t *testing.T) {
		router, ctrl, _ := setupRouter(t)
		defer ctrl.Finish()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/bookings", bytes.NewBufferString("{"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"failed to parse JSON body"}`, w.Body.String())
	})

	t.Run("service error", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		body := []byte(`{"game":"SW"}`)
		mockService.EXPECT().CreateBooking(gomock.Any(), gomock.Any()).Return(bk.Booking{}, assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/bookings", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"failed to create booking"}`, w.Body.String())
	})
}

func TestAccept(t *testing.T) {
	admin := discord.DiscordUser{ID: "1", Username: "admin", Admin: true}
	nonAdmin := discord.DiscordUser{ID: "2", Username: "user", Admin: false}

	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().AcceptBooking(gomock.Any(), "123").Return(nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/accept", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, `{"message":"booking accepted"}`, w.Body.String())
	})

	t.Run("forbidden", func(t *testing.T) {
		router, ctrl, _ := setupRouterWithUser(t, nonAdmin)
		defer ctrl.Finish()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/accept", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 403, w.Code)
		assert.JSONEq(t, `{"error":"not allowed"}`, w.Body.String())
	})

	t.Run("not found", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().AcceptBooking(gomock.Any(), "123").Return(bk.ErrBookingNotFound).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/accept", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 404, w.Code)
		assert.JSONEq(t, `{"error":"booking not found"}`, w.Body.String())
	})

	t.Run("invalid state", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().AcceptBooking(gomock.Any(), "123").Return(bk.ErrInvalidBookingState).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/accept", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"invalid booking state"}`, w.Body.String())
	})

	t.Run("other error", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().AcceptBooking(gomock.Any(), "123").Return(assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/accept", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to accept booking"}`, w.Body.String())
	})
}

func TestRefuse(t *testing.T) {
	admin := discord.DiscordUser{ID: "1", Username: "admin", Admin: true}
	nonAdmin := discord.DiscordUser{ID: "2", Username: "user", Admin: false}

	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().RefuseBooking(gomock.Any(), "123", "no").Return(nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/refuse?reason=no", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, `{"message":"booking refused"}`, w.Body.String())
	})

	t.Run("forbidden", func(t *testing.T) {
		router, ctrl, _ := setupRouterWithUser(t, nonAdmin)
		defer ctrl.Finish()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/refuse?reason=no", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 403, w.Code)
		assert.JSONEq(t, `{"error":"not allowed"}`, w.Body.String())
	})

	t.Run("not found", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().RefuseBooking(gomock.Any(), "123", "no").Return(bk.ErrBookingNotFound).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/refuse?reason=no", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 404, w.Code)
		assert.JSONEq(t, `{"error":"booking not found"}`, w.Body.String())
	})

	t.Run("invalid state", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().RefuseBooking(gomock.Any(), "123", "no").Return(bk.ErrInvalidBookingState).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/refuse?reason=no", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"invalid booking state"}`, w.Body.String())
	})

	t.Run("other error", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, admin)
		defer ctrl.Finish()

		mockService.EXPECT().RefuseBooking(gomock.Any(), "123", "no").Return(assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/refuse?reason=no", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to refuse booking"}`, w.Body.String())
	})
}

func TestCancel(t *testing.T) {
	user := discord.DiscordUser{ID: "1", Username: "user", Admin: false}

	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		mockService.EXPECT().CancelBooking(gomock.Any(), "123", user).Return(nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/cancel", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, `{"message":"booking canceled"}`, w.Body.String())
	})

	t.Run("not found", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		mockService.EXPECT().CancelBooking(gomock.Any(), "123", user).Return(bk.ErrBookingNotFound).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/cancel", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 404, w.Code)
		assert.JSONEq(t, `{"error":"booking not found"}`, w.Body.String())
	})

	t.Run("invalid state", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		mockService.EXPECT().CancelBooking(gomock.Any(), "123", user).Return(bk.ErrInvalidBookingState).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/cancel", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"invalid booking state"}`, w.Body.String())
	})

	t.Run("other error", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		mockService.EXPECT().CancelBooking(gomock.Any(), "123", user).Return(assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/cancel", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to cancel booking"}`, w.Body.String())
	})
}

func TestModify(t *testing.T) {
	user := discord.DiscordUser{ID: "1", Username: "user", Admin: false}

	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		body := []byte(`{"game":"SW"}`)
		mockService.EXPECT().ModifyBooking(gomock.Any(), gomock.Any(), user).Return(nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/modify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, `{"message":"booking modified"}`, w.Body.String())
	})

	t.Run("not allowed", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		body := []byte(`{"game":"SW"}`)
		mockService.EXPECT().ModifyBooking(gomock.Any(), gomock.Any(), user).Return(bk.ErrNotAllowed).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/modify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 403, w.Code)
		assert.JSONEq(t, `{"error":"not allowed to modify this booking"}`, w.Body.String())
	})

	t.Run("service error", func(t *testing.T) {
		router, ctrl, mockService := setupRouterWithUser(t, user)
		defer ctrl.Finish()

		body := []byte(`{"game":"SW"}`)
		mockService.EXPECT().ModifyBooking(gomock.Any(), gomock.Any(), user).Return(assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/bookings/123/modify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to modify booking"}`, w.Body.String())
	})
}

func TestGetGameStats(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		stats := []bk.GameBookingCount{{Game: "SW", Count: 2}}
		statsJson, _ := json.MarshalIndent(stats, "", "    ")
		mockService.EXPECT().GetBookingCountPerGame(gomock.Any()).Return(stats, nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/game", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, string(statsJson), w.Body.String())
	})

	t.Run("error", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		mockService.EXPECT().GetBookingCountPerGame(gomock.Any()).Return(nil, assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/game", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to get stats"}`, w.Body.String())
	})
}

func TestGetGameStatsPerPeriod(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		startStr := "2026-02-01"
		endStr := "2026-02-07"
		startTime, _ := time.Parse(time.DateOnly, startStr)
		endTime, _ := time.Parse(time.DateOnly, endStr)

		stats := []bk.GameBookingCount{{Game: "SW", Count: 2}}
		statsJson, _ := json.MarshalIndent(stats, "", "    ")
		mockService.EXPECT().GetBookingCountPerGameInPeriod(gomock.Any(), startTime, endTime).Return(stats, nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/game/period?startPeriod="+startStr+"&endPeriod="+endStr, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, string(statsJson), w.Body.String())
	})

	t.Run("bad startPeriod", func(t *testing.T) {
		router, ctrl, _ := setupRouter(t)
		defer ctrl.Finish()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/game/period?startPeriod=bad&endPeriod=2026-02-07", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"failed to parse startPeriod"}`, w.Body.String())
	})

	t.Run("bad endPeriod", func(t *testing.T) {
		router, ctrl, _ := setupRouter(t)
		defer ctrl.Finish()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/game/period?startPeriod=2026-02-01&endPeriod=bad", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 400, w.Code)
		assert.JSONEq(t, `{"error":"failed to parse endPeriod"}`, w.Body.String())
	})

	t.Run("service error", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		startStr := "2026-02-01"
		endStr := "2026-02-07"
		startTime, _ := time.Parse(time.DateOnly, startStr)
		endTime, _ := time.Parse(time.DateOnly, endStr)

		mockService.EXPECT().GetBookingCountPerGameInPeriod(gomock.Any(), startTime, endTime).Return(nil, assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/game/period?startPeriod="+startStr+"&endPeriod="+endStr, nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to get stats"}`, w.Body.String())
	})
}

func TestGetGameStatsPerDay(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		stats := []bk.WeekDayBookingCount{{WeekDay: "Monday", Count: 2}}
		statsJson, _ := json.MarshalIndent(stats, "", "    ")
		mockService.EXPECT().GetBookingCountPerWeekDay(gomock.Any()).Return(stats, nil).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/day", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		assert.JSONEq(t, string(statsJson), w.Body.String())
	})

	t.Run("error", func(t *testing.T) {
		router, ctrl, mockService := setupRouter(t)
		defer ctrl.Finish()

		mockService.EXPECT().GetBookingCountPerWeekDay(gomock.Any()).Return(nil, assert.AnError).Times(1)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/bookings/stats/day", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"error":"failed to get stats"}`, w.Body.String())
	})
}
