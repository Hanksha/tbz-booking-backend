package booking

import "errors"

var ErrBookingNotFound = errors.New("booking not found")

var ErrInvalidBookingState = errors.New("invalid booking state")

var ErrNotAllowed = errors.New("not allowed to perform this operation")