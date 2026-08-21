package warehouse

import (
	"context"
	"github.com/VanceMichael/go-base-airbridge/internal/domain"
	"sync"
	"time"
)

type Slot struct {
	ID       string
	Terminal string
	Zone     string
	MaxKg    int64
	UsedKg   int64
	OpenAt   time.Time
	CloseAt  time.Time
}
type Booking struct {
	ID         string
	ShipmentID string
	SlotID     string
	WeightKg   int64
	Start      time.Time
	End        time.Time
}
type Calendar struct {
	mu       sync.Mutex
	slots    map[string]Slot
	bookings map[string]Booking
}

type TransferValidator interface {
	ApproveTransfer(context.Context, Booking, string) error
}

func New() *Calendar { return &Calendar{slots: map[string]Slot{}, bookings: map[string]Booking{}} }
func (c *Calendar) Add(s Slot) error {
	if s.ID == "" || s.Terminal == "" || s.MaxKg <= 0 || !s.CloseAt.After(s.OpenAt) {
		return domain.ErrInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.slots[s.ID]; ok {
		return domain.ErrConflict
	}
	c.slots[s.ID] = s
	return nil
}
func (c *Calendar) Book(ctx context.Context, b Booking) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if b.ID == "" || b.ShipmentID == "" || b.SlotID == "" || b.WeightKg <= 0 || !b.End.After(b.Start) {
		return domain.ErrInvalid
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.slots[b.SlotID]
	if !ok {
		return domain.ErrNotFound
	}
	if b.Start.Before(s.OpenAt) || b.End.After(s.CloseAt) || s.UsedKg+b.WeightKg > s.MaxKg {
		return domain.ErrCapacity
	}
	for _, old := range c.bookings {
		if old.SlotID == b.SlotID && b.Start.Before(old.End) && old.Start.Before(b.End) {
			return domain.ErrConflict
		}
	}
	c.bookings[b.ID] = b
	s.UsedKg += b.WeightKg
	c.slots[b.SlotID] = s
	return nil
}
func (c *Calendar) Cancel(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.bookings[id]
	if !ok {
		return domain.ErrNotFound
	}
	s := c.slots[b.SlotID]
	s.UsedKg -= b.WeightKg
	delete(c.bookings, id)
	c.slots[b.SlotID] = s
	return nil
}

func Transfer(ctx context.Context, source, destination *Calendar, bookingID, targetSlotID string, validator TransferValidator) error {
	if source == nil || destination == nil || source == destination || bookingID == "" || targetSlotID == "" || validator == nil {
		return domain.ErrInvalid
	}
	source.mu.Lock()
	booking, ok := source.bookings[bookingID]
	defer source.mu.Unlock()
	if !ok {
		return domain.ErrNotFound
	}
	if err := validator.ApproveTransfer(ctx, booking, targetSlotID); err != nil {
		return err
	}
	destination.mu.Lock()
	defer destination.mu.Unlock()
	target, ok := destination.slots[targetSlotID]
	if !ok {
		return domain.ErrNotFound
	}
	if booking.Start.Before(target.OpenAt) || booking.End.After(target.CloseAt) || target.UsedKg+booking.WeightKg > target.MaxKg {
		return domain.ErrCapacity
	}
	for _, existing := range destination.bookings {
		if existing.SlotID == targetSlotID && booking.Start.Before(existing.End) && existing.Start.Before(booking.End) {
			return domain.ErrConflict
		}
	}
	oldSlot := source.slots[booking.SlotID]
	oldSlot.UsedKg -= booking.WeightKg
	target.UsedKg += booking.WeightKg
	delete(source.bookings, bookingID)
	booking.SlotID = targetSlotID
	destination.bookings[bookingID] = booking
	source.slots[oldSlot.ID] = oldSlot
	destination.slots[target.ID] = target
	return nil
}
func (c *Calendar) Utilization(id string) (float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.slots[id]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return float64(s.UsedKg) / float64(s.MaxKg), nil
}
