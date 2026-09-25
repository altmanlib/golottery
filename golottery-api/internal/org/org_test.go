package org

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"golottery/api/internal/bizerr"
	"golottery/api/internal/store"
)

var ops = Operator{Type: "platform", ID: "operator-1"}

func newService(t *testing.T) (*Service, *store.DB) {
	t.Helper()
	db := store.OpenTest(t)
	store.Reset(t, db, &LedgerEntry{}, &Quota{}, &Org{})
	return NewService(db.Gorm), db
}

func mustCreate(t *testing.T, s *Service, credits int) Summary {
	t.Helper()
	created, err := s.Create(context.Background(), CreateInput{Name: "  Acme  ", Contact: "Li", EventCredits: credits, MaxAttendees: 800}, ops)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return created
}

func expectCode(t *testing.T, err error, code bizerr.Code) {
	t.Helper()
	be, ok := bizerr.As(err)
	if !ok || be.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
}

func ledgerCount(t *testing.T, db *store.DB, id uuid.UUID) int64 {
	t.Helper()
	var n int64
	if err := db.Gorm.Model(&LedgerEntry{}).Where("org_id = ?", id).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateWritesOrgQuotaAndFirstEntry(t *testing.T) {
	s, db := newService(t)
	ctx := context.Background()

	created := mustCreate(t, s, 3)
	if created.Name != "Acme" || created.Status != StatusActive || created.EventCredits != 3 || created.MaxAttendees != 800 {
		t.Fatalf("created = %+v", created)
	}
	detail, err := s.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Ledger) != 1 || detail.Ledger[0].Delta != 3 || detail.Ledger[0].BalanceAfter != 3 || detail.Ledger[0].OperatorID != ops.ID {
		t.Fatalf("ledger = %+v", detail.Ledger)
	}

	empty := mustCreate(t, s, 0)
	if n := ledgerCount(t, db, empty.ID); n != 0 {
		t.Fatalf("zero-credit org ledger = %d, want 0", n)
	}
}

func TestCreateValidation(t *testing.T) {
	s, db := newService(t)
	ctx := context.Background()
	_, err := s.Create(ctx, CreateInput{Name: "  ", EventCredits: 1, MaxAttendees: 100}, ops)
	expectCode(t, err, bizerr.CodeNameRequired)
	_, err = s.Create(ctx, CreateInput{Name: "A", EventCredits: -1, MaxAttendees: 100}, ops)
	expectCode(t, err, bizerr.CodeBadRequest)
	_, err = s.Create(ctx, CreateInput{Name: "A", EventCredits: 1, MaxAttendees: 0}, ops)
	expectCode(t, err, bizerr.CodeBadRequest)

	var n int64
	db.Gorm.Model(&Org{}).Count(&n)
	if n != 0 {
		t.Fatalf("rejected creates left %d orgs", n)
	}
}

func TestCreateRollsBackTogether(t *testing.T) {
	s, db := newService(t)
	// A missing credit_ledger table makes the last insert fail; the org and quota must not survive it.
	if err := db.Gorm.Exec("ALTER TABLE credit_ledger RENAME TO credit_ledger_hidden").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Gorm.Exec("ALTER TABLE credit_ledger_hidden RENAME TO credit_ledger") })

	_, err := s.Create(context.Background(), CreateInput{Name: "A", EventCredits: 1, MaxAttendees: 100}, ops)
	expectCode(t, err, bizerr.CodeInternal)
	var orgs, quotas int64
	db.Gorm.Model(&Org{}).Count(&orgs)
	db.Gorm.Model(&Quota{}).Count(&quotas)
	if orgs != 0 || quotas != 0 {
		t.Fatalf("orgs = %d quotas = %d after failed create, want 0", orgs, quotas)
	}
}

