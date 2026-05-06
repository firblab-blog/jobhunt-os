package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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
)

type Server struct {
	mux       *http.ServeMux
	templates *template.Template
	store     store.ApplicationStore
	dataDir   string
}

type Options struct {
	DataDir string
}

type dashboardData struct {
	Applications []dashboardApplication
	Metrics      []dashboardMetric
	NextActions  []dashboardNextAction
}

type dashboardApplication struct {
	model.Application
	Status    string
	StatusKey string
	Updated   string
}

type dashboardMetric struct {
	Label string
	Value string
}

type dashboardNextAction struct {
	Text string
}

type applicationsIndexData struct {
	Applications  []applicationListItem
	Query         string
	Status        string
	StatusOptions []selectOption
}

type applicationListItem struct {
	model.Application
	StatusLabel   string
	PriorityLabel string
	Updated       string
	NextActionDue string
}

type applicationsFormData struct {
	CSRFToken       template.HTML
	Values          applicationFormValues
	Errors          formErrors
	StatusOptions   []selectOption
	PriorityOptions []selectOption
}

type applicationShowData struct {
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
}

type selectOption struct {
	Value string
	Label string
}

type documentsIndexData struct {
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
	DownloadURL    string
	ReferenceLabel string
}

type documentFormValues struct {
	Name        string
	Type        string
	StoragePath string
	Notes       string
}

type contactsIndexData struct {
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

type followUpsIndexData struct {
	Items []followUpItem
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

type backupData struct {
	GeneratedAt string
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
	templates := template.Must(template.ParseFS(jobhuntos.Assets, "web/templates/*.html"))

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
	s.mux.HandleFunc("GET /documents/{id}/download", s.documentsDownload)
	s.mux.HandleFunc("GET /contacts", s.contactsIndex)
	s.mux.HandleFunc("POST /contacts", s.contactsCreate)
	s.mux.HandleFunc("GET /follow-ups", s.followUpsIndex)
	s.mux.HandleFunc("GET /backup", s.backupIndex)
	s.mux.HandleFunc("GET /export.json", s.exportJSON)

	return s
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
	data := s.dashboard(r)

	var body bytes.Buffer
	if err := s.templates.ExecuteTemplate(&body, "home.html", data); err != nil {
		serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body.Bytes())
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
	items := make([]dashboardApplication, 0, min(len(applications), 5))
	nextActions := make([]dashboardNextAction, 0)
	now := time.Now()

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
				nextActions = append(nextActions, dashboardNextAction{
					Text: nextActionText(app),
				})
			}
		}
		if len(items) < 5 {
			items = append(items, dashboardApplication{
				Application: app,
				Status:      statusLabel(app.Status),
				StatusKey:   string(app.Status),
				Updated:     shortDate(app.UpdatedAt),
			})
		}
	}

	documentCount, err := s.store.CountDocuments(r.Context())
	if err != nil {
		slog.Error("count dashboard documents", "error", err)
		documentCount = 0
	}

	return dashboardData{
		Metrics: []dashboardMetric{
			{Label: "Active applications", Value: itoa(active)},
			{Label: "Need follow-up", Value: itoa(needFollowUp)},
			{Label: "Interview loops", Value: itoa(interviews)},
			{Label: "Documents", Value: itoa(documentCount)},
		},
		Applications: items,
		NextActions:  nextActions,
	}
}

