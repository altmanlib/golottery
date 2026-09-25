package guest

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"golottery/api/internal/bizerr"
)

// Staff roles. Admins may also change the check-in mode and fence on site.
const (
	RoleStaff = "staff"
	RoleAdmin = "admin"
)

// Staff grants one guest identity on-site powers for one event.
type Staff struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventID   uuid.UUID `gorm:"type:uuid;not null"`
	OpenID    string    `gorm:"column:openid;size:64;not null"`
	Role      string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName returns the event_staff table name.
func (Staff) TableName() string { return "event_staff" }

func (s *Service) staffRole(ctx context.Context, g Guest) (string, error) {
	var staff Staff
	err := s.db.WithContext(ctx).Where("event_id = ? AND openid = ?", g.EventID, g.OpenID).Take(&staff).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", bizerr.Wrap(bizerr.CodeInternal, err)
	}
	return staff.Role, nil
}
