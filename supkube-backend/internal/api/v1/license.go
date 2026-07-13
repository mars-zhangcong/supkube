package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/supkube/supkube-backend/pkg/license"
)

// LicenseStatusDTO is the /api/v1/license/status payload (for a future UI).
type LicenseStatusDTO struct {
	ID           string                `json:"id,omitempty"`
	Product      string                `json:"product,omitempty"`
	Customer     string                `json:"customer,omitempty"`
	Edition      string                `json:"edition,omitempty"`
	DateStart    *time.Time            `json:"dateStart,omitempty"`
	DateEnd      *time.Time            `json:"dateEnd,omitempty"`
	DaysLeft     int                   `json:"daysLeft"`
	Restrictions *license.Restrictions `json:"restrictions,omitempty"`
	Features     []string              `json:"features,omitempty"`
	State        string                `json:"state"` // licensed | grace | degraded | missing | invalid
	NodeCount    int                   `json:"nodeCount"`
	NodeExcluded int                   `json:"nodeExcluded"`
	Violations   []string              `json:"violations"`
	LastChecked  time.Time             `json:"lastChecked"`
}

// GetLicenseStatus returns the current license status from the controller's
// cached snapshot. Read-only; safe for viewer-and-above.
func GetLicenseStatus(c *gin.Context) {
	s := license.Snapshot()
	dto := LicenseStatusDTO{
		State:        s.State,
		DaysLeft:     s.DaysLeft(),
		NodeCount:    s.NodeCount,
		NodeExcluded: s.NodeExcluded,
		Violations:   s.Violations,
		LastChecked:  s.LastChecked,
	}
	if dto.Violations == nil {
		dto.Violations = []string{}
	}
	if l := s.License; l != nil {
		dto.ID, dto.Product, dto.Customer, dto.Edition = l.ID, l.Product, l.Customer, l.Edition
		ds, de := l.DateStart, l.DateEnd
		dto.DateStart, dto.DateEnd = &ds, &de
		r := l.Restrictions
		dto.Restrictions = &r
		dto.Features = l.Features
	}
	c.JSON(http.StatusOK, dto)
}

// LicenseWriteGate is gin middleware that blocks write operations (new backup
// tasks) with 402 when writes aren't licensed. Reads AND restores are never
// gated — a lapsed license must never hold a customer's recovery hostage.
func LicenseWriteGate(c *gin.Context) {
	if !license.Allowed() {
		c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
			"error": "license required or expired — creating new backups is blocked; restores and read access remain available",
			"state": license.Snapshot().State,
		})
		return
	}
	c.Next()
}
