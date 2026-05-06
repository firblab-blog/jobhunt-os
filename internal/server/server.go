package server

import (
	"html/template"
	"io/fs"
	"net/http"

	jobhuntos "gitlab.home.firblab.org/applications/jobhunt-os"
)

type Server struct {
	mux       *http.ServeMux
	templates *template.Template
}

type DashboardData struct {
	Applications []ApplicationSummary
	Metrics      []Metric
}

type ApplicationSummary struct {
	Company    string
	Role       string
	Status     string
	NextAction string
	Updated    string
}

type Metric struct {
	Label string
	Value string
}

func New() http.Handler {
	templates := template.Must(template.ParseFS(jobhuntos.Assets, "web/templates/*.html"))

	s := &Server{
		mux:       http.NewServeMux(),
		templates: templates,
	}

	staticFiles, err := fs.Sub(jobhuntos.Assets, "web/static")
	if err != nil {
		panic(err)
	}

	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /", s.home)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) home(w http.ResponseWriter, _ *http.Request) {
	data := DashboardData{
		Metrics: []Metric{
			{Label: "Active applications", Value: "12"},
			{Label: "Need follow-up", Value: "3"},
			{Label: "Interview loops", Value: "2"},
			{Label: "Draft documents", Value: "5"},
		},
		Applications: []ApplicationSummary{
			{
				Company:    "Northstar Systems",
				Role:       "Senior Platform Engineer",
				Status:     "Interviewing",
				NextAction: "Prep system design notes",
				Updated:    "Today",
			},
			{
				Company:    "Atlas Cloud",
				Role:       "Staff DevOps Engineer",
				Status:     "Applied",
				NextAction: "Follow up with recruiter",
				Updated:    "Yesterday",
			},
			{
				Company:    "Signal Works",
				Role:       "Infrastructure Lead",
				Status:     "Drafting",
				NextAction: "Tailor cover letter",
				Updated:    "Apr 30",
			},
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "home.html", data); err != nil {
		http.Error(w, "render dashboard", http.StatusInternalServerError)
	}
}
