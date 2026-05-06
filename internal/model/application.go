// Package model defines application/domain concepts shared across delivery layers.
package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ApplicationStatus is the persisted pipeline state for an application.
type ApplicationStatus string

// Application statuses match the applications.status schema constraint.
const (
	StatusProspect     ApplicationStatus = "prospect"
	StatusApplied      ApplicationStatus = "applied"
	StatusInterviewing ApplicationStatus = "interviewing"
	StatusOffer        ApplicationStatus = "offer"
	StatusAccepted     ApplicationStatus = "accepted"
	StatusDeclined     ApplicationStatus = "declined"
	StatusRejected     ApplicationStatus = "rejected"
	StatusWithdrawn    ApplicationStatus = "withdrawn"
	StatusArchived     ApplicationStatus = "archived"
)

// Valid reports whether the status is accepted by the schema.
func (s ApplicationStatus) Valid() bool {
	switch s {
	case StatusProspect,
		StatusApplied,
		StatusInterviewing,
		StatusOffer,
		StatusAccepted,
		StatusDeclined,
		StatusRejected,
		StatusWithdrawn,
		StatusArchived:
		return true
	default:
		return false
	}
}

// ValidApplicationStatus reports whether status is accepted by the schema.
func ValidApplicationStatus(status ApplicationStatus) bool {
	return status.Valid()
}

// Priority is the persisted importance marker for an application.
type Priority string

// Priority values match the applications.priority schema constraint.
const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
)

// Valid reports whether the priority is accepted by the schema.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh:
		return true
	default:
		return false
	}
}

// ValidPriority reports whether priority is accepted by the schema.
func ValidPriority(priority Priority) bool {
	return priority.Valid()
}

// Compensation maps the compensation columns on applications.
type Compensation struct {
	MinCents *int64
	MaxCents *int64
	Currency string
	Notes    string
}

// Validate checks the small invariants captured by the schema.
func (c Compensation) Validate() error {
	if c.MinCents != nil && *c.MinCents < 0 {
		return errors.New("compensation minimum cannot be negative")
	}
	if c.MaxCents != nil && *c.MaxCents < 0 {
		return errors.New("compensation maximum cannot be negative")
	}
	if c.MinCents != nil && c.MaxCents != nil && *c.MinCents > *c.MaxCents {
		return errors.New("compensation minimum cannot exceed maximum")
	}
	return nil
}

// ValidateCompensation checks the small invariants captured by the schema.
func ValidateCompensation(compensation Compensation) error {
	return compensation.Validate()
}

// NextAction maps the next_action columns on applications.
type NextAction struct {
	Due     *time.Time
	Summary string
}

// String returns the human-readable next-action summary.
func (n NextAction) String() string {
	return n.Summary
}

// Application is a job opportunity the user is tracking.
type Application struct {
	ID           string
	Company      string
	Role         string
	Status       ApplicationStatus
	Priority     Priority
	Source       string
	PostingURL   string
	Location     string
	Compensation Compensation
	Notes        string
	NextAction   NextAction
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ValidateForCreate checks basic fields needed before an application is stored.
func (a Application) ValidateForCreate() error {
	if strings.TrimSpace(a.Company) == "" {
		return errors.New("company is required")
	}
	if strings.TrimSpace(a.Role) == "" {
		return errors.New("role is required")
	}
	if a.Status != "" && !a.Status.Valid() {
		return fmt.Errorf("status %q is invalid", a.Status)
	}
	if a.Priority != "" && !a.Priority.Valid() {
		return fmt.Errorf("priority %q is invalid", a.Priority)
	}
	if strings.TrimSpace(a.PostingURL) != "" && !ValidHTTPURL(a.PostingURL) {
		return errors.New("posting URL must be a valid HTTP or HTTPS URL")
	}
	if err := a.Compensation.Validate(); err != nil {
		return err
	}
	return nil
}

// ValidHTTPURL reports whether raw is an absolute HTTP(S) URL.
func ValidHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return parsed.IsAbs() && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// EventType is the persisted category for an application timeline event.
type EventType string

// Event types match the application_events.event_type schema constraint.
const (
	EventApplied         EventType = "applied"
	EventRecruiterScreen EventType = "recruiter_screen"
	EventPhoneScreen     EventType = "phone_screen"
	EventInterview       EventType = "interview"
	EventOnsite          EventType = "onsite"
	EventTakeHome        EventType = "take_home"
	EventFollowUp        EventType = "follow_up"
	EventDeadline        EventType = "deadline"
	EventOffer           EventType = "offer"
	EventDecision        EventType = "decision"
	EventNote            EventType = "note"
	EventOther           EventType = "other"
)

// Valid reports whether the event type is accepted by the schema.
func (e EventType) Valid() bool {
	switch e {
	case EventApplied,
		EventRecruiterScreen,
		EventPhoneScreen,
		EventInterview,
		EventOnsite,
		EventTakeHome,
		EventFollowUp,
		EventDeadline,
		EventOffer,
		EventDecision,
		EventNote,
		EventOther:
		return true
	default:
		return false
	}
}

// ValidEventType reports whether eventType is accepted by the schema.
func ValidEventType(eventType EventType) bool {
	return eventType.Valid()
}

// ApplicationEvent is a timeline entry attached to an application.
type ApplicationEvent struct {
	ID            string
	ApplicationID string
	ContactID     string
	EventType     EventType
	OccurredAt    time.Time
	Summary       string
	Notes         string
	CreatedAt     time.Time
}
