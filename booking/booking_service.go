package booking

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hanksha/tbz-booking-system-backend/discord"
)

type BookingRepository interface {
	GetActiveBookings(ctx context.Context) ([]Booking, error)
	GetBookingByID(ctx context.Context, id string) (Booking, error)
	GetBookingsPerUsername(ctx context.Context, username string) ([]Booking, error)
	InsertBooking(ctx context.Context, booking Booking) (Booking, error)
	UpdateBooking(ctx context.Context, booking Booking) error
	SetBookingStatus(ctx context.Context, id string, status string) error
	GetBookingCountPerGame(ctx context.Context) ([]GameBookingCount, error)
	GetBookingCountPerWeekDay(ctx context.Context) ([]WeekDayBookingCount, error)
	GetBookingCountPerGameInPeriod(ctx context.Context, start, end time.Time) ([]GameBookingCount, error)
}

type Service struct {
	repo      BookingRepository
	client    discord.DiscordClient
	channelID string
}

func NewService(repo BookingRepository, client discord.DiscordClient, channelID string) *Service {
	return &Service{repo: repo, client: client, channelID: channelID}
}

func (s *Service) GetActiveBookings(ctx context.Context) ([]Booking, error) {
	return s.repo.GetActiveBookings(ctx)
}

func (s *Service) FindBookingByID(ctx context.Context, id string) (Booking, error) {
	return s.repo.GetBookingByID(ctx, id)
}

func (s *Service) FindBookingsPerUsername(ctx context.Context, username string) ([]Booking, error) {
	return s.repo.GetBookingsPerUsername(ctx, username)
}

func (s *Service) CreateBooking(ctx context.Context, booking Booking) (Booking, error) {
	booking, err := s.repo.InsertBooking(ctx, booking)

	if err == nil {
		s.sendNotification(ctx, booking, NotificationOptions{message: "Nouvelle Réservation :calendar:"})
	}

	return booking, err
}

func (s *Service) ModifyBooking(ctx context.Context, updated Booking, user discord.DiscordUser) error {
	booking, err := s.repo.GetBookingByID(ctx, updated.ID)

	if err != nil {
		return err
	}

	if booking.Status != "pending" {
		return ErrInvalidBookingState
	}

	if !checkUserAllowed(booking, user) {
		return ErrNotAllowed
	}

	booking.Game = updated.Game
	booking.Points = updated.Points
	booking.Description = updated.Description
	booking.ReminderEnabled = updated.ReminderEnabled
	booking.DateTime = updated.DateTime
	booking.Players = updated.Players

	err = s.repo.UpdateBooking(ctx, booking)

	if err == nil {
		s.sendNotification(ctx, booking, NotificationOptions{message: "Réservation Modifiée :pencil:"})
	}

	return err
}

func (s *Service) AcceptBooking(ctx context.Context, id string) error {
	booking, err := s.repo.GetBookingByID(ctx, id)

	if err != nil {
		return err
	}

	if booking.Status == "canceled" || booking.Status == "accepted" {
		return ErrInvalidBookingState
	}

	err = s.repo.SetBookingStatus(ctx, id, "accepted")

	if err == nil {
		s.sendNotification(ctx, booking, NotificationOptions{message: "Réservation Acceptée :white_check_mark:"})
	}

	return err
}

func (s *Service) RefuseBooking(ctx context.Context, id, reason string) error {
	booking, err := s.repo.GetBookingByID(ctx, id)

	if err != nil {
		return err
	}

	if booking.Status == "refused" || booking.Status == "canceled" {
		return ErrInvalidBookingState
	}

	err = s.repo.SetBookingStatus(ctx, id, "refused")

	if err == nil {
		s.sendNotification(ctx, booking, NotificationOptions{message: "Réservation Refusée :no_entry:", reason: reason})
	}

	return err
}

func (s *Service) CancelBooking(ctx context.Context, id string, user discord.DiscordUser) error {
	booking, err := s.repo.GetBookingByID(ctx, id)

	if err != nil {
		return err
	}

	if booking.Status == "canceled" || booking.Status == "refused" {
		return ErrInvalidBookingState
	}

	if !checkUserAllowed(booking, user) {
		return ErrNotAllowed
	}

	err = s.repo.SetBookingStatus(ctx, id, "canceled")

	if err != nil {
		return fmt.Errorf("failed to cancel booking: %w", err)
	}

	s.sendNotification(ctx, booking, NotificationOptions{message: "Réservation Annulée :negative_squared_cross_mark:"})

	return nil
}

func (s *Service) GetBookingCountPerGame(ctx context.Context) ([]GameBookingCount, error) {
	return s.repo.GetBookingCountPerGame(ctx)
}

func (s *Service) GetBookingCountPerGameInPeriod(ctx context.Context, start, end time.Time) ([]GameBookingCount, error) {
	return s.repo.GetBookingCountPerGameInPeriod(ctx, start, end)
}

func (s *Service) GetBookingCountPerWeekDay(ctx context.Context) ([]WeekDayBookingCount, error) {
	return s.repo.GetBookingCountPerWeekDay(ctx)
}

func checkUserAllowed(booking Booking, user discord.DiscordUser) bool {
	if booking.UserID != user.ID && !slices.Contains(booking.Players, user.Username) {
		return false
	}

	return true
}

type NotificationOptions struct {
	message string
	reason  string
}

func (s *Service) sendNotification(ctx context.Context, booking Booking, options NotificationOptions) error {
	playerTags := []string{}

	for _, player := range booking.Players {
		_members, err := s.client.SearchMembers(ctx, player, 1)

		if err == nil && len(_members) != 0 {
			playerTags = append(playerTags, fmt.Sprintf("<@%v>", _members[0].User.ID))
		}
	}

	var userTag string

	if len(booking.UserID) != 0 {
		userTag = fmt.Sprintf("<@%v>", booking.UserID)
	} else {
		userTag = fmt.Sprintf("<@%v>", booking.Username)
	}

	loc, err := time.LoadLocation("Europe/Paris")

	if err != nil {
		loc = time.Now().Location()
	}

	description := "Aucune"

	if len(booking.Description) != 0 {
		description = booking.Description
	}

	embed := discord.Embed{
		Type:      "rich",
		ChannelID: s.channelID,
		Title:     options.message,
		Fields: []discord.EmbedField{
			{
				Name:   "Utilisateur",
				Value:  userTag,
				Inline: true,
			},
			{
				Name:   "Date et Heure",
				Value:  booking.DateTime.In(loc).Format(time.DateTime),
				Inline: true,
			},
			{
				Name:   "Jeu",
				Value:  booking.Game,
				Inline: true,
			},
			{
				Name:   "Points",
				Value:  strconv.Itoa(booking.Points),
				Inline: true,
			},
			{
				Name:   "Joueurs",
				Value:  strings.Join(playerTags, ", "),
				Inline: true,
			},
			{
				Name:   "Description",
				Value:  description,
				Inline: true,
			},
		},
	}

	if len(options.reason) != 0 {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:   "Raison",
			Value:  options.reason,
			Inline: true,
		})
	}

	s.client.SendMessage(ctx, s.channelID, discord.Message{
		Embeds: []discord.Embed{embed},
	})

	return nil
}