func fallbackDashboardData() dashboardData {
	return dashboardData{
		Metrics: []dashboardMetric{
			{Label: "Active applications", Value: "12"},
			{Label: "Need follow-up", Value: "3"},
			{Label: "Interview loops", Value: "2"},
			{Label: "Draft documents", Value: "5"},
		},
		Applications: []dashboardApplication{
			{
				Application: model.Application{
					Company:    "Northstar Systems",
					Role:       "Senior Platform Engineer",
					Status:     model.StatusInterviewing,
					NextAction: model.NextAction{Summary: "Prep system design notes"},
				},
				Status:    "Interviewing",
				StatusKey: "interviewing",
				Updated:   "Today",
			},
			{
				Application: model.Application{
					Company:    "Atlas Cloud",
					Role:       "Staff DevOps Engineer",
					Status:     model.StatusApplied,
					NextAction: model.NextAction{Summary: "Follow up with recruiter"},
				},
				Status:    "Applied",
				StatusKey: "applied",
				Updated:   "Yesterday",
			},
			{
				Application: model.Application{
					Company:    "Signal Works",
					Role:       "Infrastructure Lead",
					Status:     model.StatusProspect,
					NextAction: model.NextAction{Summary: "Tailor cover letter"},
				},
				Status:    "Drafting",
				StatusKey: "drafting",
				Updated:   "Apr 30",
			},
		},
		NextActions: []dashboardNextAction{
			{Text: "Prep system design notes for Northstar Systems."},
			{Text: "Send recruiter follow-up for Atlas Cloud."},
			{Text: "Attach tailored cover letter to Signal Works draft."},
		},
	}
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
	filtered := filterApplications(applications, query, model.ApplicationStatus(status))

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
		Applications:  items,
		Query:         query,
		Status:        status,
		StatusOptions: applicationStatusOptions(),
	})
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

	form, err := parseLimitedForm(w, r, defaultMaxFormBytes)
	if err != nil {
		handleFormParseError(w, err)
		return
	}
	if err := verifyCSRF(r, time.Now()); err != nil {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}

	values := documentFormValuesFromForm(form)
	document := documentFromForm(form)
	if !form.errors.Any() {
		if _, err := s.store.CreateDocument(r.Context(), document); err != nil {
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

func (s *Server) followUpsIndex(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		serverError(w, r, errors.New("application store is not configured"))
		return
	}

	applications, err := s.store.ListApplications(r.Context())
	if err != nil {
		serverError(w, r, err)
		return
	}

	items := followUpItems(applications)
	s.render(w, r, "followups_index.html", followUpsIndexData{Items: items})
}

func (s *Server) backupIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "backup.html", backupData{GeneratedAt: time.Now().Format(time.RFC1123)})
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

	values := applicationFormValuesFromForm(form)
	app := applicationFromForm(form)
	if !form.errors.Any() {
		created, err := s.store.CreateApplication(r.Context(), app)
		if err != nil {
			form.errors.Add("form", "Could not save application. Please check the fields and try again.")
			slog.Error("create application", "error", err)
		} else {
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
			DownloadURL:    documentDownloadURL(document),
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
	if err := s.templates.ExecuteTemplate(&body, name, data); err != nil {
		serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
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

func (s *Server) savePostingPDF(ctx context.Context, app model.Application, file multipart.File, header *multipart.FileHeader) (model.ApplicationDocument, error) {
	if file == nil || header == nil || strings.TrimSpace(header.Filename) == "" {
		return model.ApplicationDocument{}, errors.New("posting PDF is required")
	}
	if header.Size <= 0 || header.Size > maxPostingPDFBytes {
		return model.ApplicationDocument{}, errors.New("posting PDF size is invalid")
	}

	documentID, err := newDocumentUploadID()
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	storagePath := filepath.ToSlash(filepath.Join("documents", app.ID, documentID+".pdf"))
	destination, err := safeDocumentPath(s.dataDir, storagePath)
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return model.ApplicationDocument{}, err
	}

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	removeDestination := true
	defer func() {
		_ = out.Close()
		if removeDestination {
			_ = os.Remove(destination)
		}
	}()

	headerBytes := make([]byte, 512)
	n, err := file.Read(headerBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return model.ApplicationDocument{}, err
	}
	if !bytes.HasPrefix(headerBytes[:n], []byte("%PDF-")) {
		return model.ApplicationDocument{}, errors.New("posting document is not a PDF")
	}
	if _, err := out.Write(headerBytes[:n]); err != nil {
		return model.ApplicationDocument{}, err
	}
	if _, err := io.Copy(out, io.LimitReader(file, maxPostingPDFBytes-int64(n)+1)); err != nil {
		return model.ApplicationDocument{}, err
	}

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

func documentReferenceLabel(document model.Document) string {
	if documentDownloadURL(document) != "" {
		return "Stored PDF"
	}
	return "Reference recorded"
}

func documentFormValuesFromForm(form *formData) documentFormValues {
	return documentFormValues{
		Name:        form.Value("name"),
		Type:        form.Value("document_type"),
		StoragePath: form.Value("storage_path"),
		Notes:       form.Value("notes"),
	}
}

func documentFromForm(form *formData) model.Document {
	document := model.Document{
		Name:        form.RequiredString("name", "Name"),
		Type:        model.DocumentType(form.RequiredString("document_type", "Document type")),
		StoragePath: form.RequiredString("storage_path", "File path or reference"),
		Notes:       form.Value("notes"),
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

func endOfDay(value time.Time) time.Time {
	local := value.Local()
	year, month, day := local.Date()
	return time.Date(year, month, day, 23, 59, 59, int(time.Second-time.Nanosecond), local.Location())
}

func isActiveStatus(status model.ApplicationStatus) bool {
	switch status {
	case model.StatusAccepted, model.StatusDeclined, model.StatusRejected, model.StatusWithdrawn, model.StatusArchived:
		return false
	default:
		return true
	}
}

func filterApplications(applications []model.Application, query string, status model.ApplicationStatus) []model.Application {
	query = strings.ToLower(strings.TrimSpace(query))
	statusValid := status != "" && status.Valid()
	if query == "" && !statusValid {
		return applications
	}

	filtered := make([]model.Application, 0, len(applications))
	for _, app := range applications {
		if statusValid && app.Status != status {
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
