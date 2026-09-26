package draw

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"golottery/api/internal/auth"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
)

const (
	maxVoidReasonLen  = 200
	pgUniqueViolation = "23505"
)

// Service implements host login, draws, voiding and exports.
type Service struct {
	db      *gorm.DB
	tokens  *auth.TokenIssuer
	limiter *auth.LoginLimiter
	hub     *Hub
	now     func() time.Time
	logger  *slog.Logger
}

// Config wires the collaborators of Service.
type Config struct {
	DB      *gorm.DB
	Tokens  *auth.TokenIssuer
	Limiter *auth.LoginLimiter
	Redis   *redis.Client
	Logger  *slog.Logger
}

// NewService builds a Service and its SSE hub.
func NewService(cfg Config) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{
		db:      cfg.DB,
		tokens:  cfg.Tokens,
		limiter: cfg.Limiter,
		now:     time.Now,
		logger:  logger,
	}
	s.hub = NewHub(cfg.Redis, logger, s.loadStats)
	return s
}

// Hub returns the SSE hub used by the host stream.
func (s *Service) Hub() *Hub { return s.hub }

// PrizeView is one prize with remaining quota for the host screen.
type PrizeView struct {
	ID        uuid.UUID
	Name      string
	Gift      string
	Quota     int
	Remaining int
	SortNo    int
}

// ResultView is a draw result with roster and prize labels.
type ResultView struct {
	Result
	PrizeName    string
	AttendeeName string
	AttendeeDept string
}

// Snapshot is the host screen state.
type Snapshot struct {
	EventName   string
	PublicID    string
	Status      string
	CheckedIn   int64
	DrawVersion int64
	Prizes      []PrizeView
	Winners     []ResultView
}

// PoolPerson is one eligible attendee for a draw.
type PoolPerson struct {
	ID   uuid.UUID
	Name string
	Dept string
}

// Batch is the response of one draw request.
type Batch struct {
	RequestID   uuid.UUID
	DrawVersion int64
	Results     []ResultView
}

// Snapshot returns the current draw state of the host's event.
func (s *Service) Snapshot(ctx context.Context, host HostSession) (Snapshot, error) {
	ev, err := s.loadEvent(ctx, host.EventID)
	if err != nil {
		return Snapshot{}, err
	}
	var checkedIn int64
	if err := s.db.WithContext(ctx).Model(&event.Attendee{}).
		Where("event_id = ? AND status = ?", host.EventID, event.AttendeeCheckedIn).
		Count(&checkedIn).Error; err != nil {
		return Snapshot{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	prizes, err := s.prizeViews(ctx, host.EventID)
	if err != nil {
		return Snapshot{}, err
	}
	winners, err := s.listWinners(ctx, host.EventID, true)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		EventName: ev.Name, PublicID: ev.PublicID, Status: ev.Status,
		CheckedIn: checkedIn, DrawVersion: ev.DrawVersion,
		Prizes: prizes, Winners: winners,
	}, nil
}

// Pool returns every checked-in person who can still win under the current rules.
func (s *Service) Pool(ctx context.Context, host HostSession) ([]PoolPerson, error) {
	ev, err := s.loadEvent(ctx, host.EventID)
	if err != nil {
		return nil, err
	}
	rows, err := s.eligiblePool(ctx, s.db, ev, uuid.Nil)
	if err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	out := make([]PoolPerson, len(rows))
	for i, row := range rows {
		out[i] = PoolPerson{ID: row.ID, Name: row.Name, Dept: row.Dept}
	}
	return out, nil
}

// Draw draws count winners for prizeID. The same requestID returns the first batch.
func (s *Service) Draw(ctx context.Context, host HostSession, prizeID uuid.UUID, count int, requestID uuid.UUID) (Batch, error) {
	if count < 1 || requestID == uuid.Nil {
		return Batch{}, bizerr.New(bizerr.CodeBadRequest)
	}
	var batch Batch
	var publish *Message
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := lockEventByID(ctx, tx, host.EventID)
		if err != nil {
			return err
		}
		if ev.Status != event.StatusReady {
			return bizerr.New(bizerr.CodeConflict)
		}
		var existing Log
		err = tx.Where("request_id = ?", requestID).Take(&existing).Error
		if err == nil {
			if existing.EventID != host.EventID {
				return bizerr.New(bizerr.CodeConflict)
			}
			results, err := s.resultsOfLog(ctx, tx, existing)
			if err != nil {
				return err
			}
			batch = Batch{RequestID: requestID, DrawVersion: ev.DrawVersion, Results: results}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var prize event.Prize
		if err := tx.Where("id = ? AND event_id = ?", prizeID, host.EventID).Take(&prize).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return bizerr.New(bizerr.CodeNotFound)
			}
			return err
		}
		var won int64
		if err := tx.Model(&Result{}).Where("prize_id = ? AND status = ?", prizeID, StatusValid).Count(&won).Error; err != nil {
			return err
		}
		if int(won)+count > prize.Quota {
			return bizerr.New(bizerr.CodeConflict)
		}
		pool, err := s.eligiblePool(ctx, tx, ev, prizeID)
		if err != nil {
			return err
		}
		if len(pool) < count {
			return bizerr.New(bizerr.CodeConflict)
		}
		picked, err := pickRandom(pool, count)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		exclusive := !ev.AllowMultiWin
		log := Log{
			ID: uuid.New(), OrgID: host.OrgID, EventID: host.EventID, PrizeID: prizeID,
			PoolSize: len(pool), DrawCount: count, OperatorID: host.HostID,
			RequestID: requestID, CreatedAt: now,
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		created := make([]Result, 0, count)
		for _, person := range picked {
			row := Result{
				ID: uuid.New(), OrgID: host.OrgID, EventID: host.EventID,
				PrizeID: prizeID, AttendeeID: person.ID, RequestID: requestID,
				Status: StatusValid, Exclusive: exclusive, CreatedAt: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			created = append(created, row)
		}
		if err := tx.Model(&ev).Update("draw_version", gorm.Expr("draw_version + 1")).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", ev.ID).Take(&ev).Error; err != nil {
			return err
		}
		views := make([]ResultView, len(created))
		for i, row := range created {
			person := picked[i]
			views[i] = ResultView{Result: row, PrizeName: prize.Name, AttendeeName: person.Name, AttendeeDept: person.Dept}
		}
		batch = Batch{RequestID: requestID, DrawVersion: ev.DrawVersion, Results: views}
		publish = &Message{Type: "draw", DrawVersion: ev.DrawVersion, Results: views}
		return nil
	})
	if isUniqueViolation(err) {
		// A concurrent request with the same request_id won the race; return that batch.
		return s.Draw(ctx, host, prizeID, count, requestID)
	}
	if err != nil {
		return Batch{}, asBizErr(err)
	}
	if publish != nil {
		s.hub.Publish(host.EventID, *publish)
	}
	return batch, nil
}

