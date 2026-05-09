package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	jobhuntos "github.com/firblab-blog/jobhunt-os"
	"github.com/firblab-blog/jobhunt-os/internal/model"
	"github.com/firblab-blog/jobhunt-os/internal/store"
)

const (
	maxPostingPDFBytes       int64 = 20 << 20
	maxPostingMultipartBytes int64 = maxPostingPDFBytes + (2 << 20)
	staleActiveDays                = 7
	themeCookieName                = "jobhunt_theme"
	themeCookieMaxAge              = 60 * 60 * 24 * 365
	themeSystem                    = "system"
	themeLight                     = "light"
	themeDark                      = "dark"
)

type Server struct {
	mux       *http.ServeMux
	templates *template.Template
	store     store.ApplicationStore
	dataDir   string
}

const closedApplicationStatusFilter = "closed"

type Options struct {
	DataDir string
}

type dashboardData struct {
	Theme         pageTheme
	Applications  []dashboardApplication
	Metrics       []dashboardMetric
	NextActions   []dashboardNextAction
	PipelinePulse dashboardPipelinePulse
	Stats         dashboardStats
}

type dashboardApplication struct {
	model.Application
	Status        string
	StatusKey     string
	PriorityLabel string
	Updated       string
	NextActionDue string
	QueueLabel    string
	ActionText    string
}

type dashboardMetric struct {
	Label  string
	Value  string
	Action string
	Href   string
}

type dashboardPipelinePulse struct {
	Groups             []dashboardPipelinePulseGroup
	Signals            []dashboardPipelinePulseSignal
	Sankey             applicationsSankeyData
	TotalApplications  int
	ActiveApplications int
	ClosedApplications int
	DueFollowUps       int
	InterviewLoops     int
	DocumentCount      int
	StaleOpportunities int
	NoNextAction       int
	HasApplications    bool
}

type dashboardPipelinePulseGroup struct {
	Key      string
	Label    string
	Count    int
	Share    int
	Href     string
	Closed   bool
	Statuses []dashboardPipelinePulseStatus
}

type dashboardPipelinePulseStatus struct {
	Key   string
	Label string
	Count int
	Href  string
}

type dashboardPipelinePulseSignal struct {
	Key   string
	Label string
	Count int
	Href  string
}

type dashboardStats struct {
	PipelineCounts          []dashboardStatCount
	FollowUpHealth          dashboardFollowUpHealth
	PriorityMix             dashboardPriorityMix
	ThisWeekActivity        dashboardThisWeekActivity
	TotalApplications       int
	ActiveApplications      int
	StaleActiveApplications int
}

type dashboardStatCount struct {
	Key   string
	Label string
	Count int
	Href  string
}

type dashboardFollowUpHealth struct {
	Overdue      int
	DueToday     int
	Upcoming     int
	Unscheduled  int
	NoNextAction int
}

type dashboardPriorityMix struct {
	High   int
	Normal int
	Low    int
}

type dashboardThisWeekActivity struct {
	CreatedApplications int
	UpdatedApplications int
	Events              int
	Total               int
}

type dashboardNextAction struct {
	Text  string
	Href  string
	Meta  string
	State string
}

type applicationsIndexData struct {
	Theme              pageTheme
	Applications       []applicationListItem
	NextActions        []followUpItem
	NextActionCount    int
	Query              string
	Status             string
	StatusOptions      []selectOption
	TotalCount         int
	ResultCount        int
	HasFilters         bool
	HasMoreNextActions bool
}

type applicationsFlowData struct {
	Stages             []applicationsFlowStage
	ClosedStatuses     []applicationsFlowStatus
	Sankey             applicationsSankeyData
	TotalApplications  int
	ActiveApplications int
	ClosedApplications int
	HasApplications    bool
}

type applicationsFlowStage struct {
	Key      string
	Label    string
	Count    int
	Share    int
	Href     string
	Closed   bool
	Terminal bool
	Statuses []applicationsFlowStatus
}

type applicationsFlowStatus struct {
	Key   string
	Label string
	Count int
	Share int
	Href  string
}

type applicationsSankeyData struct {
	ViewBox string
	Nodes   []applicationsSankeyNode
	Links   []applicationsSankeyLink
}

type applicationsSankeyNode struct {
	Key      string
	Label    string
	Count    int
	Href     string
	X        int
	Y        int
	Height   int
	TextX    int
	TextY    int
	Anchor   string
	Terminal bool
	Closed   bool
}

type applicationsSankeyLink struct {
	Key   string
	Label string
	Count int
	Href  string
	Path  string
	Width int
}

type applicationListItem struct {
	model.Application
	StatusLabel   string
	PriorityLabel string
	Updated       string
	NextActionDue string
}

type applicationsFormData struct {
	Theme           pageTheme
	CSRFToken       template.HTML
	Values          applicationFormValues
	Errors          formErrors
	StatusOptions   []selectOption
	PriorityOptions []selectOption
}

type applicationShowData struct {
	Theme            pageTheme
	Application      model.Application
	StatusLabel      string
	Priority         string
	NextActionDue    string
	Created          string
	Updated          string
	CSRFToken        template.HTML
	Events           []applicationEventItem
	Documents        []applicationDocumentItem
	EventForm        applicationEventFormData
	StatusForm       applicationStatusFormData
	PostingForm      postingFormData
	EventTypeOptions []selectOption
	StatusOptions    []selectOption
}

type applicationFormValues struct {
	Company           string
	Role              string
	Status            string
	Priority          string
	Source            string
	PostingURL        string
	Location          string
	NextActionSummary string
	NextActionDue     string
	Notes             string
}

type applicationEventItem struct {
	model.ApplicationEvent
	EventTypeLabel string
	Occurred       string
}

type applicationEventFormData struct {
	Values applicationEventFormValues
	Errors formErrors
}

type applicationEventFormValues struct {
	EventType  string
	OccurredAt string
	Summary    string
	Notes      string
}

type applicationStatusFormData struct {
	Values applicationStatusFormValues
	Errors formErrors
}

type applicationStatusFormValues struct {
	Status            string
	NextActionSummary string
	NextActionDue     string
}

type postingFormData struct {
	Values postingFormValues
	Errors formErrors
}

type postingFormValues struct {
	PostingURL string
}

type applicationDocumentItem struct {
	model.ApplicationDocument
	TypeLabel string
	Updated   string
	ViewURL   string
}

type selectOption struct {
	Value string
	Label string
}

type documentsIndexData struct {
	Theme       pageTheme
	CSRFToken   template.HTML
	Documents   []documentItem
	Values      documentFormValues
	Errors      formErrors
	TypeOptions []selectOption
}

type documentItem struct {
	model.Document
	TypeLabel      string
	Updated        string
	ViewURL        string
	ReferenceLabel string
}

type documentShowData struct {
	Theme       pageTheme
	Document    model.Document
	TypeLabel   string
	Updated     string
	DownloadURL string
}

type documentFormValues struct {
	Name  string
	Type  string
	Notes string
}

type contactsIndexData struct {
	Theme     pageTheme
	CSRFToken template.HTML
	Contacts  []contactItem
	Values    contactFormValues
	Errors    formErrors
}

type contactItem struct {
	model.Contact
	Updated string
}

type contactFormValues struct {
	Name         string
	Organization string
	Role         string
	Email        string
	Phone        string
	Location     string
	Notes        string
}

type followUpItem struct {
	ID          string
	Company     string
	Role        string
	Status      model.ApplicationStatus
	StatusLabel string
	Summary     string
	Due         string
}

type settingsData struct {
	Theme       pageTheme
	GeneratedAt string
}

type pageTheme struct {
	Value    string
	Label    string
	ReturnTo string
	Options  []themeOption
}

type themeOption struct {
	Value   string
	Label   string
	Current bool
}

type exportSnapshot struct {
	Version      string              `json:"version"`
	ExportedAt   time.Time           `json:"exported_at"`
	Applications []exportApplication `json:"applications"`
	Documents    []model.Document    `json:"documents"`
	Contacts     []model.Contact     `json:"contacts"`
}

type exportApplication struct {
	model.Application
	Events []model.ApplicationEvent `json:"events"`
}

func New(appStore store.ApplicationStore) http.Handler {
	return NewWithOptions(appStore, Options{})
}

func NewWithOptions(appStore store.ApplicationStore, opts Options) http.Handler {
	templates := template.Must(template.New("").Funcs(templateFuncs()).ParseFS(jobhuntos.Assets, "web/templates/*.html"))

	s := &Server{
		mux:       http.NewServeMux(),
		templates: templates,
		store:     appStore,
		dataDir:   strings.TrimSpace(opts.DataDir),
	}

	staticFiles, err := fs.Sub(jobhuntos.Assets, "web/static")
	if err != nil {
		panic(err)
	}

	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /theme", s.themeUpdate)
	s.mux.HandleFunc("GET /{$}", s.home)
	s.mux.HandleFunc("GET /applications", s.applicationsIndex)
	s.mux.HandleFunc("GET /applications/new", s.applicationsNew)
	s.mux.HandleFunc("POST /applications", s.applicationsCreate)
	s.mux.HandleFunc("POST /applications/{id}/events", s.applicationsAddEvent)
	s.mux.HandleFunc("POST /applications/{id}/status", s.applicationsUpdateStatus)
	s.mux.HandleFunc("POST /applications/{id}/documents", s.applicationsUpdatePosting)
	s.mux.HandleFunc("GET /applications/{id}", s.applicationsShow)
	s.mux.HandleFunc("GET /documents", s.documentsIndex)
	s.mux.HandleFunc("POST /documents", s.documentsCreate)
	s.mux.HandleFunc("GET /documents/{id}", s.documentsShow)
	s.mux.HandleFunc("GET /documents/{id}/download", s.documentsDownload)
	s.mux.HandleFunc("GET /contacts", s.contactsIndex)
	s.mux.HandleFunc("POST /contacts", s.contactsCreate)
	s.mux.HandleFunc("GET /follow-ups", s.followUpsRedirect)
	s.mux.HandleFunc("GET /settings", s.settingsIndex)
	s.mux.HandleFunc("GET /backup", s.backupRedirect)
	s.mux.HandleFunc("GET /export.json", s.exportJSON)

	return s
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"queryEscape": queryEscapeTemplateValue,
	}
}

