package booking_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	bk "github.com/hanksha/tbz-booking-system-backend/booking"
	bk_mocks "github.com/hanksha/tbz-booking-system-backend/booking/mocks"
	"github.com/hanksha/tbz-booking-system-backend/discord"
	dc_mocks "github.com/hanksha/tbz-booking-system-backend/discord/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var activeBookings = []bk.Booking{{
	ID:              "1",
	Game:            "test1",
	UserID:          "user1ID",
	Username:        "user1",
	Points:          10,
	Description:     "test description1",
	Status:          "pending",
	ReminderEnabled: true,
	DateTime:        time.Now(),
	Players:         []string{"user1", "player2"},
}}

type testDeps struct {
	repo    *bk_mocks.MockBookingRepository
	client  *dc_mocks.MockDiscordClient
	service *bk.Service
	ctx     context.Context
}

func newTestDeps(t *testing.T) (*gomock.Controller, testDeps) {
	t.Helper()
	ctrl := gomock.NewController(t)

	repo := bk_mocks.NewMockBookingRepository(ctrl)
	client := dc_mocks.NewMockDiscordClient(ctrl)
	svc := bk.NewService(repo, client, "test-channel-d")

	return ctrl, testDeps{
		repo: repo, client: client, service: svc, ctx: context.Background(),
	}
}

