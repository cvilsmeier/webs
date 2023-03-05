package main

import (
	"log"
	"net/http"
	"strconv"
	"time"
	"webs"
)

func main() {
	log.Printf("webs sample - press Ctrl-C to abort")
	http.Handle("/static/", http.FileServer(http.Dir("assets")))
	templateLoader, err := webs.NewDefaultTemplateLoader("assets/templates/*.html", nil, true)
	if err != nil {
		log.Fatal(err)
	}
	server := NewServer(templateLoader)
	http.Handle("/", server)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

// Server is a http.Handler that serves incoming HTTP requests.
type Server struct {
	responseRenderer *webs.ResponseRenderer
	sessionStore     webs.SessionStore
}

func NewServer(templateLoader webs.TemplateLoader) *Server {
	responseRenderer := webs.NewResponseRenderer(templateLoader)
	sessionStore := webs.NewMemorySessionStore()
	return &Server{responseRenderer, sessionStore}
}

// ServeHTTP implements http.Handler and dispatches requests to serv methods.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	// wrap http.Request in webs.Request
	req := webs.NewRequest(r)
	// call serv() method based on path
	var res webs.Response
	switch r.URL.Path {
	case "/":
		res = s.servIndex(req)
	case "/say":
		res = s.servSay(req)
	case "/add":
		res = s.servAdd(req)
	}
	// render response (or 404)
	s.responseRenderer.Render(w, r, res)
	// log request
	latency := time.Since(start)
	log.Printf("[webs] %-4s %-20s  - %s", r.Method, r.URL.Path, latency)
}

const (
	sessionIdCookieName = "SAMPLE_SESSION_ID"
)

func (s *Server) servIndex(req webs.Request) webs.Response {
	sessionId := req.CookieValue(sessionIdCookieName, "")
	session := s.sessionStore.Find(sessionId)
	if req.IsPost() {
		if session.IsZero() {
			session = webs.NewSession()
		}
		session = session.WithValue("name", req.PostForm("name"))
		if err := s.sessionStore.Save(session); err != nil {
			return webs.NewStatusInternalServerErrorResponse("cannot save session: %s", err)
		}
		res := webs.NewRedirectResponse("/")
		if sessionId != session.Id() {
			res = res.WithCookie(sessionIdCookieName, session.Id(), 24*time.Hour)
		}
		return res
	}
	name := session.Get("name", "")
	res := webs.NewTemplateResponse("index.html", webs.M{
		"name": name,
	})
	return res
}

func (s *Server) servSay(req webs.Request) webs.Response {
	return webs.NewTemplateResponse("say.html", webs.M{
		"message": req.Query("message"),
	})
}

func (s *Server) servAdd(req webs.Request) webs.Response {
	var value1 int
	var value2 int
	var result int
	if req.IsPost() {
		value1, _ = strconv.Atoi(req.PostForm("value1")) // ignore parse error
		value2, _ = strconv.Atoi(req.PostForm("value2")) // ignore parse error
		result = value1 + value2
	}
	return webs.NewTemplateResponse("add.html", webs.M{
		"value1": value1,
		"value2": value2,
		"result": result,
	})
}