func queryEscapeTemplateValue(value string) template.URL {
	return template.URL(url.QueryEscape(value))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w.Header())

	rec := &statusRecorder{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
	start := time.Now()

	s.mux.ServeHTTP(rec, r)

	slog.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rec.status,
		"duration", time.Since(start),
	)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "home.html", s.dashboard(r))
}

func (s *Server) themeUpdate(w http.ResponseWriter, r *http.Request) {
	value := normalizeTheme(r.URL.Query().Get("theme"))
	http.SetCookie(w, &http.Cookie{
		Name:     themeCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   themeCookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	target := safeRedirectTarget(r, r.URL.Query().Get("return_to"))
	if target == "" {
		target = safeRedirectTarget(r, r.Referer())
	}
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) dashboard(r *http.Request) dashboardData {
	if s.store == nil {
		return fallbackDashboardData()
	}

	applications, err := s.store.ListApplications(r.Context())
	if err != nil {
		slog.Error("load dashboard applications", "error", err)
		return fallbackDashboardData()
	}

	active := 0
	needFollowUp := 0
	interviews := 0
	now := time.Now()
	nextActionApps := make([]model.Application, 0)

	for _, app := range applications {
		if isActiveStatus(app.Status) {
			active++
		}
		if app.Status == model.StatusInterviewing {
			interviews++
		}
		if app.NextAction.Summary != "" {
			if app.NextAction.Due == nil || !app.NextAction.Due.After(endOfDay(now)) {
				needFollowUp++
				nextActionApps = append(nextActionApps, app)
			}
		}
	}
	sort.SliceStable(nextActionApps, func(i, j int) bool {
		return dashboardNextActionLess(nextActionApps[i], nextActionApps[j], now)
	})
	if len(nextActionApps) > 5 {
		nextActionApps = nextActionApps[:5]
	}
	nextActions := make([]dashboardNextAction, 0, len(nextActionApps))
	for _, app := range nextActionApps {
		nextActions = append(nextActions, dashboardNextAction{
			Text:  nextActionText(app),
			Href:  applicationHref(app),
			Meta:  nextActionMeta(app, now),
			State: nextActionState(app.NextAction.Due, now),
		})
	}

	items := dashboardQueue(applications, now, 5)
	stats := dashboardStatsFor(applications, s.dashboardApplicationEvents(r.Context(), applications), now)

	documentCount, err := s.store.CountDocuments(r.Context())
	if err != nil {
		slog.Error("count dashboard documents", "error", err)
		documentCount = 0
	}
	pulse := dashboardPipelinePulseFor(applications, stats, documentCount)

	return dashboardData{
		Metrics: []dashboardMetric{
			{Label: "Active applications", Value: itoa(active), Action: "Work the queue", Href: "/applications"},
			{Label: "Need follow-up", Value: itoa(needFollowUp), Action: "Clear today", Href: "/applications#next-actions"},
			{Label: "Interview loops", Value: itoa(interviews), Action: "Prep next", Href: "/applications?status=interviewing"},
			{Label: "Documents", Value: itoa(documentCount), Action: "Review library", Href: "/documents"},
		},
		Applications:  items,
		NextActions:   nextActions,
		PipelinePulse: pulse,
		Stats:         stats,
	}
}

func fallbackDashboardData() dashboardData {
	return dashboardData{
		Metrics: []dashboardMetric{
			{Label: "Active applications", Value: "12", Action: "Work the queue", Href: "/applications"},
			{Label: "Need follow-up", Value: "3", Action: "Clear today", Href: "/applications#next-actions"},
			{Label: "Interview loops", Value: "2", Action: "Prep next", Href: "/applications?status=interviewing"},
			{Label: "Documents", Value: "5", Action: "Review library", Href: "/documents"},
		},
		Applications: []dashboardApplication{
			{
				Application: model.Application{
					Company:    "Northstar Systems",
					Role:       "Senior Platform Engineer",
					Status:     model.StatusInterviewing,
					NextAction: model.NextAction{Summary: "Prep system design notes"},
				},
				Status:        "Interviewing",
				StatusKey:     "interviewing",
				PriorityLabel: "High",
				Updated:       "Today",
				QueueLabel:    "Due today",
				ActionText:    "Prep system design notes",
			},
			{
				Application: model.Application{
					Company:    "Atlas Cloud",
					Role:       "Staff DevOps Engineer",
					Status:     model.StatusApplied,
					NextAction: model.NextAction{Summary: "Follow up with recruiter"},
				},
				Status:        "Applied",
				StatusKey:     "applied",
				PriorityLabel: "Normal",
				Updated:       "Yesterday",
				QueueLabel:    "Ready when you are",
				ActionText:    "Follow up with recruiter",
			},
			{
				Application: model.Application{
					Company:    "Signal Works",
					Role:       "Infrastructure Lead",
					Status:     model.StatusProspect,
					NextAction: model.NextAction{Summary: "Tailor cover letter"},
				},
				Status:        "Prospect",
				StatusKey:     "prospect",
				PriorityLabel: "High",
				Updated:       "Apr 30",
				QueueLabel:    "High priority",
				ActionText:    "Tailor cover letter",
			},
		},
		NextActions: []dashboardNextAction{
			{Text: "Prep system design notes for Northstar Systems.", Meta: "Interviewing", State: "Due today"},
			{Text: "Send recruiter follow-up for Atlas Cloud.", Meta: "Applied", State: "Ready when you are"},
			{Text: "Attach tailored cover letter to Signal Works draft.", Meta: "Prospect", State: "High priority"},
		},
		PipelinePulse: dashboardPipelinePulseFor([]model.Application{
			{Status: model.StatusProspect, NextAction: model.NextAction{Summary: "Tailor cover letter"}},
			{Status: model.StatusProspect, NextAction: model.NextAction{Summary: "Find hiring manager"}},
			{Status: model.StatusProspect},
			{Status: model.StatusProspect},
			{Status: model.StatusApplied, NextAction: model.NextAction{Summary: "Follow up"}},
			{Status: model.StatusApplied, NextAction: model.NextAction{Summary: "Send note"}},
			{Status: model.StatusApplied, NextAction: model.NextAction{Summary: "Check portal"}},
			{Status: model.StatusApplied, NextAction: model.NextAction{Summary: "Ask referral"}},
			{Status: model.StatusApplied, NextAction: model.NextAction{Summary: "Refresh notes"}},
			{Status: model.StatusInterviewing, NextAction: model.NextAction{Summary: "Prep system design notes"}},
			{Status: model.StatusInterviewing, NextAction: model.NextAction{Summary: "Prep behavioral notes"}},
			{Status: model.StatusOffer},
			{Status: model.StatusRejected},
			{Status: model.StatusRejected},
			{Status: model.StatusRejected},
			{Status: model.StatusWithdrawn},
		}, dashboardStats{
			FollowUpHealth: dashboardFollowUpHealth{
				Overdue:      1,
				DueToday:     1,
				Upcoming:     1,
				Unscheduled:  1,
				NoNextAction: 2,
			},
			TotalApplications:       16,
			ActiveApplications:      12,
			StaleActiveApplications: 2,
		}, 5),
		Stats: dashboardStats{
			PipelineCounts: []dashboardStatCount{
				{Key: string(model.StatusProspect), Label: "Prospect", Count: 4, Href: "/applications?status=prospect"},
				{Key: string(model.StatusApplied), Label: "Applied", Count: 5, Href: "/applications?status=applied"},
				{Key: string(model.StatusInterviewing), Label: "Interviewing", Count: 2, Href: "/applications?status=interviewing"},
				{Key: string(model.StatusOffer), Label: "Offer", Count: 1, Href: "/applications?status=offer"},
				{Key: string(model.StatusAccepted), Label: "Accepted", Count: 0, Href: "/applications?status=accepted"},
				{Key: string(model.StatusDeclined), Label: "Declined", Count: 0, Href: "/applications?status=declined"},
				{Key: string(model.StatusRejected), Label: "Rejected", Count: 3, Href: "/applications?status=rejected"},
				{Key: string(model.StatusWithdrawn), Label: "Withdrawn", Count: 1, Href: "/applications?status=withdrawn"},
				{Key: string(model.StatusArchived), Label: "Archived", Count: 0, Href: "/applications?status=archived"},
			},
			FollowUpHealth: dashboardFollowUpHealth{
				Overdue:      1,
				DueToday:     1,
				Upcoming:     1,
				Unscheduled:  1,
				NoNextAction: 2,
			},
			PriorityMix: dashboardPriorityMix{
				High:   5,
				Normal: 6,
				Low:    1,
			},
			ThisWeekActivity: dashboardThisWeekActivity{
				CreatedApplications: 2,
				UpdatedApplications: 5,
				Events:              4,
				Total:               11,
			},
			TotalApplications:       16,
			ActiveApplications:      12,
			StaleActiveApplications: 2,
		},
	}
}

func (s *Server) dashboardApplicationEvents(ctx context.Context, applications []model.Application) []model.ApplicationEvent {
	events := make([]model.ApplicationEvent, 0)
	for _, app := range applications {
		appEvents, err := s.store.ListApplicationEvents(ctx, app.ID)
		if err != nil {
			slog.Error("load dashboard application events", "application_id", app.ID, "error", err)
			continue
		}
		events = append(events, appEvents...)
	}
	return events
}

func dashboardStatsFor(applications []model.Application, events []model.ApplicationEvent, now time.Time) dashboardStats {
	pipelineCounts := dashboardPipelineCounts(applications)
	stats := dashboardStats{
		PipelineCounts:    pipelineCounts,
		TotalApplications: len(applications),
	}
	staleBefore := now.AddDate(0, 0, -staleActiveDays)
	weekStart := startOfWeek(now)

	for _, app := range applications {
		active := isActiveStatus(app.Status)
		if active {
			stats.ActiveApplications++
			stats.FollowUpHealth = dashboardFollowUpHealthFor(stats.FollowUpHealth, app, now)
			stats.PriorityMix = dashboardPriorityMixFor(stats.PriorityMix, app.Priority)
			if !app.UpdatedAt.IsZero() && !app.UpdatedAt.After(staleBefore) {
				stats.StaleActiveApplications++
			}
		}
		if inTimeWindow(app.CreatedAt, weekStart, now) {
			stats.ThisWeekActivity.CreatedApplications++
		}
		if inTimeWindow(app.UpdatedAt, weekStart, now) && !app.UpdatedAt.Equal(app.CreatedAt) {
			stats.ThisWeekActivity.UpdatedApplications++
		}
	}

	for _, event := range events {
		if inTimeWindow(event.OccurredAt, weekStart, now) {
			stats.ThisWeekActivity.Events++
		}
	}
	stats.ThisWeekActivity.Total = stats.ThisWeekActivity.CreatedApplications +
		stats.ThisWeekActivity.UpdatedApplications +
		stats.ThisWeekActivity.Events
	return stats
}

func dashboardPipelineCounts(applications []model.Application) []dashboardStatCount {
	counts := make(map[model.ApplicationStatus]int, len(applicationStatusOptions()))
	for _, app := range applications {
		counts[app.Status]++
	}

	items := make([]dashboardStatCount, 0, len(applicationStatusOptions()))
	for _, option := range applicationStatusOptions() {
		status := model.ApplicationStatus(option.Value)
		items = append(items, dashboardStatCount{
			Key:   option.Value,
			Label: option.Label,
			Count: counts[status],
			Href:  "/applications?status=" + option.Value,
		})
	}
	return items
}

func dashboardFollowUpHealthFor(health dashboardFollowUpHealth, app model.Application, now time.Time) dashboardFollowUpHealth {
	if app.NextAction.Summary == "" {
		health.NoNextAction++
		return health
	}
	if app.NextAction.Due == nil || app.NextAction.Due.IsZero() {
		health.Unscheduled++
		return health
	}
	switch dashboardDueRank(app.NextAction.Due, now) {
	case 0:
		health.Overdue++
	case 1:
		health.DueToday++
	default:
		health.Upcoming++
	}
	return health
}

func dashboardPriorityMixFor(mix dashboardPriorityMix, priority model.Priority) dashboardPriorityMix {
	switch priority {
	case model.PriorityHigh:
		mix.High++
	case model.PriorityLow:
		mix.Low++
	default:
		mix.Normal++
	}
	return mix
}

func dashboardQueue(applications []model.Application, now time.Time, limit int) []dashboardApplication {
	candidates := make([]model.Application, 0, len(applications))
	for _, app := range applications {
		if isActiveStatus(app.Status) {
			candidates = append(candidates, app)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return dashboardApplicationLess(candidates[i], candidates[j], now)
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	items := make([]dashboardApplication, 0, len(candidates))
	for _, app := range candidates {
		items = append(items, dashboardApplication{
			Application:   app,
			Status:        statusLabel(app.Status),
			StatusKey:     string(app.Status),
			PriorityLabel: priorityLabel(app.Priority),
			Updated:       shortDate(app.UpdatedAt),
			NextActionDue: optionalDate(app.NextAction.Due),
			QueueLabel:    dashboardQueueLabel(app, now),
			ActionText:    dashboardActionText(app),
		})
	}
	return items
}

func dashboardPipelinePulseFor(applications []model.Application, stats dashboardStats, documentCount int) dashboardPipelinePulse {
	groups := dashboardPipelinePulseGroups(applications)
	dueFollowUps := stats.FollowUpHealth.Overdue + stats.FollowUpHealth.DueToday
	interviewLoops := dashboardPipelinePulseGroupCount(groups, string(model.StatusInterviewing))
	closedApplications := dashboardPipelinePulseGroupCount(groups, "closed")

	return dashboardPipelinePulse{
		Groups:             groups,
		Signals:            dashboardPipelinePulseSignals(stats, documentCount, dueFollowUps, interviewLoops),
		Sankey:             applicationsFlowFor(applications).Sankey,
		TotalApplications:  len(applications),
		ActiveApplications: stats.ActiveApplications,
		ClosedApplications: closedApplications,
		DueFollowUps:       dueFollowUps,
		InterviewLoops:     interviewLoops,
		DocumentCount:      documentCount,
		StaleOpportunities: stats.StaleActiveApplications,
		NoNextAction:       stats.FollowUpHealth.NoNextAction,
		HasApplications:    len(applications) > 0,
	}
}

func dashboardPipelinePulseGroups(applications []model.Application) []dashboardPipelinePulseGroup {
	statusCounts := make(map[model.ApplicationStatus]int, len(applicationStatusOptions()))
	for _, app := range applications {
		statusCounts[app.Status]++
	}

	definitions := dashboardPipelinePulseGroupDefinitions()
	groups := make([]dashboardPipelinePulseGroup, 0, len(definitions))
	for _, definition := range definitions {
		statuses := make([]dashboardPipelinePulseStatus, 0, len(definition.statuses))
		groupCount := 0
		for _, status := range definition.statuses {
			count := statusCounts[status]
			groupCount += count
			statuses = append(statuses, dashboardPipelinePulseStatus{
				Key:   string(status),
				Label: statusLabel(status),
				Count: count,
				Href:  "/applications?status=" + string(status),
			})
		}

		groups = append(groups, dashboardPipelinePulseGroup{
			Key:      definition.key,
			Label:    definition.label,
			Count:    groupCount,
			Share:    dashboardPulseShare(groupCount, len(applications)),
			Href:     definition.href,
			Closed:   definition.closed,
			Statuses: statuses,
		})
	}
	return groups
}

type dashboardPipelinePulseGroupDefinition struct {
	key      string
	label    string
	href     string
	closed   bool
	statuses []model.ApplicationStatus
}

func dashboardPipelinePulseGroupDefinitions() []dashboardPipelinePulseGroupDefinition {
	return []dashboardPipelinePulseGroupDefinition{
		{
			key:      string(model.StatusProspect),
			label:    "Prospect",
			href:     "/applications?status=" + string(model.StatusProspect),
			statuses: []model.ApplicationStatus{model.StatusProspect},
		},
		{
			key:      string(model.StatusApplied),
			label:    "Applied",
			href:     "/applications?status=" + string(model.StatusApplied),
			statuses: []model.ApplicationStatus{model.StatusApplied},
		},
		{
			key:      string(model.StatusInterviewing),
			label:    "Interviewing",
			href:     "/applications?status=" + string(model.StatusInterviewing),
			statuses: []model.ApplicationStatus{model.StatusInterviewing},
		},
		{
			key:      string(model.StatusOffer),
			label:    "Offer",
			href:     "/applications?status=" + string(model.StatusOffer),
			statuses: []model.ApplicationStatus{model.StatusOffer},
		},
		{
			key:      string(model.StatusAccepted),
			label:    "Accepted",
			href:     "/applications?status=" + string(model.StatusAccepted),
			statuses: []model.ApplicationStatus{model.StatusAccepted},
		},
		{
			key:    "closed",
			label:  "Closed",
			href:   "/applications?status=" + closedApplicationStatusFilter,
			closed: true,
			statuses: []model.ApplicationStatus{
				model.StatusDeclined,
				model.StatusRejected,
				model.StatusWithdrawn,
				model.StatusArchived,
			},
		},
	}
}

func dashboardPipelinePulseSignals(stats dashboardStats, documentCount int, dueFollowUps int, interviewLoops int) []dashboardPipelinePulseSignal {
	return []dashboardPipelinePulseSignal{
		{Key: "active_applications", Label: "Active applications", Count: stats.ActiveApplications, Href: "/applications"},
		{Key: "due_follow_ups", Label: "Due follow-ups", Count: dueFollowUps, Href: "/applications#next-actions"},
		{Key: "interview_loops", Label: "Interview loops", Count: interviewLoops, Href: "/applications?status=interviewing"},
		{Key: "documents", Label: "Documents", Count: documentCount, Href: "/documents"},
		{Key: "stale_opportunities", Label: "Stale opportunities", Count: stats.StaleActiveApplications, Href: "/applications"},
		{Key: "no_next_action", Label: "No next action", Count: stats.FollowUpHealth.NoNextAction, Href: "/applications"},
	}
}

func dashboardPipelinePulseGroupCount(groups []dashboardPipelinePulseGroup, key string) int {
	for _, group := range groups {
		if group.Key == key {
			return group.Count
		}
	}
	return 0
}

func dashboardPulseShare(count int, total int) int {
	if total <= 0 || count <= 0 {
		return 0
	}
	return (count*100 + total/2) / total
}

func dashboardApplicationLess(left, right model.Application, now time.Time) bool {
	if cmp := dashboardDueRank(left.NextAction.Due, now) - dashboardDueRank(right.NextAction.Due, now); cmp != 0 {
		return cmp < 0
	}
	if cmp := dashboardStatusRank(left.Status) - dashboardStatusRank(right.Status); cmp != 0 {
		return cmp < 0
	}
	if cmp := dashboardPriorityRank(left.Priority) - dashboardPriorityRank(right.Priority); cmp != 0 {
		return cmp < 0
	}
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	if strings.EqualFold(left.Company, right.Company) {
		return strings.ToLower(left.Role) < strings.ToLower(right.Role)
	}
	return strings.ToLower(left.Company) < strings.ToLower(right.Company)
}

func dashboardNextActionLess(left, right model.Application, now time.Time) bool {
	if cmp := dashboardDueRank(left.NextAction.Due, now) - dashboardDueRank(right.NextAction.Due, now); cmp != 0 {
		return cmp < 0
	}
	if cmp := dashboardPriorityRank(left.Priority) - dashboardPriorityRank(right.Priority); cmp != 0 {
		return cmp < 0
	}
	return dashboardApplicationLess(left, right, now)
}

func dashboardDueRank(due *time.Time, now time.Time) int {
	if due == nil || due.IsZero() {
		return 3
	}
	if due.Before(startOfDay(now)) {
		return 0
	}
	if !due.After(endOfDay(now)) {
		return 1
	}
	return 2
}

func dashboardStatusRank(status model.ApplicationStatus) int {
	switch status {
	case model.StatusOffer:
		return 0
	case model.StatusInterviewing:
		return 1
	case model.StatusApplied:
		return 2
	case model.StatusProspect:
		return 3
	default:
		return 4
	}
}

func dashboardPriorityRank(priority model.Priority) int {
	switch priority {
	case model.PriorityHigh:
		return 0
	case model.PriorityLow:
		return 2
	default:
		return 1
	}
}

func dashboardQueueLabel(app model.Application, now time.Time) string {
	if app.NextAction.Summary != "" {
		return nextActionState(app.NextAction.Due, now)
	}
	switch app.Status {
	case model.StatusProspect:
		return "Choose next move"
	case model.StatusApplied:
		return "Add a follow-up"
	case model.StatusInterviewing:
		return "Prep the loop"
	case model.StatusOffer:
		return "Review decision"
	default:
		return "Set next action"
	}
}

func dashboardActionText(app model.Application) string {
	if app.NextAction.Summary != "" {
		return app.NextAction.Summary
	}
	switch app.Status {
	case model.StatusProspect:
		return "Decide whether to apply."
	case model.StatusApplied:
		return "Add the next follow-up."
	case model.StatusInterviewing:
		return "Capture prep notes."
	case model.StatusOffer:
		return "Review offer details."
	default:
		return "Set a next action."
	}
}

func nextActionMeta(app model.Application, now time.Time) string {
	meta := statusLabel(app.Status)
	if app.Priority != "" {
		meta += " / " + priorityLabel(app.Priority)
	}
	if app.NextAction.Due != nil && !app.NextAction.Due.IsZero() && !sameDay(*app.NextAction.Due, now) {
		meta += " / " + optionalDate(app.NextAction.Due)
	}
	return meta
}

func nextActionState(due *time.Time, now time.Time) string {
	if due == nil || due.IsZero() {
		return "Ready when you are"
	}
	if due.Before(startOfDay(now)) {
		return "Overdue"
	}
	if !due.After(endOfDay(now)) {
		return "Due today"
	}
	return "Due " + due.Format("Jan 2")
}

func applicationHref(app model.Application) string {
	if app.ID == "" {
		return ""
	}
	return "/applications/" + app.ID
}

func (s *Server) applicationsIndex(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	applications, err := s.store.ListApplications(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	filtered := filterApplications(applications, query, status)
	nextActions := followUpItems(applications)
	visibleNextActions := nextActions
	if len(visibleNextActions) > 5 {
		visibleNextActions = visibleNextActions[:5]
	}

	items := make([]applicationListItem, 0, len(filtered))
	for _, app := range filtered {
		items = append(items, applicationListItem{
			Application:   app,
			StatusLabel:   statusLabel(app.Status),
			PriorityLabel: priorityLabel(app.Priority),
			Updated:       shortDate(app.UpdatedAt),
			NextActionDue: optionalDate(app.NextAction.Due),
		})
	}

	s.render(w, r, "applications_index.html", applicationsIndexData{
		Applications:       items,
		NextActions:        visibleNextActions,
		NextActionCount:    len(nextActions),
		Query:              query,
		Status:             status,
		StatusOptions:      applicationStatusFilterOptions(),
		TotalCount:         len(applications),
		ResultCount:        len(items),
		HasFilters:         query != "" || status != "",
		HasMoreNextActions: len(nextActions) > len(visibleNextActions),
	})
}

func applicationsFlowFor(applications []model.Application) applicationsFlowData {
	statusCounts := make(map[model.ApplicationStatus]int, len(applicationStatusOptions()))
	activeApplications := 0
	for _, app := range applications {
		statusCounts[app.Status]++
		if isActiveStatus(app.Status) {
			activeApplications++
		}
	}

	definitions := applicationsFlowStageDefinitions()
	stages := make([]applicationsFlowStage, 0, len(definitions))
	closedStatuses := make([]applicationsFlowStatus, 0, 4)
	closedApplications := 0
	totalApplications := len(applications)

	for _, definition := range definitions {
		statuses := make([]applicationsFlowStatus, 0, len(definition.statuses))
		stageCount := 0
		for _, status := range definition.statuses {
			count := statusCounts[status]
			stageCount += count
			statusItem := applicationsFlowStatus{
				Key:   string(status),
				Label: statusLabel(status),
				Count: count,
				Share: applicationsFlowShare(count, totalApplications),
				Href:  applicationsStatusFilterHref(status),
			}
			statuses = append(statuses, statusItem)
			if definition.closed {
				closedStatuses = append(closedStatuses, statusItem)
			}
		}
		if definition.closed {
			closedApplications = stageCount
		}

		stages = append(stages, applicationsFlowStage{
			Key:      definition.key,
			Label:    definition.label,
			Count:    stageCount,
			Share:    applicationsFlowShare(stageCount, totalApplications),
			Href:     definition.href,
			Closed:   definition.closed,
			Terminal: definition.terminal,
			Statuses: statuses,
		})
	}

	return applicationsFlowData{
		Stages:             stages,
		ClosedStatuses:     closedStatuses,
		Sankey:             applicationsSankeyFor(stages, closedStatuses, totalApplications),
		TotalApplications:  totalApplications,
		ActiveApplications: activeApplications,
		ClosedApplications: closedApplications,
		HasApplications:    totalApplications > 0,
	}
}

func applicationsSankeyFor(stages []applicationsFlowStage, closedStatuses []applicationsFlowStatus, total int) applicationsSankeyData {
	const (
		nodeWidth = 9
	)
	nodes := []applicationsSankeyNode{
		applicationsSankeyNodeFor("tracked", "Tracked", total, "/applications", 24, 86, applicationsSankeyRootHeight(total), false, false),
	}
	links := make([]applicationsSankeyLink, 0, len(stages)+len(closedStatuses))

	stagePositions := map[string][2]int{
		string(model.StatusProspect):     {238, 276},
		string(model.StatusApplied):      {432, 132},
		string(model.StatusInterviewing): {626, 110},
		string(model.StatusOffer):        {820, 132},
		string(model.StatusAccepted):     {1036, 78},
		"closed":                         {1036, 258},
	}
	outcomePositions := map[string][2]int{
		string(model.StatusDeclined):  {1206, 214},
		string(model.StatusRejected):  {1206, 260},
		string(model.StatusWithdrawn): {1206, 306},
		string(model.StatusArchived):  {1206, 352},
	}

	root := nodes[0]
	rootExitY := map[string]int{
		string(model.StatusProspect):     root.Y + 248,
		string(model.StatusApplied):      root.Y + 92,
		string(model.StatusInterviewing): root.Y + 60,
		string(model.StatusOffer):        root.Y + 126,
		string(model.StatusAccepted):     root.Y + 28,
		"closed":                         root.Y + 205,
	}

	for _, stage := range stages {
		position, ok := stagePositions[stage.Key]
		if !ok {
			continue
		}
		node := applicationsSankeyNodeFor(stage.Key, stage.Label, stage.Count, stage.Href, position[0], position[1], applicationsSankeyNodeHeight(stage.Count, total), stage.Terminal, stage.Closed)
		nodes = append(nodes, node)
		if stage.Count > 0 {
			links = append(links, applicationsSankeyLinkFor(
				"tracked-"+stage.Key,
				"Tracked to "+stage.Label,
				stage.Count,
				stage.Href,
				root.X+nodeWidth,
				rootExitY[stage.Key],
				node.X,
				node.Y+node.Height/2,
				total,
			))
		}
	}

	closedNode := applicationsSankeyFindNode(nodes, "closed")
	if closedNode.Count > 0 {
		for _, status := range closedStatuses {
			position, ok := outcomePositions[status.Key]
			if !ok {
				continue
			}
			node := applicationsSankeyNodeFor(status.Key, status.Label, status.Count, status.Href, position[0], position[1], applicationsSankeyOutcomeHeight(status.Count, total), true, true)
			nodes = append(nodes, node)
			if status.Count > 0 {
				links = append(links, applicationsSankeyLinkFor(
					"closed-"+status.Key,
					"Closed to "+status.Label,
					status.Count,
					status.Href,
					closedNode.X+nodeWidth,
					closedNode.Y+closedNode.Height/2,
					node.X,
					node.Y+node.Height/2,
					total,
				))
			}
		}
	}

	return applicationsSankeyData{
		ViewBox: "0 0 1280 430",
		Nodes:   nodes,
		Links:   links,
	}
}

func applicationsSankeyNodeFor(key string, label string, count int, href string, x int, y int, height int, terminal bool, closed bool) applicationsSankeyNode {
	anchor := "start"
	textX := x + 16
	if x >= 1036 {
		anchor = "end"
		textX = x - 12
	}
	return applicationsSankeyNode{
		Key:      key,
		Label:    label,
		Count:    count,
		Href:     href,
		X:        x,
		Y:        y,
		Height:   height,
		TextX:    textX,
		TextY:    y + height/2 - 8,
		Anchor:   anchor,
		Terminal: terminal,
		Closed:   closed,
	}
}

func applicationsSankeyLinkFor(key string, label string, count int, href string, x1 int, y1 int, x2 int, y2 int, total int) applicationsSankeyLink {
	control := (x2 - x1) / 2
	if control < 70 {
		control = 70
	}
	return applicationsSankeyLink{
		Key:   key,
		Label: label,
		Count: count,
		Href:  href,
		Path:  fmt.Sprintf("M %d %d C %d %d, %d %d, %d %d", x1, y1, x1+control, y1, x2-control, y2, x2, y2),
		Width: applicationsSankeyLinkWidth(count, total),
	}
}

func applicationsSankeyFindNode(nodes []applicationsSankeyNode, key string) applicationsSankeyNode {
	for _, node := range nodes {
		if node.Key == key {
			return node
		}
	}
	return applicationsSankeyNode{}
}

func applicationsSankeyRootHeight(total int) int {
	if total <= 0 {
		return 180
	}
	return 288
}

func applicationsSankeyNodeHeight(count int, total int) int {
	if total <= 0 || count <= 0 {
		return 22
	}
	height := 34 + (count * 150 / total)
	if height > 190 {
		return 190
	}
	return height
}

func applicationsSankeyOutcomeHeight(count int, total int) int {
	if total <= 0 || count <= 0 {
		return 18
	}
	height := 24 + (count * 90 / total)
	if height > 120 {
		return 120
	}
	return height
}

func applicationsSankeyLinkWidth(count int, total int) int {
	if total <= 0 || count <= 0 {
		return 0
	}
	width := 8 + (count * 64 / total)
	if width > 72 {
		return 72
	}
	return width
}

type applicationsFlowStageDefinition struct {
	key      string
	label    string
	href     string
	closed   bool
	terminal bool
	statuses []model.ApplicationStatus
}

func applicationsFlowStageDefinitions() []applicationsFlowStageDefinition {
	return []applicationsFlowStageDefinition{
		{
			key:      string(model.StatusProspect),
			label:    "Prospect",
			href:     applicationsStatusFilterHref(model.StatusProspect),
			statuses: []model.ApplicationStatus{model.StatusProspect},
		},
		{
			key:      string(model.StatusApplied),
			label:    "Applied",
			href:     applicationsStatusFilterHref(model.StatusApplied),
			statuses: []model.ApplicationStatus{model.StatusApplied},
		},
		{
			key:      string(model.StatusInterviewing),
			label:    "Interviewing",
			href:     applicationsStatusFilterHref(model.StatusInterviewing),
			statuses: []model.ApplicationStatus{model.StatusInterviewing},
		},
		{
			key:      string(model.StatusOffer),
			label:    "Offer",
			href:     applicationsStatusFilterHref(model.StatusOffer),
			statuses: []model.ApplicationStatus{model.StatusOffer},
		},
		{
			key:      string(model.StatusAccepted),
			label:    "Accepted",
			href:     applicationsStatusFilterHref(model.StatusAccepted),
			terminal: true,
			statuses: []model.ApplicationStatus{model.StatusAccepted},
		},
		{
			key:    "closed",
			label:  "Closed",
			href:   "/applications?status=" + closedApplicationStatusFilter,
			closed: true,
			statuses: []model.ApplicationStatus{
				model.StatusDeclined,
				model.StatusRejected,
				model.StatusWithdrawn,
				model.StatusArchived,
			},
		},
	}
}

func applicationsStatusFilterHref(status model.ApplicationStatus) string {
	return "/applications?status=" + string(status)
}

func applicationsFlowShare(count int, total int) int {
	if total <= 0 || count <= 0 {
		return 0
	}
	return (count*100 + total/2) / total
}

func (s *Server) documentsIndex(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	documents, err := s.store.ListDocuments(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}

	s.renderDocumentsIndex(w, r, documents, documentFormValues{
		Type: string(model.DocumentResume),
	}, formErrors{}, http.StatusOK)
}

func (s *Server) documentsCreate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	form, file, fileHeader, err := s.parseDocumentUploadForm(w, r)
	if err != nil {
		handleFormParseError(w, err)
		return
	}
	if file != nil {
		defer file.Close()
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	values := documentFormValuesFromForm(form)
	document := documentFromUploadForm(form)
	if !hasPostingPDF(fileHeader) {
		form.errors.Add("document_file", "Choose a PDF to upload.")
	}
	if hasPostingPDF(fileHeader) && s.dataDir == "" {
		form.errors.Add("document_file", "Document storage is not configured.")
	}
	if hasPostingPDF(fileHeader) && !form.errors.Any() {
		if err := validatePostingPDF(file, fileHeader); err != nil {
			form.errors.Add("document_file", "Choose a valid PDF under 20 MB.")
		}
	}
	if !form.errors.Any() {
		if _, err := s.saveDocumentPDF(r.Context(), document, file, fileHeader); err != nil {
			form.errors.Add("form", "Could not save document. Please check the fields and try again.")
			slog.Error("create document", "error", err)
		} else {
			http.Redirect(w, r, "/documents", http.StatusSeeOther)
			return
		}
	}

	documents, err := s.store.ListDocuments(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.renderDocumentsIndex(w, r, documents, values, form.errors, http.StatusUnprocessableEntity)
}

func (s *Server) documentsShow(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	document, err := s.store.GetDocument(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}
	if documentDownloadURL(document) == "" {
		http.NotFound(w, r)
		return
	}

	s.renderWithStatus(w, r, "documents_show.html", documentShowData{
		Document:    document,
		TypeLabel:   documentTypeLabel(document.Type),
		Updated:     longDate(document.UpdatedAt),
		DownloadURL: documentDownloadURL(document),
	}, http.StatusOK)
}

func (s *Server) documentsDownload(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}
	if s.dataDir == "" {
		serverError(w, r, errors.New("document storage directory is not configured"))
		return
	}

	document, err := s.store.GetDocument(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	path, err := safeDocumentPath(s.dataDir, document.StoragePath)
	if err != nil {
		serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+downloadFileName(document.Name)+`.pdf"`)
	w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'")
	w.Header().Del("X-Frame-Options")
	http.ServeFile(w, r, path)
}

func (s *Server) contactsIndex(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	contacts, err := s.store.ListContacts(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}

	s.renderContactsIndex(w, r, contacts, contactFormValues{}, formErrors{}, http.StatusOK)
}

func (s *Server) contactsCreate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	form, err := parseLimitedForm(w, r, defaultMaxFormBytes)
	if err != nil {
		handleFormParseError(w, err)
		return
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	values := contactFormValuesFromForm(form)
	contact := contactFromForm(form)
	if !form.errors.Any() {
		if _, err := s.store.CreateContact(r.Context(), contact); err != nil {
			form.errors.Add("form", "Could not save contact. Please check the fields and try again.")
			slog.Error("create contact", "error", err)
		} else {
			http.Redirect(w, r, "/contacts", http.StatusSeeOther)
			return
		}
	}

	contacts, err := s.store.ListContacts(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}
	s.renderContactsIndex(w, r, contacts, values, form.errors, http.StatusUnprocessableEntity)
}

func (s *Server) followUpsRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/applications#next-actions", http.StatusSeeOther)
}

func (s *Server) settingsIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings.html", settingsData{GeneratedAt: time.Now().Format(time.RFC1123)})
}

func (s *Server) backupRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) exportJSON(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	snapshot, err := s.exportSnapshot(r)
	if err != nil {
		serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="jobhunt-os-export.json"`)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		slog.Error("encode export snapshot", "error", err)
	}
}

func (s *Server) applicationsNew(w http.ResponseWriter, r *http.Request) {
	s.renderApplicationForm(w, r, applicationFormValues{
		Status:   string(model.StatusProspect),
		Priority: string(model.PriorityNormal),
	}, formErrors{}, http.StatusOK)
}

func (s *Server) applicationsCreate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	form, file, fileHeader, err := s.parseApplicationCreateForm(w, r)
	if err != nil {
		if errors.Is(err, errFormTooLarge) {
			http.Error(w, "form body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form body", http.StatusBadRequest)
		return
	}
	if file != nil {
		defer file.Close()
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	values := applicationFormValuesFromForm(form)
	app := applicationFromForm(form)
	hasFile := hasPostingPDF(fileHeader)
	if hasFile && s.dataDir == "" {
		form.errors.Add("posting_pdf", "Document storage is not configured.")
	}
	if hasFile && !form.errors.Any() {
		if err := validatePostingPDF(file, fileHeader); err != nil {
			form.errors.Add("posting_pdf", "Choose a valid PDF under 20 MB.")
		}
	}
	if !form.errors.Any() {
		created, err := s.store.CreateApplication(r.Context(), app)
		if err != nil {
			form.errors.Add("form", "Could not save application. Please check the fields and try again.")
			slog.Error("create application", "error", err)
		} else {
			if hasFile {
				if _, err := s.savePostingPDF(r.Context(), created, file, fileHeader); err != nil {
					slog.Error("save posting pdf for new application", "application_id", created.ID, "error", err)
				}
			}
			http.Redirect(w, r, "/applications/"+created.ID, http.StatusSeeOther)
			return
		}
	}

	s.renderApplicationForm(w, r, values, form.errors, http.StatusUnprocessableEntity)
}

func (s *Server) applicationsShow(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	app, events, documents, err := s.applicationDetail(r, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	s.renderApplicationDetail(w, r, app, events, documents, applicationEventFormData{
		Values: defaultApplicationEventFormValues(time.Now()),
	}, applicationStatusFormData{
		Values: applicationStatusFormValuesFromApplication(app),
	}, postingFormData{
		Values: postingFormValuesFromApplication(app),
	}, http.StatusOK)
}

func (s *Server) applicationsAddEvent(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	form, err := parseLimitedForm(w, r, defaultMaxFormBytes)
	if err != nil {
		if errors.Is(err, errFormTooLarge) {
			http.Error(w, "form body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form body", http.StatusBadRequest)
		return
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	app, events, documents, err := s.applicationDetail(r, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	values := applicationEventFormValuesFromForm(form)
	event := applicationEventFromForm(form, app.ID)
	if !form.errors.Any() {
		if _, err := s.store.AddApplicationEvent(r.Context(), event); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			form.errors.Add("form", "Could not save timeline event. Please check the fields and try again.")
			slog.Error("add application event", "error", err)
		} else {
			http.Redirect(w, r, "/applications/"+app.ID, http.StatusSeeOther)
			return
		}
	}

	s.renderApplicationDetail(w, r, app, events, documents, applicationEventFormData{
		Values: values,
		Errors: form.errors,
	}, applicationStatusFormData{
		Values: applicationStatusFormValuesFromApplication(app),
	}, postingFormData{
		Values: postingFormValuesFromApplication(app),
	}, http.StatusUnprocessableEntity)
}

func (s *Server) applicationsUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	form, err := parseLimitedForm(w, r, defaultMaxFormBytes)
	if err != nil {
		if errors.Is(err, errFormTooLarge) {
			http.Error(w, "form body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form body", http.StatusBadRequest)
		return
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	app, events, documents, err := s.applicationDetail(r, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	values := applicationStatusFormValuesFromForm(form)
	status, nextAction := applicationStatusUpdateFromForm(form)
	if !form.errors.Any() {
		if _, err := s.store.UpdateApplicationStatusAndNextAction(r.Context(), app.ID, status, nextAction); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			form.errors.Add("form", "Could not update status. Please check the fields and try again.")
			slog.Error("update application status", "error", err)
		} else {
			http.Redirect(w, r, "/applications/"+app.ID, http.StatusSeeOther)
			return
		}
	}

	s.renderApplicationDetail(w, r, app, events, documents, applicationEventFormData{
		Values: defaultApplicationEventFormValues(time.Now()),
	}, applicationStatusFormData{
		Values: values,
		Errors: form.errors,
	}, postingFormData{
		Values: postingFormValuesFromApplication(app),
	}, http.StatusUnprocessableEntity)
}

func (s *Server) applicationsUpdatePosting(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	form, file, fileHeader, err := parsePostingMultipartForm(w, r)
	if err != nil {
		if errors.Is(err, errFormTooLarge) {
			http.Error(w, "form body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid form body", http.StatusBadRequest)
		return
	}
	if file != nil {
		defer file.Close()
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	app, events, documents, err := s.applicationDetail(r, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	values := postingFormValuesFromForm(form)
	postingURL := strings.TrimSpace(values.PostingURL)
	if postingURL != "" && !model.ValidHTTPURL(postingURL) {
		form.errors.Add("posting_url", "Original link must be a valid HTTP or HTTPS URL.")
	}
	hasFile := fileHeader != nil && fileHeader.Filename != ""
	if !hasFile && postingURL == app.PostingURL {
		form.errors.Add("form", "Update the original link or choose a PDF to attach.")
	}
	if hasFile && s.dataDir == "" {
		form.errors.Add("posting_pdf", "Document storage is not configured.")
	}

	if !form.errors.Any() {
		if postingURL != app.PostingURL {
			updated, err := s.store.UpdateApplicationPostingURL(r.Context(), app.ID, postingURL)
			if err != nil {
				form.errors.Add("form", "Could not update original link.")
				slog.Error("update application posting url", "error", err)
			} else {
				app = updated
			}
		}
	}

	if !form.errors.Any() && hasFile {
		document, err := s.savePostingPDF(r.Context(), app, file, fileHeader)
		if err != nil {
			form.errors.Add("posting_pdf", "Could not save PDF. Choose a valid PDF under 20 MB.")
			slog.Error("save posting pdf", "error", err)
		} else {
			documents = append([]model.ApplicationDocument{document}, documents...)
		}
	}

	if !form.errors.Any() {
		http.Redirect(w, r, "/applications/"+app.ID, http.StatusSeeOther)
		return
	}

	s.renderApplicationDetail(w, r, app, events, documents, applicationEventFormData{
		Values: defaultApplicationEventFormValues(time.Now()),
	}, applicationStatusFormData{
		Values: applicationStatusFormValuesFromApplication(app),
	}, postingFormData{
		Values: values,
		Errors: form.errors,
	}, http.StatusUnprocessableEntity)
}

func (s *Server) renderApplicationForm(w http.ResponseWriter, r *http.Request, values applicationFormValues, errors formErrors, status int) {
	token, err := issueCSRFToken(w, time.Now())
	if err != nil {
		serverError(w, r, err)
		return
	}

	s.renderWithStatus(w, r, "applications_new.html", applicationsFormData{
		CSRFToken:       csrfField(token),
		Values:          values,
		Errors:          errors,
		StatusOptions:   applicationStatusOptions(),
		PriorityOptions: applicationPriorityOptions(),
	}, status)
}

func (s *Server) renderApplicationDetail(w http.ResponseWriter, r *http.Request, app model.Application, events []model.ApplicationEvent, documents []model.ApplicationDocument, eventForm applicationEventFormData, statusForm applicationStatusFormData, postingForm postingFormData, status int) {
	token, err := issueCSRFToken(w, time.Now())
	if err != nil {
		serverError(w, r, err)
		return
	}

	items := make([]applicationEventItem, 0, len(events))
	for _, event := range events {
		items = append(items, applicationEventItem{
			ApplicationEvent: event,
			EventTypeLabel:   eventTypeLabel(event.EventType),
			Occurred:         longDate(event.OccurredAt),
		})
	}

	documentItems := make([]applicationDocumentItem, 0, len(documents))
	for _, document := range documents {
		documentItems = append(documentItems, applicationDocumentItem{
			ApplicationDocument: document,
			TypeLabel:           documentTypeLabel(document.Document.Type),
			Updated:             shortDate(document.Document.UpdatedAt),
			ViewURL:             documentViewURL(document.Document),
		})
	}

	s.renderWithStatus(w, r, "applications_show.html", applicationShowData{
		Application:      app,
		StatusLabel:      statusLabel(app.Status),
		Priority:         priorityLabel(app.Priority),
		NextActionDue:    optionalDate(app.NextAction.Due),
		Created:          longDate(app.CreatedAt),
		Updated:          longDate(app.UpdatedAt),
		CSRFToken:        csrfField(token),
		Events:           items,
		Documents:        documentItems,
		EventForm:        eventForm,
		StatusForm:       statusForm,
		PostingForm:      postingForm,
		EventTypeOptions: applicationEventTypeOptions(),
		StatusOptions:    applicationStatusOptions(),
	}, status)
}

func (s *Server) renderDocumentsIndex(w http.ResponseWriter, r *http.Request, documents []model.Document, values documentFormValues, errors formErrors, status int) {
	token, err := issueCSRFToken(w, time.Now())
	if err != nil {
		serverError(w, r, err)
		return
	}

	items := make([]documentItem, 0, len(documents))
	for _, document := range documents {
		items = append(items, documentItem{
			Document:       document,
			TypeLabel:      documentTypeLabel(document.Type),
			Updated:        shortDate(document.UpdatedAt),
			ViewURL:        documentViewURL(document),
			ReferenceLabel: documentReferenceLabel(document),
		})
	}

	s.renderWithStatus(w, r, "documents_index.html", documentsIndexData{
		CSRFToken:   csrfField(token),
		Documents:   items,
		Values:      values,
		Errors:      errors,
		TypeOptions: documentTypeOptions(),
	}, status)
}

func (s *Server) renderContactsIndex(w http.ResponseWriter, r *http.Request, contacts []model.Contact, values contactFormValues, errors formErrors, status int) {
	token, err := issueCSRFToken(w, time.Now())
	if err != nil {
		serverError(w, r, err)
		return
	}

	items := make([]contactItem, 0, len(contacts))
	for _, contact := range contacts {
		items = append(items, contactItem{
			Contact: contact,
			Updated: shortDate(contact.UpdatedAt),
		})
	}

	s.renderWithStatus(w, r, "contacts_index.html", contactsIndexData{
		CSRFToken: csrfField(token),
		Contacts:  items,
		Values:    values,
		Errors:    errors,
	}, status)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	s.renderWithStatus(w, r, name, data, http.StatusOK)
}

func (s *Server) renderWithStatus(w http.ResponseWriter, r *http.Request, name string, data any, status int) {
	var body bytes.Buffer
	data = withPageTheme(r, data)
	if err := s.templates.ExecuteTemplate(&body, name, data); err != nil {
		serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func withPageTheme(r *http.Request, data any) any {
	theme := pageThemeForRequest(r)
	themeType := reflect.TypeOf(theme)
	themeValue := reflect.ValueOf(theme)
	value := reflect.ValueOf(data)
	if !value.IsValid() {
		return data
	}

	if value.Kind() == reflect.Pointer {
		if value.IsNil() || value.Elem().Kind() != reflect.Struct {
			return data
		}
		copyValue := reflect.New(value.Elem().Type())
		copyValue.Elem().Set(value.Elem())
		field := copyValue.Elem().FieldByName("Theme")
		if field.IsValid() && field.CanSet() && field.Type() == themeType {
			field.Set(themeValue)
			return copyValue.Interface()
		}
		return data
	}

	if value.Kind() != reflect.Struct {
		return data
	}
	copyValue := reflect.New(value.Type()).Elem()
	copyValue.Set(value)
	field := copyValue.FieldByName("Theme")
	if field.IsValid() && field.CanSet() && field.Type() == themeType {
		field.Set(themeValue)
		return copyValue.Interface()
	}
	return data
}

func pageThemeForRequest(r *http.Request) pageTheme {
	value := themeFromRequest(r)
	options := []themeOption{
		{Value: themeSystem, Label: "System", Current: value == themeSystem},
		{Value: themeLight, Label: "Light", Current: value == themeLight},
		{Value: themeDark, Label: "Dark", Current: value == themeDark},
	}
	return pageTheme{
		Value:    value,
		Label:    themeLabel(value),
		ReturnTo: currentRequestTarget(r),
		Options:  options,
	}
}

func themeFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(themeCookieName)
	if err != nil {
		return themeSystem
	}
	return normalizeTheme(cookie.Value)
}

func normalizeTheme(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case themeLight:
		return themeLight
	case themeDark:
		return themeDark
	default:
		return themeSystem
	}
}

func themeLabel(value string) string {
	switch normalizeTheme(value) {
	case themeLight:
		return "Light"
	case themeDark:
		return "Dark"
	default:
		return "System"
	}
}

func currentRequestTarget(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	target := r.URL.RequestURI()
	if target == "" {
		return "/"
	}
	return target
}

func safeRedirectTarget(r *http.Request, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() || parsed.Host != "" {
		if r == nil || !sameRequestHost(r, parsed) {
			return ""
		}
		if parsed.Path == "" {
			parsed.Path = "/"
		}
	} else if parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return ""
	}

	if parsed.Path == "/theme" {
		return "/"
	}
	if parsed.RawQuery == "" {
		return parsed.EscapedPath()
	}
	return parsed.EscapedPath() + "?" + parsed.RawQuery
}

func sameRequestHost(r *http.Request, target *url.URL) bool {
	if r.Host == "" || target.Host == "" || !strings.EqualFold(r.Host, target.Host) {
		return false
	}
	return target.Scheme == "http" || target.Scheme == "https"
}

func parsePostingMultipartForm(w http.ResponseWriter, r *http.Request) (*formData, multipart.File, *multipart.FileHeader, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostingMultipartBytes)
	if err := r.ParseMultipartForm(maxPostingPDFBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return nil, nil, nil, errFormTooLarge
		}
		return nil, nil, nil, err
	}

	form := &formData{
		values: trimmedValues(r.PostForm),
		errors: formErrors{},
	}

	file, header, err := r.FormFile("posting_pdf")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return form, nil, nil, nil
		}
		return nil, nil, nil, err
	}

	return form, file, header, nil
}

func (s *Server) parseApplicationCreateForm(w http.ResponseWriter, r *http.Request) (*formData, multipart.File, *multipart.FileHeader, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return parsePostingMultipartForm(w, r)
	}
	form, err := parseLimitedForm(w, r, defaultMaxFormBytes)
	return form, nil, nil, err
}

func (s *Server) parseDocumentUploadForm(w http.ResponseWriter, r *http.Request) (*formData, multipart.File, *multipart.FileHeader, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		form, err := parseLimitedForm(w, r, defaultMaxFormBytes)
		return form, nil, nil, err
	}

	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxFormBytes)
	if err := r.ParseMultipartForm(defaultMaxFormBytes); err != nil {
		return nil, nil, nil, err
	}

	form := &formData{
		values: trimmedValues(r.PostForm),
		errors: formErrors{},
	}

	file, header, err := r.FormFile("document_file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return form, nil, nil, nil
		}
		return nil, nil, nil, err
	}

	return form, file, header, nil
}

func (s *Server) saveDocumentPDF(ctx context.Context, document model.Document, file multipart.File, header *multipart.FileHeader) (model.Document, error) {
	if err := validatePostingPDF(file, header); err != nil {
		return model.Document{}, err
	}

	documentID, err := newDocumentUploadID()
	if err != nil {
		return model.Document{}, err
	}
	storagePath := filepath.ToSlash(filepath.Join("documents", "library", documentID+".pdf"))
	destination, err := s.writeDocumentPDF(storagePath, file, header)
	if err != nil {
		return model.Document{}, err
	}

	document.ID = documentID
	document.StoragePath = storagePath
	created, err := s.store.CreateDocument(ctx, document)
	if err != nil {
		_ = os.Remove(destination)
		return model.Document{}, err
	}

	return created, nil
}

func (s *Server) savePostingPDF(ctx context.Context, app model.Application, file multipart.File, header *multipart.FileHeader) (model.ApplicationDocument, error) {
	if err := validatePostingPDF(file, header); err != nil {
		return model.ApplicationDocument{}, err
	}

	documentID, err := newDocumentUploadID()
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	storagePath := filepath.ToSlash(filepath.Join("documents", app.ID, documentID+".pdf"))
	destination, err := s.writeDocumentPDF(storagePath, file, header)
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	removeDestination := true
	defer func() {
		if removeDestination {
			_ = os.Remove(destination)
		}
	}()

	document := model.Document{
		ID:          documentID,
		Name:        app.Company + " - " + app.Role + " job posting",
		Type:        model.DocumentJobPosting,
		StoragePath: storagePath,
		Notes:       "PDF snapshot saved from application detail.",
	}
	attached, err := s.store.AttachDocumentToApplication(ctx, app.ID, document, model.AttachmentJobPosting, "")
	if err != nil {
		return model.ApplicationDocument{}, err
	}

	removeDestination = false
	return attached, nil
}

func (s *Server) writeDocumentPDF(storagePath string, file multipart.File, header *multipart.FileHeader) (string, error) {
	if err := validatePostingPDF(file, header); err != nil {
		return "", err
	}

	destination, err := safeDocumentPath(s.dataDir, storagePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return "", err
	}

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	removeDestination := true
	defer func() {
		_ = out.Close()
		if removeDestination {
			_ = os.Remove(destination)
		}
	}()

	written, err := io.Copy(out, io.LimitReader(file, maxPostingPDFBytes+1))
	if err != nil {
		return "", err
	}
	if written > maxPostingPDFBytes {
		return "", errors.New("document exceeds PDF upload limit")
	}

	removeDestination = false
	return destination, nil
}

func hasPostingPDF(header *multipart.FileHeader) bool {
	return header != nil && strings.TrimSpace(header.Filename) != ""
}

func validatePostingPDF(file multipart.File, header *multipart.FileHeader) error {
	if file == nil || !hasPostingPDF(header) {
		return errors.New("posting PDF is required")
	}
	if header.Size <= 0 || header.Size > maxPostingPDFBytes {
		return errors.New("posting PDF size is invalid")
	}

	headerBytes := make([]byte, 512)
	n, err := file.Read(headerBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if !bytes.HasPrefix(headerBytes[:n], []byte("%PDF-")) {
		return errors.New("posting document is not a PDF")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	return nil
}

func newDocumentUploadID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "doc_" + hex.EncodeToString(buf), nil
}

func safeDocumentPath(dataDir string, storagePath string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("document storage directory is required")
	}
	if strings.TrimSpace(storagePath) == "" {
		return "", errors.New("document storage path is required")
	}
	if filepath.IsAbs(storagePath) {
		return "", errors.New("document storage path must be relative")
	}

	root, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(storagePath)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.New("document storage path escapes data directory")
	}

	return target, nil
}

func downloadFileName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "document"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-_")
	if result == "" {
		return "document"
	}
	return result
}

func (s *Server) applicationDetail(r *http.Request, id string) (model.Application, []model.ApplicationEvent, []model.ApplicationDocument, error) {
	app, err := s.store.GetApplication(r.Context(), id)
	if err != nil {
		return model.Application{}, nil, nil, err
	}

	events, err := s.store.ListApplicationEvents(r.Context(), app.ID)
	if err != nil {
		return model.Application{}, nil, nil, err
	}

	documents, err := s.store.ListApplicationDocuments(r.Context(), app.ID)
	if err != nil {
		return model.Application{}, nil, nil, err
	}

	return app, events, documents, nil
}

func (s *Server) exportSnapshot(r *http.Request) (exportSnapshot, error) {
	applications, err := s.store.ListApplications(r.Context())
	if err != nil {
		return exportSnapshot{}, err
	}
	documents, err := s.store.ListDocuments(r.Context())
	if err != nil {
		return exportSnapshot{}, err
	}
	contacts, err := s.store.ListContacts(r.Context())
	if err != nil {
		return exportSnapshot{}, err
	}

	exportApps := make([]exportApplication, 0, len(applications))
	for _, app := range applications {
		events, err := s.store.ListApplicationEvents(r.Context(), app.ID)
		if err != nil {
			return exportSnapshot{}, err
		}
		exportApps = append(exportApps, exportApplication{
			Application: app,
			Events:      events,
		})
	}

	return exportSnapshot{
		Version:      "1",
		ExportedAt:   time.Now().UTC(),
		Applications: exportApps,
		Documents:    documents,
		Contacts:     contacts,
	}, nil
}

func applicationFormValuesFromForm(form *formData) applicationFormValues {
	return applicationFormValues{
		Company:           form.Value("company"),
		Role:              form.Value("role"),
		Status:            form.Value("status"),
		Priority:          form.Value("priority"),
		Source:            form.Value("source"),
		PostingURL:        form.Value("posting_url"),
		Location:          form.Value("location"),
		NextActionSummary: form.Value("next_action_summary"),
		NextActionDue:     form.Value("next_action_due"),
		Notes:             form.Value("notes"),
	}
}

func applicationFromForm(form *formData) model.Application {
	app := model.Application{
		Company:    form.RequiredString("company", "Company"),
		Role:       form.RequiredString("role", "Role"),
		Status:     model.ApplicationStatus(form.RequiredString("status", "Status")),
		Priority:   model.Priority(form.RequiredString("priority", "Priority")),
		Source:     form.Value("source"),
		PostingURL: form.Value("posting_url"),
		Location:   form.Value("location"),
		Notes:      form.Value("notes"),
		NextAction: model.NextAction{
			Summary: form.Value("next_action_summary"),
		},
	}

	if app.Status != "" && !app.Status.Valid() {
		form.errors.Add("status", "Status must be a valid pipeline state.")
	}
	if app.Priority != "" && !app.Priority.Valid() {
		form.errors.Add("priority", "Priority must be low, normal, or high.")
	}
	if app.PostingURL != "" && !model.ValidHTTPURL(app.PostingURL) {
		form.errors.Add("posting_url", "Original link must be a valid HTTP or HTTPS URL.")
	}
	if due, ok := form.OptionalDate("next_action_due", "Next action due"); ok {
		app.NextAction.Due = &due
	}

	return app
}

func postingFormValuesFromApplication(app model.Application) postingFormValues {
	return postingFormValues{PostingURL: app.PostingURL}
}

func postingFormValuesFromForm(form *formData) postingFormValues {
	return postingFormValues{PostingURL: form.Value("posting_url")}
}

func applicationEventFormValuesFromForm(form *formData) applicationEventFormValues {
	return applicationEventFormValues{
		EventType:  form.Value("event_type"),
		OccurredAt: form.Value("occurred_at"),
		Summary:    form.Value("summary"),
		Notes:      form.Value("notes"),
	}
}

func applicationEventFromForm(form *formData, applicationID string) model.ApplicationEvent {
	event := model.ApplicationEvent{
		ApplicationID: applicationID,
		EventType:     model.EventType(form.RequiredString("event_type", "Event type")),
		Summary:       form.RequiredString("summary", "Summary"),
		Notes:         form.Value("notes"),
	}
	if event.EventType != "" && !event.EventType.Valid() {
		form.errors.Add("event_type", "Event type must be valid.")
	}
	if occurredAt, ok := form.RequiredDate("occurred_at", "Occurred date"); ok {
		event.OccurredAt = occurredAt
	}

	return event
}

func defaultApplicationEventFormValues(now time.Time) applicationEventFormValues {
	return applicationEventFormValues{
		EventType:  string(model.EventNote),
		OccurredAt: now.Format("2006-01-02"),
	}
}

func applicationStatusFormValuesFromApplication(app model.Application) applicationStatusFormValues {
	return applicationStatusFormValues{
		Status:            string(app.Status),
		NextActionSummary: app.NextAction.Summary,
		NextActionDue:     inputDate(app.NextAction.Due),
	}
}

func applicationStatusFormValuesFromForm(form *formData) applicationStatusFormValues {
	return applicationStatusFormValues{
		Status:            form.Value("status"),
		NextActionSummary: form.Value("next_action_summary"),
		NextActionDue:     form.Value("next_action_due"),
	}
}

func applicationStatusUpdateFromForm(form *formData) (model.ApplicationStatus, model.NextAction) {
	status := model.ApplicationStatus(form.RequiredString("status", "Status"))
	if status != "" && !status.Valid() {
		form.errors.Add("status", "Status must be a valid pipeline state.")
	}

	nextAction := model.NextAction{
		Summary: form.Value("next_action_summary"),
	}
	if due, ok := form.OptionalDate("next_action_due", "Next action due"); ok {
		nextAction.Due = &due
	}

	return status, nextAction
}

func applicationStatusOptions() []selectOption {
	return []selectOption{
		{Value: string(model.StatusProspect), Label: "Prospect"},
		{Value: string(model.StatusApplied), Label: "Applied"},
		{Value: string(model.StatusInterviewing), Label: "Interviewing"},
		{Value: string(model.StatusOffer), Label: "Offer"},
		{Value: string(model.StatusAccepted), Label: "Accepted"},
		{Value: string(model.StatusDeclined), Label: "Declined"},
		{Value: string(model.StatusRejected), Label: "Rejected"},
		{Value: string(model.StatusWithdrawn), Label: "Withdrawn"},
		{Value: string(model.StatusArchived), Label: "Archived"},
	}
}

func applicationStatusFilterOptions() []selectOption {
	options := []selectOption{
		{Value: string(model.StatusProspect), Label: "Prospect"},
		{Value: string(model.StatusApplied), Label: "Applied"},
		{Value: string(model.StatusInterviewing), Label: "Interviewing"},
		{Value: string(model.StatusOffer), Label: "Offer"},
		{Value: string(model.StatusAccepted), Label: "Accepted"},
		{Value: closedApplicationStatusFilter, Label: "Closed outcomes"},
	}
	options = append(options,
		selectOption{Value: string(model.StatusDeclined), Label: "Declined"},
		selectOption{Value: string(model.StatusRejected), Label: "Rejected"},
		selectOption{Value: string(model.StatusWithdrawn), Label: "Withdrawn"},
		selectOption{Value: string(model.StatusArchived), Label: "Archived"},
	)
	return options
}

func documentTypeOptions() []selectOption {
	return []selectOption{
		{Value: string(model.DocumentResume), Label: "Resume"},
		{Value: string(model.DocumentCoverLetter), Label: "Cover letter"},
		{Value: string(model.DocumentWorkSample), Label: "Work sample"},
		{Value: string(model.DocumentSnippet), Label: "Snippet"},
		{Value: string(model.DocumentPortfolio), Label: "Portfolio"},
		{Value: string(model.DocumentJobPosting), Label: "Job posting"},
		{Value: string(model.DocumentOther), Label: "Other"},
	}
}

func applicationPriorityOptions() []selectOption {
	return []selectOption{
		{Value: string(model.PriorityLow), Label: "Low"},
		{Value: string(model.PriorityNormal), Label: "Normal"},
		{Value: string(model.PriorityHigh), Label: "High"},
	}
}

func applicationEventTypeOptions() []selectOption {
	return []selectOption{
		{Value: string(model.EventNote), Label: "Note"},
		{Value: string(model.EventApplied), Label: "Applied"},
		{Value: string(model.EventRecruiterScreen), Label: "Recruiter screen"},
		{Value: string(model.EventPhoneScreen), Label: "Phone screen"},
		{Value: string(model.EventInterview), Label: "Interview"},
		{Value: string(model.EventOnsite), Label: "Onsite"},
		{Value: string(model.EventTakeHome), Label: "Take-home"},
		{Value: string(model.EventFollowUp), Label: "Follow-up"},
		{Value: string(model.EventDeadline), Label: "Deadline"},
		{Value: string(model.EventOffer), Label: "Offer"},
		{Value: string(model.EventDecision), Label: "Decision"},
		{Value: string(model.EventOther), Label: "Other"},
	}
}

func statusLabel(status model.ApplicationStatus) string {
	for _, option := range applicationStatusOptions() {
		if option.Value == string(status) {
			return option.Label
		}
	}
	return string(status)
}

func priorityLabel(priority model.Priority) string {
	for _, option := range applicationPriorityOptions() {
		if option.Value == string(priority) {
			return option.Label
		}
	}
	return string(priority)
}

func eventTypeLabel(eventType model.EventType) string {
	for _, option := range applicationEventTypeOptions() {
		if option.Value == string(eventType) {
			return option.Label
		}
	}
	return string(eventType)
}

func documentTypeLabel(documentType model.DocumentType) string {
	for _, option := range documentTypeOptions() {
		if option.Value == string(documentType) {
			return option.Label
		}
	}
	return string(documentType)
}

func documentDownloadURL(document model.Document) string {
	if strings.HasPrefix(filepath.ToSlash(document.StoragePath), "documents/") {
		return "/documents/" + document.ID + "/download"
	}
	return ""
}

func documentViewURL(document model.Document) string {
	if documentDownloadURL(document) != "" {
		return "/documents/" + document.ID
	}
	return ""
}

func documentReferenceLabel(document model.Document) string {
	if documentDownloadURL(document) != "" {
		return "Stored PDF"
	}
	return "Reference recorded"
}

func documentFormValuesFromForm(form *formData) documentFormValues {
	return documentFormValues{
		Name:  form.Value("name"),
		Type:  form.Value("document_type"),
		Notes: form.Value("notes"),
	}
}

func documentFromUploadForm(form *formData) model.Document {
	document := model.Document{
		Name:  form.RequiredString("name", "Name"),
		Type:  model.DocumentType(form.RequiredString("document_type", "Document type")),
		Notes: form.Value("notes"),
	}
	if document.Type != "" && !document.Type.Valid() {
		form.errors.Add("document_type", "Document type must be valid.")
	}
	return document
}

func contactFormValuesFromForm(form *formData) contactFormValues {
	return contactFormValues{
		Name:         form.Value("name"),
		Organization: form.Value("organization"),
		Role:         form.Value("role"),
		Email:        form.Value("email"),
		Phone:        form.Value("phone"),
		Location:     form.Value("location"),
		Notes:        form.Value("notes"),
	}
}

func contactFromForm(form *formData) model.Contact {
	return model.Contact{
		Name:         form.RequiredString("name", "Name"),
		Organization: form.Value("organization"),
		Role:         form.Value("role"),
		Email:        form.Value("email"),
		Phone:        form.Value("phone"),
		Location:     form.Value("location"),
		Notes:        form.Value("notes"),
	}
}

func inputDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func optionalDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format("Jan 2, 2006")
}

func longDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("Jan 2, 2006 at 3:04 PM")
}

func shortDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	now := time.Now()
	if sameDay(value, now) {
		return "Today"
	}
	if sameDay(value, now.AddDate(0, 0, -1)) {
		return "Yesterday"
	}
	return value.Format("Jan 2")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}

func startOfDay(value time.Time) time.Time {
	local := value.Local()
	year, month, day := local.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, local.Location())
}

func endOfDay(value time.Time) time.Time {
	local := value.Local()
	year, month, day := local.Date()
	return time.Date(year, month, day, 23, 59, 59, int(time.Second-time.Nanosecond), local.Location())
}

func startOfWeek(value time.Time) time.Time {
	dayStart := startOfDay(value)
	weekdayOffset := (int(dayStart.Weekday()) + 6) % 7
	return dayStart.AddDate(0, 0, -weekdayOffset)
}

func inTimeWindow(value, start, end time.Time) bool {
	return !value.IsZero() && !value.Before(start) && !value.After(end)
}

func isActiveStatus(status model.ApplicationStatus) bool {
	switch status {
	case model.StatusAccepted, model.StatusDeclined, model.StatusRejected, model.StatusWithdrawn, model.StatusArchived:
		return false
	default:
		return true
	}
}

func filterApplications(applications []model.Application, query string, status string) []model.Application {
	query = strings.ToLower(strings.TrimSpace(query))
	status = strings.TrimSpace(status)
	statusValue := model.ApplicationStatus(status)
	statusValid := statusValue.Valid()
	closedStatus := status == closedApplicationStatusFilter
	if query == "" && !statusValid && !closedStatus {
		return applications
	}

	filtered := make([]model.Application, 0, len(applications))
	for _, app := range applications {
		if statusValid && app.Status != statusValue {
			continue
		}
		if closedStatus && isActiveStatus(app.Status) {
			continue
		}
		if query != "" && !applicationMatchesQuery(app, query) {
			continue
		}
		filtered = append(filtered, app)
	}
	return filtered
}

func applicationMatchesQuery(app model.Application, query string) bool {
	values := []string{
		app.Company,
		app.Role,
		app.Source,
		app.Location,
		app.Notes,
		app.NextAction.Summary,
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func followUpItems(applications []model.Application) []followUpItem {
	items := make([]followUpItem, 0)
	for _, app := range applications {
		if app.NextAction.Summary == "" {
			continue
		}
		items = append(items, followUpItem{
			ID:          app.ID,
			Company:     app.Company,
			Role:        app.Role,
			Status:      app.Status,
			StatusLabel: statusLabel(app.Status),
			Summary:     app.NextAction.Summary,
			Due:         optionalDate(app.NextAction.Due),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := applicationsNextActionDue(applications, items[i].ID)
		right := applicationsNextActionDue(applications, items[j].ID)
		if left == nil && right == nil {
			return items[i].Company < items[j].Company
		}
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.Before(*right)
	})
	return items
}

func applicationsNextActionDue(applications []model.Application, id string) *time.Time {
	for _, app := range applications {
		if app.ID == id {
			return app.NextAction.Due
		}
	}
	return nil
}

func nextActionText(app model.Application) string {
	if app.NextAction.Summary == "" {
		return ""
	}
	return app.NextAction.Summary + " for " + app.Company + "."
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

func handleFormParseError(w http.ResponseWriter, err error) {
	if errors.Is(err, errFormTooLarge) {
		http.Error(w, "form body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid form body", http.StatusBadRequest)
}

func setSecurityHeaders(h http.Header) {
	h.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
}

func serverError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("internal server error",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