func TestGetAllActiveBookings(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetActiveBookings(testDeps.ctx).Return(activeBookings, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		bookings, err := testDeps.service.GetActiveBookings(testDeps.ctx)

		require.Nil(t, err)
		require.NotEqual(t, 0, len(bookings))

		if !reflect.DeepEqual(bookings, activeBookings) {
			t.Fatalf("expected bookings %#v, got %#v", activeBookings, bookings)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetActiveBookings(testDeps.ctx).Return(nil, errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		bookings, err := testDeps.service.GetActiveBookings(testDeps.ctx)

		require.Error(t, err)
		require.Equal(t, 0, len(bookings))
	})
}

func TestGetBookingById(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(activeBookings[0], nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		booking, err := testDeps.service.FindBookingByID(testDeps.ctx, "123")

		require.Nil(t, err)

		if !reflect.DeepEqual(booking, activeBookings[0]) {
			t.Fatalf("expected bookings %#v, got %#v", activeBookings[0], booking)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(bk.Booking{}, bk.ErrBookingNotFound).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.FindBookingByID(testDeps.ctx, "123")

		require.ErrorIs(t, err, bk.ErrBookingNotFound)
		require.NotNil(t, got)
	})
}

func TestGetBookingsPerUsername(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingsPerUsername(testDeps.ctx, "john.doe").Return(activeBookings, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		bookings, err := testDeps.service.FindBookingsPerUsername(testDeps.ctx, "john.doe")

		require.Nil(t, err)
		require.NotEqual(t, 0, len(bookings))

		if !reflect.DeepEqual(bookings, activeBookings) {
			t.Fatalf("expected bookings %#v, got %#v", activeBookings, bookings)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingsPerUsername(testDeps.ctx, "john.doe").Return(nil, errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		bookings, err := testDeps.service.FindBookingsPerUsername(testDeps.ctx, "john.doe")

		require.Error(t, err)
		require.Equal(t, 0, len(bookings))
	})
}

func TestCreateBooking(t *testing.T) {
	dateTime := time.Now()

	toInsert := bk.Booking{
		Game:            "test1",
		UserID:          "user1ID",
		Username:        "user1",
		Points:          10,
		Description:     "test description1",
		ReminderEnabled: true,
		DateTime:        dateTime,
		Players:         []string{"user1", "player2"},
	}
	inserted := bk.Booking{
		ID:              "1",
		Game:            "test1",
		UserID:          "user1ID",
		Username:        "user1",
		Points:          10,
		Description:     "test description1",
		Status:          "pending",
		ReminderEnabled: true,
		DateTime:        dateTime,
		Players:         []string{"user1", "player2"},
	}
	member1 := discord.Member{
		User: discord.User{
			ID:       "12345",
			Username: "user1",
		},
	}

	member2 := discord.Member{
		User: discord.User{
			ID:       "abcdef",
			Username: "player2",
		},
	}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().InsertBooking(testDeps.ctx, toInsert).Return(inserted, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(1)

		booking, err := testDeps.service.CreateBooking(testDeps.ctx, toInsert)

		require.Nil(t, err)

		if !reflect.DeepEqual(booking, inserted) {
			t.Fatalf("expected bookings %#v, got %#v", activeBookings[0], booking)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().InsertBooking(testDeps.ctx, toInsert).Return(bk.Booking{}, errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		booking, err := testDeps.service.CreateBooking(testDeps.ctx, toInsert)

		require.Error(t, err)
		require.NotNil(t, booking)
	})
}

func TestModifyBooking(t *testing.T) {
	user := discord.DiscordUser{
		ID:       "user1ID",
		Username: "user1",
		Admin:    false,
	}

	member1 := discord.Member{
		User: discord.User{
			ID:       "user1ID",
			Username: "user1",
		},
	}

	member2 := discord.Member{
		User: discord.User{
			ID:       "abcdef",
			Username: "player2",
		},
	}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		dateTime := time.Now()

		booking := bk.Booking{
			ID:              "123",
			Game:            "test1",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		updated := bk.Booking{
			ID:              "123",
			Game:            "modified",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(booking, nil).Times(1)
		testDeps.repo.EXPECT().UpdateBooking(testDeps.ctx, updated).Return(nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(1)

		err := testDeps.service.ModifyBooking(testDeps.ctx, updated, user)

		require.Nil(t, err)

	})

	t.Run("invalid state", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		dateTime := time.Now()

		booking := bk.Booking{
			ID:              "123",
			Game:            "test1",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "approved",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		updated := bk.Booking{
			ID:              "123",
			Game:            "modified",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "approved",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(booking, nil).Times(1)
		testDeps.repo.EXPECT().UpdateBooking(testDeps.ctx, updated).Return(nil).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(0)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(0)

		err := testDeps.service.ModifyBooking(testDeps.ctx, updated, user)

		require.ErrorIs(t, err, bk.ErrInvalidBookingState)
	})

	t.Run("not allowed", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		user := discord.DiscordUser{
			ID:       "user2ID",
			Username: "user2",
			Admin:    false,
		}

		dateTime := time.Now()

		booking := bk.Booking{
			ID:              "123",
			Game:            "test1",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		updated := bk.Booking{
			ID:              "123",
			Game:            "modified",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(booking, nil).Times(1)
		testDeps.repo.EXPECT().UpdateBooking(testDeps.ctx, updated).Return(nil).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(0)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(0)

		err := testDeps.service.ModifyBooking(testDeps.ctx, updated, user)

		require.ErrorIs(t, err, bk.ErrNotAllowed)
	})

	t.Run("repo error GetBookingById", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		dateTime := time.Now()

		updated := bk.Booking{
			ID:              "123",
			Game:            "modified",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(bk.Booking{}, errors.New("repo error")).Times(1)
		testDeps.repo.EXPECT().UpdateBooking(testDeps.ctx, updated).Return(nil).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(0)

		err := testDeps.service.ModifyBooking(testDeps.ctx, updated, user)

		require.Error(t, err)
	})

	t.Run("repo error UpdateBooking", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		dateTime := time.Now()

		booking := bk.Booking{
			ID:              "123",
			Game:            "test1",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		updated := bk.Booking{
			ID:              "123",
			Game:            "modified",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        dateTime,
			Players:         []string{"user1", "player2"},
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(booking, nil).Times(1)
		testDeps.repo.EXPECT().UpdateBooking(testDeps.ctx, updated).Return(errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(0)

		err := testDeps.service.ModifyBooking(testDeps.ctx, updated, user)

		require.Error(t, err)
	})

}

func TestAcceptBooking(t *testing.T) {
	member1 := discord.Member{
		User: discord.User{
			ID:       "12345",
			Username: "user1",
		},
	}

	member2 := discord.Member{
		User: discord.User{
			ID:       "abcdef",
			Username: "player2",
		},
	}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{
			ID:              "123",
			Game:            "test1",
			UserID:          "user1ID",
			Username:        "user1",
			Status:          "pending",
			Points:          10,
			Description:     "test description1",
			ReminderEnabled: true,
			DateTime:        time.Now(),
			Players:         []string{"user1", "player2"},
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, "123", "accepted").Return(nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		err := testDeps.service.AcceptBooking(testDeps.ctx, "123")
		require.Nil(t, err)
	})

	t.Run("repo error GetBookingById", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(bk.Booking{}, errors.New("repo error")).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.AcceptBooking(testDeps.ctx, "123")
		require.Error(t, err)
	})

	t.Run("invalid state", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", Status: "accepted"}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.AcceptBooking(testDeps.ctx, "123")
		require.ErrorIs(t, err, bk.ErrInvalidBookingState)
	})

	t.Run("repo error SetBookingStatus", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", Status: "pending", Players: []string{"user1", "player2"}}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, "123", "accepted").Return(errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.AcceptBooking(testDeps.ctx, "123")
		require.Error(t, err)
	})
}

func TestRefuseBooking(t *testing.T) {
	member1 := discord.Member{
		User: discord.User{
			ID:       "12345",
			Username: "user1",
		},
	}

	member2 := discord.Member{
		User: discord.User{
			ID:       "abcdef",
			Username: "player2",
		},
	}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{
			ID:       "123",
			Status:   "pending",
			Players:  []string{"user1", "player2"},
			DateTime: time.Now(),
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, "123", "refused").Return(nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		err := testDeps.service.RefuseBooking(testDeps.ctx, "123", "because")
		require.Nil(t, err)
	})

	t.Run("invalid state", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", Status: "canceled"}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.RefuseBooking(testDeps.ctx, "123", "because")
		require.ErrorIs(t, err, bk.ErrInvalidBookingState)
	})

	t.Run("repo error GetBookingById", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(bk.Booking{}, errors.New("repo error")).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.RefuseBooking(testDeps.ctx, "123", "because")
		require.Error(t, err)
	})

	t.Run("repo error SetBookingStatus", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", Status: "pending"}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, "123", "refused").Return(errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.RefuseBooking(testDeps.ctx, "123", "because")
		require.Error(t, err)
	})
}

