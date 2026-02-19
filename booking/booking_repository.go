package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Repository struct{ conn *pgx.Conn }

func NewRepository(conn *pgx.Conn) *Repository {
	return &Repository{conn: conn}
}

func (r *Repository) GetActiveBookings(ctx context.Context) ([]Booking, error) {
	sql := `SELECT id, game, COALESCE("userId", ''), COALESCE(username, ''), points, description, status, COALESCE("reminderEnabled", false), "dateTime", players
            FROM "game-table-booking".booking
            WHERE "dateTime" >= $1;
        `

	rows, err := r.conn.Query(ctx, sql, time.Now())

	if err != nil {
		return nil, fmt.Errorf("failed to fetch bookings: %w", err)
	}

	defer rows.Close()

	var bookings []Booking

	for rows.Next() {
		var booking Booking
		err := rows.Scan(
			&booking.ID,
			&booking.Game,
			&booking.UserID,
			&booking.Username,
			&booking.Points,
			&booking.Description,
			&booking.Status,
			&booking.ReminderEnabled,
			&booking.DateTime,
			&booking.Players,
		)

		if err != nil {
			return nil, fmt.Errorf("error scanning booking row: %w", err)
		}

		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bookings rows: %w", err)
	}

	return bookings, nil
}

func (r *Repository) GetBookingByID(ctx context.Context, id string) (Booking, error) {
	sql := `
			SELECT id, game, COALESCE("userId", ''), COALESCE(username, ''), points, description, status, COALESCE("reminderEnabled", false), "dateTime", players 
			FROM "game-table-booking".booking 
			WHERE id=$1;
		`

	var booking Booking
	err := r.conn.QueryRow(ctx, sql, id).Scan(
		&booking.ID,
		&booking.Game,
		&booking.UserID,
		&booking.Username,
		&booking.Points,
		&booking.Description,
		&booking.Status,
		&booking.ReminderEnabled,
		&booking.DateTime,
		&booking.Players,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrBookingNotFound
	}

	if err != nil {
		return Booking{}, fmt.Errorf("failed to fetch booking with id %v: %w", id, err)
	}

	return booking, nil
}

func (r *Repository) GetBookingsPerUsername(ctx context.Context, username string) ([]Booking, error) {
	sql := `
            SELECT id, game, COALESCE("userId", ''), COALESCE(username, ''), points, description, status, COALESCE("reminderEnabled", false), "dateTime", players
            FROM "game-table-booking".booking
            WHERE username=$1 OR $1 = ANY(players);
        `

	rows, err := r.conn.Query(ctx, sql, username)

	if err != nil {
		return []Booking{}, fmt.Errorf("failed to fetch bookings for username '%v': %w", username, err)
	}

	defer rows.Close()

	var bookings []Booking

	for rows.Next() {
		var booking Booking
		err := rows.Scan(
			&booking.ID,
			&booking.Game,
			&booking.UserID,
			&booking.Username,
			&booking.Points,
			&booking.Description,
			&booking.Status,
			&booking.ReminderEnabled,
			&booking.DateTime,
			&booking.Players,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan bookings for username '%v': %w", username, err)
		}

		bookings = append(bookings, booking)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bookings rows: %w", err)
	}

	return bookings, nil
}

func (r *Repository) InsertBooking(ctx context.Context, booking Booking) (Booking, error) {
	sql := `
			INSERT INTO "game-table-booking".booking(
			game, "userId", username, points, description, status, "reminderEnabled", "dateTime", players)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id;
		`

	err := r.conn.QueryRow(ctx, sql,
		booking.Game,
		booking.UserID,
		booking.Username,
		booking.Points,
		booking.Description,
		"pending",
		booking.ReminderEnabled,
		booking.DateTime,
		booking.Players,
	).Scan(&booking.ID)

	if err != nil {
		return Booking{}, fmt.Errorf("failed to insert booking: %w", err)
	}

	return booking, nil
}

func (r *Repository) UpdateBooking(ctx context.Context, booking Booking) error {
	sql := `
			UPDATE "game-table-booking".booking
			SET
				game=$1,
				points=$2,
				description=$3,
				"reminderEnabled"=$4,
				"dateTime"=$5,
				players=$6
			WHERE id=$7;
		`

	tag, err := r.conn.Exec(ctx, sql,
		booking.Game,
		booking.Points,
		booking.Description,
		booking.ReminderEnabled,
		booking.DateTime,
		booking.Players,
		booking.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update booking: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrBookingNotFound
	}

	return nil
}

func (r *Repository) SetBookingStatus(ctx context.Context, id string, status string) error {
	sql := `
            UPDATE "game-table-booking".booking
            SET status=$1
            WHERE id=$2;
        `

	tag, err := r.conn.Exec(ctx, sql, status, id)

	if err != nil {
		return fmt.Errorf("failed to update booking '%v' status: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return ErrBookingNotFound
	}

	return err
}

type GameBookingCount struct {
	Game  string `json:"game"`
	Count int    `json:"bookingCount"`
}

type WeekDayBookingCount struct {
	WeekDay string `json:"dayOfWeek"`
	Count   int    `json:"bookingCount"`
}

func (r *Repository) GetBookingCountPerGame(ctx context.Context) ([]GameBookingCount, error) {
	sql := `
		SELECT booking.game, COUNT(*) as booking_count FROM "game-table-booking".booking 
		WHERE booking.status NOT IN ('pending', 'canceled', 'refused')
		GROUP BY booking.game
		ORDER BY booking_count DESC
	`

	rows, err := r.conn.Query(ctx, sql)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch bookings count per game: %w", err)
	}

	defer rows.Close()

	stats := []GameBookingCount{}

	for rows.Next() {
		var game string
		var count int
		err := rows.Scan(&game, &count)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		stats = append(stats, GameBookingCount{Game: game, Count: count})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bookings rows: %w", err)
	}

	return stats, err
}

func (r *Repository) GetBookingCountPerWeekDay(ctx context.Context) ([]WeekDayBookingCount, error) {
	sql := `
		SELECT 
			TO_CHAR("dateTime", 'Day') as day_of_week,
			COUNT(*) as booking_count
		FROM 
			"game-table-booking".booking
		WHERE booking.status NOT IN ('pending', 'canceled', 'refused')
		GROUP BY 
			TO_CHAR("dateTime", 'Day')
		ORDER BY 
			booking_count DESC;
	`

	rows, err := r.conn.Query(ctx, sql)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch bookings count per game: %w", err)
	}

	defer rows.Close()

	stats := []WeekDayBookingCount{}

	for rows.Next() {
		var weekDay string
		var count int
		err := rows.Scan(&weekDay, &count)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		stats = append(stats, WeekDayBookingCount{WeekDay: weekDay, Count: count})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bookings rows: %w", err)
	}

	return stats, err
}

func (r *Repository) GetBookingCountPerGameInPeriod(ctx context.Context, start, end time.Time) ([]GameBookingCount, error) {
	sql := `
		SELECT booking.game, COUNT(*) as booking_count FROM "game-table-booking".booking
		WHERE booking."dateTime" BETWEEN $1 AND $2
		AND booking.status NOT IN ('pending', 'canceled', 'refused')
		GROUP BY booking.game
		ORDER BY booking_count DESC
	`

	rows, err := r.conn.Query(ctx, sql, start, end)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch bookings count per game: %w", err)
	}

	defer rows.Close()

	stats := []GameBookingCount{}

	for rows.Next() {
		var game string
		var count int
		err := rows.Scan(&game, &count)

		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		stats = append(stats, GameBookingCount{Game: game, Count: count})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bookings rows: %w", err)
	}

	return stats, err
}