// Void marks a result as void. Already void rows are returned as they are.
func (s *Service) Void(ctx context.Context, host HostSession, resultID uuid.UUID, reason string) (ResultView, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > maxVoidReasonLen {
		return ResultView{}, bizerr.New(bizerr.CodeBadRequest)
	}
	var view ResultView
	var publish *Message
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev, err := lockEventByID(ctx, tx, host.EventID)
		if err != nil {
			return err
		}
		var row Result
		if err := tx.Where("id = ? AND event_id = ?", resultID, host.EventID).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return bizerr.New(bizerr.CodeNotFound)
			}
			return err
		}
		if row.Status != StatusVoid {
			now := s.now().UTC()
			row.Status = StatusVoid
			row.VoidReason = &reason
			row.VoidedAt = &now
			if err := tx.Model(&row).Updates(map[string]any{
				"status": StatusVoid, "void_reason": reason, "voided_at": now,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&ev).Update("draw_version", gorm.Expr("draw_version + 1")).Error; err != nil {
				return err
			}
			if err := tx.Where("id = ?", ev.ID).Take(&ev).Error; err != nil {
				return err
			}
		}
		loaded, err := s.loadResultView(ctx, tx, row.ID)
		if err != nil {
			return err
		}
		view = loaded
		publish = &Message{Type: "void", DrawVersion: ev.DrawVersion, Results: []ResultView{view}}
		return nil
	})
	if err != nil {
		return ResultView{}, asBizErr(err)
	}
	if publish != nil {
		s.hub.Publish(host.EventID, *publish)
	}
	return view, nil
}

func (s *Service) loadEvent(ctx context.Context, eventID uuid.UUID) (event.Event, error) {
	var ev event.Event
	if err := s.db.WithContext(ctx).Where("id = ?", eventID).Take(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return event.Event{}, bizerr.New(bizerr.CodeNotFound)
		}
		return event.Event{}, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return ev, nil
}

func lockEventByID(ctx context.Context, tx *gorm.DB, eventID uuid.UUID) (event.Event, error) {
	var ev event.Event
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", eventID).Take(&ev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return event.Event{}, bizerr.New(bizerr.CodeNotFound)
	}
	if err != nil {
		return event.Event{}, fmt.Errorf("draw: lock event: %w", err)
	}
	return ev, nil
}

