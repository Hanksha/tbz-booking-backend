package booking

import "time"

type Booking struct {
	ID              string    `json:"id"`
	Game            string    `json:"game"`
	UserID          string    `json:"userId"`
	Username        string    `json:"username"`
	Points          int       `json:"points"`
	Description     string    `json:"description"`
	Status          string    `json:"status"` // accepted, refused, pending, canceled
	ReminderEnabled bool      `json:"reminderEnabled"`
	DateTime        time.Time `json:"dateTime"`
	Players         []string  `json:"players"`
}