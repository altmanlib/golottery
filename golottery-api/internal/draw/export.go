package draw

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
)

// ExportWinners writes every draw result of an event, voided rows included.
func (s *Service) ExportWinners(ctx context.Context, orgID, eventID uuid.UUID) ([]byte, string, error) {
	ev, err := s.ownedEvent(ctx, orgID, eventID)
	if err != nil {
		return nil, "", err
	}
	rows, err := s.listWinners(ctx, eventID, false)
	if err != nil {
		return nil, "", err
	}
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	header := []any{"时间", "奖项", "姓名", "部门", "状态", "作废原因", "作废时间"}
	if err := book.SetSheetRow(sheet, "A1", &header); err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	for i, r := range rows {
		status := "有效"
		reason := ""
		voided := ""
		if r.Status == StatusVoid {
			status = "已作废"
			if r.VoidReason != nil {
				reason = *r.VoidReason
			}
			if r.VoidedAt != nil {
				voided = event.DisplayTime(*r.VoidedAt)
			}
		}
		line := []any{event.DisplayTime(r.CreatedAt), r.PrizeName, r.AttendeeName, r.AttendeeDept, status, reason, voided}
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

// ExportLogs writes the draw log of an event.
func (s *Service) ExportLogs(ctx context.Context, orgID, eventID uuid.UUID) ([]byte, string, error) {
	ev, err := s.ownedEvent(ctx, orgID, eventID)
	if err != nil {
		return nil, "", err
	}
	type row struct {
		Log
		PrizeName string
	}
	var rows []row
	err = s.db.WithContext(ctx).Table("draw_logs").
		Select("draw_logs.*, prizes.name AS prize_name").
		Joins("JOIN prizes ON prizes.id = draw_logs.prize_id").
		Where("draw_logs.event_id = ?", eventID).
		Order("draw_logs.created_at, draw_logs.id").
		Scan(&rows).Error
	if err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	header := []any{"时间", "奖项", "奖池人数", "抽取人数", "请求号"}
	if err := book.SetSheetRow(sheet, "A1", &header); err != nil {
		return nil, "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	for i, r := range rows {
		line := []any{event.DisplayTime(r.CreatedAt), r.PrizeName, r.PoolSize, r.DrawCount, r.RequestID.String()}
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

func (s *Service) ownedEvent(ctx context.Context, orgID, eventID uuid.UUID) (event.Event, error) {
	var ev event.Event
	err := s.db.WithContext(ctx).Where("id = ? AND org_id = ?", eventID, orgID).Take(&ev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return event.Event{}, bizerr.New(bizerr.CodeNotFound)
	}
	if err != nil {
		return event.Event{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return ev, nil
}
