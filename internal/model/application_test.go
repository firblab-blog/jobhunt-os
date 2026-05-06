package model

import "testing"

func TestApplicationStatusValidity(t *testing.T) {
	t.Parallel()

	for _, status := range []ApplicationStatus{
		StatusProspect,
		StatusApplied,
		StatusInterviewing,
		StatusOffer,
		StatusAccepted,
		StatusDeclined,
		StatusRejected,
		StatusWithdrawn,
		StatusArchived,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			if !ValidApplicationStatus(status) {
				t.Fatalf("ValidApplicationStatus(%q) = false, want true", status)
			}
		})
	}

	if ValidApplicationStatus("drafting") {
		t.Fatalf("ValidApplicationStatus(drafting) = true, want false")
	}
}

func TestPriorityValidity(t *testing.T) {
	t.Parallel()

	for _, priority := range []Priority{PriorityLow, PriorityNormal, PriorityHigh} {
		priority := priority
		t.Run(string(priority), func(t *testing.T) {
			t.Parallel()

			if !ValidPriority(priority) {
				t.Fatalf("ValidPriority(%q) = false, want true", priority)
			}
		})
	}

	if ValidPriority("urgent") {
		t.Fatalf("ValidPriority(urgent) = true, want false")
	}
}

func TestEventTypeValidity(t *testing.T) {
	t.Parallel()

	for _, eventType := range []EventType{
		EventApplied,
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
		EventOther,
	} {
		eventType := eventType
		t.Run(string(eventType), func(t *testing.T) {
			t.Parallel()

			if !ValidEventType(eventType) {
				t.Fatalf("ValidEventType(%q) = false, want true", eventType)
			}
		})
	}

	if ValidEventType("coffee_chat") {
		t.Fatalf("ValidEventType(coffee_chat) = true, want false")
	}
}

func TestCompensationValidation(t *testing.T) {
	t.Parallel()

	min := int64(100)
	max := int64(200)
	if err := ValidateCompensation(Compensation{MinCents: &min, MaxCents: &max}); err != nil {
		t.Fatalf("ValidateCompensation(valid) error = %v", err)
	}

	negative := int64(-1)
	if err := ValidateCompensation(Compensation{MinCents: &negative}); err == nil {
		t.Fatalf("ValidateCompensation(negative minimum) error = nil, want error")
	}

	if err := ValidateCompensation(Compensation{MinCents: &max, MaxCents: &min}); err == nil {
		t.Fatalf("ValidateCompensation(min > max) error = nil, want error")
	}
}

func TestApplicationValidateForCreate(t *testing.T) {
	t.Parallel()

	app := Application{
		Company:  "Northstar Systems",
		Role:     "Senior Platform Engineer",
		Status:   StatusProspect,
		Priority: PriorityNormal,
	}
	if err := app.ValidateForCreate(); err != nil {
		t.Fatalf("ValidateForCreate() error = %v", err)
	}

	for name, app := range map[string]Application{
		"missing company": {
			Role: "Senior Platform Engineer",
		},
		"missing role": {
			Company: "Northstar Systems",
		},
		"invalid status": {
			Company: "Northstar Systems",
			Role:    "Senior Platform Engineer",
			Status:  "drafting",
		},
		"invalid priority": {
			Company:  "Northstar Systems",
			Role:     "Senior Platform Engineer",
			Priority: "urgent",
		},
	} {
		name, app := name, app
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := app.ValidateForCreate(); err == nil {
				t.Fatalf("ValidateForCreate() error = nil, want error")
			}
		})
	}
}