func TestAdjustCredits(t *testing.T) {
	s, db := newService(t)
	ctx := context.Background()
	created := mustCreate(t, s, 2)

	entry, err := s.AdjustCredits(ctx, created.ID, 5, " 补充场次 ", ops)
	if err != nil {
		t.Fatal(err)
	}
	if entry.BalanceAfter != 7 || entry.Reason != "补充场次" {
		t.Fatalf("entry = %+v", entry)
	}

	_, err = s.AdjustCredits(ctx, created.ID, -8, "扣多了", ops)
	expectCode(t, err, bizerr.CodeConflict)
	detail, _ := s.Get(ctx, created.ID)
	if detail.EventCredits != 7 || len(detail.Ledger) != 2 {
		t.Fatalf("after rejected deduction credits = %d ledger = %d, want 7 and 2", detail.EventCredits, len(detail.Ledger))
	}
	if detail.Ledger[0].BalanceAfter != detail.EventCredits {
		t.Fatalf("latest balance_after = %d, credits = %d", detail.Ledger[0].BalanceAfter, detail.EventCredits)
	}

	if _, err := s.AdjustCredits(ctx, created.ID, -7, "清零", ops); err != nil {
		t.Fatalf("deduct to zero: %v", err)
	}

	_, err = s.AdjustCredits(ctx, created.ID, 0, "无变化", ops)
	expectCode(t, err, bizerr.CodeBadRequest)
	_, err = s.AdjustCredits(ctx, created.ID, 1, " ", ops)
	expectCode(t, err, bizerr.CodeBadRequest)
	_, err = s.AdjustCredits(ctx, uuid.New(), 1, "不存在", ops)
	expectCode(t, err, bizerr.CodeNotFound)
	if n := ledgerCount(t, db, created.ID); n != 3 {
		t.Fatalf("ledger = %d, want 3", n)
	}
}

func TestConcurrentDeductionsNeverGoNegative(t *testing.T) {
	s, db := newService(t)
	ctx := context.Background()
	created := mustCreate(t, s, 5)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, conflicts := 0, 0
	for range 10 {
		wg.Go(func() {
			_, err := s.AdjustCredits(ctx, created.ID, -1, "并发扣减", ops)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if be, isBiz := bizerr.As(err); isBiz && be.Code == bizerr.CodeConflict {
				conflicts++
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()

	detail, _ := s.Get(ctx, created.ID)
	if ok != 5 || conflicts != 5 || detail.EventCredits != 0 {
		t.Fatalf("ok = %d conflicts = %d credits = %d, want 5, 5, 0", ok, conflicts, detail.EventCredits)
	}
	if n := ledgerCount(t, db, created.ID); n != 6 {
		t.Fatalf("ledger = %d, want 6", n)
	}
}

func TestStatusToggleKeepsQuota(t *testing.T) {
	s, db := newService(t)
	ctx := context.Background()
	created := mustCreate(t, s, 4)

	for _, status := range []string{StatusDisabled, StatusDisabled, StatusActive, StatusActive} {
		if err := s.SetStatus(ctx, created.ID, status); err != nil {
			t.Fatalf("SetStatus(%s) error = %v", status, err)
		}
	}
	detail, _ := s.Get(ctx, created.ID)
	if detail.Status != StatusActive || detail.EventCredits != 4 || detail.MaxAttendees != 800 {
		t.Fatalf("detail = %+v", detail.Summary)
	}
	if n := ledgerCount(t, db, created.ID); n != 1 {
		t.Fatalf("ledger = %d, want 1", n)
	}
	expectCode(t, s.SetStatus(ctx, uuid.New(), StatusDisabled), bizerr.CodeNotFound)
}

func TestSetMaxAttendees(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	created := mustCreate(t, s, 1)

	if err := s.SetMaxAttendees(ctx, created.ID, 2000); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.Get(ctx, created.ID)
	if detail.MaxAttendees != 2000 || detail.EventCredits != 1 {
		t.Fatalf("detail = %+v", detail.Summary)
	}
	expectCode(t, s.SetMaxAttendees(ctx, created.ID, 0), bizerr.CodeBadRequest)
	expectCode(t, s.SetMaxAttendees(ctx, uuid.New(), 100), bizerr.CodeNotFound)
}

func TestListNewestFirstWithTotal(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	var ids []uuid.UUID
	for range 3 {
		ids = append(ids, mustCreate(t, s, 1).ID)
	}
	rows, total, err := s.List(ctx, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(rows) != 2 || rows[0].ID != ids[2] || rows[0].EventCredits != 1 {
		t.Fatalf("total = %d rows = %+v", total, rows)
	}
	rows, _, _ = s.List(ctx, 2, 2)
	if len(rows) != 1 || rows[0].ID != ids[0] {
		t.Fatalf("second page = %+v", rows)
	}
}
