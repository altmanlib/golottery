package guest

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
)

// Local table stubs so reset can clear draw rows without importing draw.
type drawResult struct{}

func (drawResult) TableName() string { return "draw_results" }

type drawLog struct{}

func (drawLog) TableName() string { return "draw_logs" }

// ResetCounts reports what a reset removed.
type ResetCounts struct {
	Unbound, Attempts, Requests, Sessions, Results, Logs int64
}

// ResetLiveData clears a trial run before check-in opens: bindings, check-ins, attempts,
// help requests, guest sessions and draw records. The roster, prizes, fence, staff and the
// credit already used stay. confirmName must equal the event name.
func (s *Service) ResetLiveData(ctx context.Context, orgID, eventID uuid.UUID, confirmName string) (ResetCounts, error) {
	var counts ResetCounts
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := s.ownedEvent(ctx, orgID, eventID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(confirmName) != ev.Name {
			return bizerr.New(bizerr.CodeBadRequest)
		}
		if ev.CheckinStart != nil && !s.now().Before(*ev.CheckinStart) {
			return bizerr.New(bizerr.CodeConflict)
		}
		res := tx.Model(&event.Attendee{}).
			Where("event_id = ? AND (openid IS NOT NULL OR status <> ?)", eventID, event.AttendeePending).
			Updates(map[string]any{"openid": nil, "status": event.AttendeePending, "checkin_at": nil, "checkin_method": nil, "checkin_by": nil})
		if res.Error != nil {
			return res.Error
		}
		counts.Unbound = res.RowsAffected
		for _, step := range []struct {
			model any
			n     *int64
		}{
			{&Attempt{}, &counts.Attempts},
			{&ManualRequest{}, &counts.Requests},
			{&Session{}, &counts.Sessions},
			{&drawResult{}, &counts.Results},
			{&drawLog{}, &counts.Logs},
		} {
			res := tx.Where("event_id = ?", eventID).Delete(step.model)
			if res.Error != nil {
				return res.Error
			}
			*step.n = res.RowsAffected
		}
		if err := tx.Model(&ev).Update("draw_version", 0).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ResetCounts{}, asBizErr(err)
	}
	return counts, nil
}
