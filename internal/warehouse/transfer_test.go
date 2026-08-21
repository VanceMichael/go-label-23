package warehouse

import (
	"context"
	"sync"
	"testing"
	"time"
)

type transferValidatorFunc func(context.Context, Booking, string) error

func (fn transferValidatorFunc) ApproveTransfer(ctx context.Context, booking Booking, target string) error {
	return fn(ctx, booking, target)
}

func transferCalendar(t *testing.T, slotID, bookingID string, now time.Time, offset time.Duration) *Calendar {
	t.Helper()
	calendar := New()
	if err := calendar.Add(Slot{ID: slotID, Terminal: slotID, MaxKg: 100, OpenAt: now, CloseAt: now.Add(4 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := calendar.Book(context.Background(), Booking{ID: bookingID, ShipmentID: bookingID, SlotID: slotID, WeightKg: 20, Start: now.Add(time.Hour + offset), End: now.Add(2*time.Hour + offset)}); err != nil {
		t.Fatal(err)
	}
	return calendar
}

func TestOppositeTerminalTransfersCompleteWithoutHoldingEachOther(t *testing.T) {
	now := time.Now().UTC()

	t.Run("single transfer", func(t *testing.T) {
		source := transferCalendar(t, "slot-a", "booking-a", now, 0)
		destination := New()
		if err := destination.Add(Slot{ID: "slot-b", Terminal: "B", MaxKg: 100, OpenAt: now, CloseAt: now.Add(4 * time.Hour)}); err != nil {
			t.Fatal(err)
		}
		validator := transferValidatorFunc(func(context.Context, Booking, string) error { return nil })
		if err := Transfer(context.Background(), source, destination, "booking-a", "slot-b", validator); err != nil {
			t.Fatalf("single transfer error = %v", err)
		}
		if len(source.bookings) != 0 || destination.bookings["booking-a"].SlotID != "slot-b" {
			t.Fatalf("source=%+v destination=%+v", source.bookings, destination.bookings)
		}
	})

	t.Run("opposite transfers", func(t *testing.T) {
		left := transferCalendar(t, "slot-left", "booking-left", now, 0)
		right := transferCalendar(t, "slot-right", "booking-right", now, time.Hour)
		var entered sync.WaitGroup
		entered.Add(2)
		release := make(chan struct{})
		validator := transferValidatorFunc(func(context.Context, Booking, string) error {
			entered.Done()
			<-release
			return nil
		})
		results := make(chan error, 2)
		go func() {
			results <- Transfer(context.Background(), left, right, "booking-left", "slot-right", validator)
		}()
		go func() {
			results <- Transfer(context.Background(), right, left, "booking-right", "slot-left", validator)
		}()
		entered.Wait()
		close(release)
		for attempt := 0; attempt < 2; attempt++ {
			select {
			case err := <-results:
				if err != nil {
					t.Fatalf("opposite transfer error = %v", err)
				}
			case <-time.After(200 * time.Millisecond):
				t.Fatal("opposite terminal transfers blocked each other")
			}
		}
	})
}
