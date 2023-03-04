package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
)

func main() {
	log.Printf("webs sample")
	http.Handle("/static/", http.FileServer(http.Dir("assets")))
	server, err := NewServer()
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", server)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	responseRenderer *ResponseRenderer
	sessionStore     SessionStore
}

func NewServer() (*Server, error) {
	funcs := template.FuncMap{}
	templateLoader, err := NewDefaultTemplateLoader("assets/templates/*.html", funcs, true)
	if err != nil {
		return nil, err
	}
	responseRenderer := NewResponseRenderer(templateLoader)
	sessionStore := NewMemorySessionStore()
	return &Server{responseRenderer, sessionStore}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	path := r.URL.Path
	start := time.Now()
	req := NewRequest(r)
	var res Response
	disp := map[string]func(req Request) Response{
		"":     s.servIndex,
		"/":    s.servIndex,
		"/say": s.servSay,
	}
	fn, ok := disp[path]
	if ok {
		res = fn(req)
	}
	s.responseRenderer.Render(w, r, res)
	// log request
	latency := time.Since(start)
	log.Printf("[webs] %-4s %-20s  - %s", method, path, latency)
}

const (
	sessionIdCookieName = "SAMPLE_SESSION_ID"
)

func (s *Server) servIndex(req Request) Response {
	sessionId := req.CookieValue(sessionIdCookieName, "")
	session := s.sessionStore.Find(sessionId)
	if req.IsPost() {
		if session.IsZero() {
			session = NewSession()
		}
		session = session.WithValue("name", req.PostForm("name"))
		if err := s.sessionStore.Save(session); err != nil {
			return NewStatusInternalServerErrorResponse("cannot save session: %s", err)
		}
		res := NewRedirectResponse("/")
		if sessionId != session.Id() {
			res = res.WithCookie(sessionIdCookieName, session.Id(), 24*time.Hour)
		}
		return res
	}
	name := session.Get("name", "")
	res := NewTemplateResponse("index.html", M{
		"name": name,
	})
	return res
}

func (s *Server) servSay(req Request) Response {
	return NewTemplateResponse("say.html", M{
		"message": req.Query("message"),
	})
}