func TestCancelBooking(t *testing.T) {
	member1 := discord.Member{
		User: discord.User{
			ID:       "12345",
			Username: "user1",
		},
	}

	member2 := discord.Member{
		User: discord.User{
			ID:       "abcdef",
			Username: "player2",
		},
	}

	user := discord.DiscordUser{ID: "user1ID", Username: "user1", Admin: false}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{
			ID:       "123",
			UserID:   "user1ID",
			Username: "user1",
			Status:   "pending",
			Players:  []string{"user1", "player2"},
			DateTime: time.Now(),
		}

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, "123", "canceled").Return(nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member1.User.Username, 1).Return([]discord.Member{member1}, nil).Times(1)
		testDeps.client.EXPECT().SearchMembers(testDeps.ctx, member2.User.Username, 1).Return([]discord.Member{member2}, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		err := testDeps.service.CancelBooking(testDeps.ctx, "123", user)
		require.Nil(t, err)
	})

	t.Run("invalid state", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", Status: "refused"}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.CancelBooking(testDeps.ctx, "123", user)
		require.ErrorIs(t, err, bk.ErrInvalidBookingState)
	})

	t.Run("not allowed", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		notAllowedUser := discord.DiscordUser{ID: "someone", Username: "someone", Admin: false}
		b := bk.Booking{ID: "123", UserID: "user1ID", Username: "user1", Status: "pending", Players: []string{"user1", "player2"}}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.CancelBooking(testDeps.ctx, "123", notAllowedUser)
		require.ErrorIs(t, err, bk.ErrNotAllowed)
	})

	t.Run("repo error GetBookingById", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(bk.Booking{}, errors.New("repo error")).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, gomock.Any(), gomock.Any()).Times(0)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.CancelBooking(testDeps.ctx, "123", user)
		require.Error(t, err)
	})

	t.Run("repo error SetBookingStatus", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		b := bk.Booking{ID: "123", UserID: "user1ID", Username: "user1", Status: "pending", Players: []string{"user1", "player2"}}
		testDeps.repo.EXPECT().GetBookingByID(testDeps.ctx, "123").Return(b, nil).Times(1)
		testDeps.repo.EXPECT().SetBookingStatus(testDeps.ctx, "123", "canceled").Return(errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := testDeps.service.CancelBooking(testDeps.ctx, "123", user)
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to cancel booking")
	})
}

func TestGetBookingCountPerGame(t *testing.T) {
	stats := []bk.GameBookingCount{{Game: "test1", Count: 2}}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingCountPerGame(testDeps.ctx).Return(stats, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.GetBookingCountPerGame(testDeps.ctx)
		require.Nil(t, err)
		require.True(t, reflect.DeepEqual(got, stats))
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingCountPerGame(testDeps.ctx).Return(nil, errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.GetBookingCountPerGame(testDeps.ctx)
		require.Error(t, err)
		require.Equal(t, 0, len(got))
	})
}

func TestGetBookingCountPerWeekDay(t *testing.T) {
	stats := []bk.WeekDayBookingCount{{WeekDay: "Monday", Count: 2}}

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingCountPerWeekDay(testDeps.ctx).Return(stats, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.GetBookingCountPerWeekDay(testDeps.ctx)
		require.Nil(t, err)
		require.True(t, reflect.DeepEqual(got, stats))
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingCountPerWeekDay(testDeps.ctx).Return(nil, errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.GetBookingCountPerWeekDay(testDeps.ctx)
		require.Error(t, err)
		require.Equal(t, 0, len(got))
	})
}

func TestGetBookingCountPerGameInPeriod(t *testing.T) {
	stats := []bk.GameBookingCount{{Game: "test1", Count: 2}}
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	t.Run("success", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingCountPerGameInPeriod(testDeps.ctx, start, end).Return(stats, nil).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.GetBookingCountPerGameInPeriod(testDeps.ctx, start, end)
		require.Nil(t, err)
		require.True(t, reflect.DeepEqual(got, stats))
	})

	t.Run("repo error", func(t *testing.T) {
		ctrl, testDeps := newTestDeps(t)
		defer ctrl.Finish()

		testDeps.repo.EXPECT().GetBookingCountPerGameInPeriod(testDeps.ctx, start, end).Return(nil, errors.New("repo error")).Times(1)
		testDeps.client.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		got, err := testDeps.service.GetBookingCountPerGameInPeriod(testDeps.ctx, start, end)
		require.Error(t, err)
		require.Equal(t, 0, len(got))
	})
}
