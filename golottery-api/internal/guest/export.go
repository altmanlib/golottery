package guest

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
)

var attemptReasonLabels = map[string]string{
	"":                "",
	"already":         "已签到",
	"not_bound":       "未绑定",
	"event_not_open":  "活动未开放",
	"window_closed":   "不在时间内",
	"bad_coordinates": "坐标无效",
	"low_accuracy":    "精度不足",
	"out_of_range":    "超出范围",
	"no_fence":        "未设围栏",
}

// attemptRow is one attempt with the roster name, when the guest was bound.
type attemptRow struct {
	Attempt
	Name *string
}

// ExportAttempts writes every check-in tap of an event, coordinates included, for its admins.
func (s *Service) ExportAttempts(ctx context.Context, orgID, eventID uuid.UUID) ([]byte, string, error) {
	ev, err := s.ownedEvent(ctx, orgID, eventID)
	if err != nil {
		return nil, "", err
	}
	var rows []attemptRow
	err = s.db.WithContext(ctx).Table("checkin_attempts").
		Select("checkin_attempts.*, attendees.name AS name").
		Joins("LEFT JOIN attendees ON attendees.id = checkin_attempts.attendee_id").
		Where("checkin_attempts.event_id = ?", eventID).Order("checkin_attempts.id").Scan(&rows).Error
	if err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	header := []any{"时间", "姓名", "结果", "原因", "纬度", "经度", "精度（米）", "距离（米）"}
	if err := book.SetSheetRow(sheet, "A1", &header); err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	num := func(v *float64) any {
		if v == nil {
			return ""
		}
		return *v
	}
	for i, r := range rows {
		name := "未绑定"
		if r.Name != nil {
			name = *r.Name
		}
		result := "通过"
		if r.Result != "passed" {
			result = "未通过"
		}
		line := []any{event.DisplayTime(r.CreatedAt), name, result, attemptReasonLabels[r.Reason], num(r.Lat), num(r.Lng), num(r.AccuracyM), num(r.DistanceM)}
		if err := book.SetSheetRow(sheet, fmt.Sprintf("A%d", i+2), &line); err != nil {
			return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
		}
	}
	var out bytes.Buffer
	if err := book.Write(&out); err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return out.Bytes(), ev.Name, nil
}