func (s *Service) prizeViews(ctx context.Context, eventID uuid.UUID) ([]PrizeView, error) {
	var prizes []event.Prize
	if err := s.db.WithContext(ctx).Where("event_id = ?", eventID).Order("sort_no, id").Find(&prizes).Error; err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	out := make([]PrizeView, 0, len(prizes))
	for _, p := range prizes {
		var won int64
		if err := s.db.WithContext(ctx).Model(&Result{}).
			Where("prize_id = ? AND status = ?", p.ID, StatusValid).Count(&won).Error; err != nil {
			return nil, bizerr.Wrap(bizerr.CodeInternal, err)
		}
		remaining := p.Quota - int(won)
		if remaining < 0 {
			remaining = 0
		}
		out = append(out, PrizeView{
			ID: p.ID, Name: p.Name, Gift: p.Gift, Quota: p.Quota, Remaining: remaining, SortNo: p.SortNo,
		})
	}
	return out, nil
}

func (s *Service) listWinners(ctx context.Context, eventID uuid.UUID, validOnly bool) ([]ResultView, error) {
	q := s.db.WithContext(ctx).Table("draw_results").
		Select(`draw_results.*, prizes.name AS prize_name, attendees.name AS attendee_name, attendees.dept AS attendee_dept`).
		Joins("JOIN prizes ON prizes.id = draw_results.prize_id").
		Joins("JOIN attendees ON attendees.id = draw_results.attendee_id").
		Where("draw_results.event_id = ?", eventID).
		Order("draw_results.created_at, draw_results.id")
	if validOnly {
		q = q.Where("draw_results.status = ?", StatusValid)
	}
	var rows []ResultView
	if err := q.Scan(&rows).Error; err != nil {
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return rows, nil
}

func (s *Service) resultsOfLog(ctx context.Context, tx *gorm.DB, log Log) ([]ResultView, error) {
	var rows []ResultView
	err := tx.WithContext(ctx).Table("draw_results").
		Select(`draw_results.*, prizes.name AS prize_name, attendees.name AS attendee_name, attendees.dept AS attendee_dept`).
		Joins("JOIN prizes ON prizes.id = draw_results.prize_id").
		Joins("JOIN attendees ON attendees.id = draw_results.attendee_id").
		Where("draw_results.request_id = ?", log.RequestID).
		Order("draw_results.created_at, draw_results.id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) loadResultView(ctx context.Context, tx *gorm.DB, id uuid.UUID) (ResultView, error) {
	var row ResultView
	err := tx.WithContext(ctx).Table("draw_results").
		Select(`draw_results.*, prizes.name AS prize_name, attendees.name AS attendee_name, attendees.dept AS attendee_dept`).
		Joins("JOIN prizes ON prizes.id = draw_results.prize_id").
		Joins("JOIN attendees ON attendees.id = draw_results.attendee_id").
		Where("draw_results.id = ?", id).Take(&row).Error
	if err != nil {
		return ResultView{}, err
	}
	return row, nil
}

func (s *Service) eligiblePool(ctx context.Context, db *gorm.DB, ev event.Event, prizeID uuid.UUID) ([]event.Attendee, error) {
	q := db.WithContext(ctx).Where("event_id = ? AND status = ?", ev.ID, event.AttendeeCheckedIn)
	if !ev.AllowMultiWin {
		q = q.Where(`id NOT IN (SELECT attendee_id FROM draw_results WHERE event_id = ? AND status = ?)`, ev.ID, StatusValid)
	}
	if prizeID != uuid.Nil {
		// People already voided for this prize, or currently holding a valid win of it, stay out.
		q = q.Where(`id NOT IN (SELECT attendee_id FROM draw_results WHERE prize_id = ?)`, prizeID)
	}
	var rows []event.Attendee
	if err := q.Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func pickRandom(pool []event.Attendee, count int) ([]event.Attendee, error) {
	rest := append([]event.Attendee(nil), pool...)
	out := make([]event.Attendee, 0, count)
	for i := 0; i < count; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(rest))))
		if err != nil {
			return nil, fmt.Errorf("draw: random: %w", err)
		}
		idx := int(n.Int64())
		out = append(out, rest[idx])
		rest = append(rest[:idx], rest[idx+1:]...)
	}
	return out, nil
}

func (s *Service) loadStats(ctx context.Context, eventID uuid.UUID) (Stats, error) {
	ev, err := s.loadEvent(ctx, eventID)
	if err != nil {
		return Stats{}, err
	}
	var checkedIn int64
	if err := s.db.WithContext(ctx).Model(&event.Attendee{}).
		Where("event_id = ? AND status = ?", eventID, event.AttendeeCheckedIn).
		Count(&checkedIn).Error; err != nil {
		return Stats{}, err
	}
	return Stats{CheckedIn: checkedIn, DrawVersion: ev.DrawVersion}, nil
}

func asBizErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.Wrap(bizerr.CodeInternal, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
