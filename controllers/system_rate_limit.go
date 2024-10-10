package controllers

import (
	"time"

	"golang.org/x/time/rate"
)

type SystemRolloutReservation interface {
	Cancel()
}

type SystemRolloutRateLimiter interface {
	Reserve() (allowed bool, reservation SystemRolloutReservation)
}

var _ SystemRolloutRateLimiter = &systemRolloutRateLimiter{}

type systemRolloutRateLimiter struct {
	rateLimit *rate.Limiter
}

func NewSystemRolloutRateLimiter(operations int, interval time.Duration) SystemRolloutRateLimiter {
	return &systemRolloutRateLimiter{
		rateLimit: rate.NewLimiter(rate.Every(interval), operations),
	}
}

func (r *systemRolloutRateLimiter) Reserve() (allowed bool, reservation SystemRolloutReservation) {
	rateLimitReservation := r.rateLimit.Reserve()

	if !rateLimitReservation.OK() {
		return false, nil
	}

	if rateLimitReservation.Delay() > time.Second {
		rateLimitReservation.Cancel()
		return false, nil
	}

	return true, &onceReservation{Reservation: rateLimitReservation}
}

type onceReservation struct {
	*rate.Reservation
	canceled bool
}

func (r *onceReservation) Cancel() {
	if r.canceled {
		return
	}

	r.Reservation.Cancel()
	r.canceled = true
}

type noopReservation struct{}

func (*noopReservation) Cancel() {}

func NoopReservation() SystemRolloutReservation {
	return &noopReservation{}
}
