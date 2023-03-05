package webs

// ----------------------------------------------------------------------------
//
// This is webs, a collection of utilities for writing small web apps in Go.
//
// version 2023-03-04
//
// ----------------------------------------------------------------------------

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

// Request represents a HTTP request.
type Request interface {
	// IsPost returns true if this is a POST request.
	IsPost() bool
	// Query returns first named query parameter, or empty string if not found.
	Query(name string) string
	// PostForm returns first named form post parameter, or empty string if not found.
	PostForm(name string) string
	// FormFile returns the first file for the provided form key.
	FormFile(name string) (FormFile, error)
	// CookieValue returns the named cookie, or empty string if not found.
	CookieValue(name, defValue string) string
}

// FormFile represents a HTTP file upload.
type FormFile interface {
	// Filename returns the original filename. Since this is set by the client, you cannot trust it.
	Filename() string
	// Size returns the size in bytes.
	Size() int64
	// Read reads uploaded data.
	Read(p []byte) (int, error)
	// Close closes it and must be called whether or not Read() was called before.
	Close() error
}

// requestImpl is a Request that wraps a *http.Request.
type requestImpl struct {
	r *http.Request
}

var _ Request = (*requestImpl)(nil) // *requestImpl implements Request

func NewRequest(r *http.Request) Request {
	return &requestImpl{r}
}

func (r *requestImpl) IsPost() bool {
	return r.r.Method == "POST"
}

func (r *requestImpl) Query(name string) string {
	valuesMap := r.r.URL.Query()
	values := valuesMap[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (r *requestImpl) PostForm(name string) string {
	return r.r.PostFormValue(name)
}

func (r *requestImpl) FormFile(name string) (FormFile, error) {
	fil, hdr, err := r.r.FormFile(name)
	if err != nil {
		return nil, err
	}
	return &formFileImpl{fil, hdr}, nil
}

func (r *requestImpl) CookieValue(name, defValue string) string {
	c, err := r.r.Cookie(name)
	if err != nil {
		return defValue
	}
	return c.Value
}

// A formFileImpl is a FormFile that wraps a multipart.File
type formFileImpl struct {
	mf multipart.File
	mh *multipart.FileHeader
}

func (f *formFileImpl) Filename() string {
	return f.mh.Filename
}

func (f *formFileImpl) Size() int64 {
	return f.mh.Size
}

func (f *formFileImpl) ReadAll() ([]byte, error) {
	return io.ReadAll(f.mf)
}

func (f *formFileImpl) Read(p []byte) (int, error) {
	return f.mf.Read(p)
}

func (f *formFileImpl) Close() error {
	return f.mf.Close()
}

// Response holds response data.
type Response struct {
	Type               ResponseType
	TemplateName       string            // for Type TemplateResponse
	TemplateData       M                 // for Type TemplateResponse
	JsonData           any               // for Type JsonResponse
	FileName           string            // for Type FileResponse
	FileType           string            // for Type FileResponse
	FileDisposition    string            // for Type FileResponse
	ContentData        []byte            // for Type ContentResponse
	ContentType        string            // for Type ContentResponse
	ContentDisposition string            // for Type ContentResponse
	RedirectLocation   string            // for Type RedirectResponse
	StatusCode         int               // for Type StatusResponse
	StatusText         string            // for Type StatusResponse
	Cookies            []*http.Cookie    // for all response types
	Headers            map[string]string // for all response types
}

type ResponseType int

const (
	TemplateResponse ResponseType = iota + 1
	JsonResponse
	FileResponse
	ContentResponse
	RedirectResponse
	StatusResponse
)

// NewTemplateResponse renders a template.
func NewTemplateResponse(name string, data M) Response {
	return Response{Type: TemplateResponse, TemplateName: name, TemplateData: data}
}

// NewJsonResponse writes JSON data.
func NewJsonResponse(data any) Response {
	return Response{Type: JsonResponse, JsonData: data}
}

// NewFileResponse writes a file.
func NewFileResponse(name string, ctype, disposition string) Response {
	return Response{Type: FileResponse, FileName: name, FileType: ctype, FileDisposition: disposition}
}

// NewContentResponse writes arbitrary data.
func NewContentResponse(data []byte, ctype string, disposition string) Response {
	return Response{Type: ContentResponse, ContentData: data, ContentType: ctype, ContentDisposition: disposition}
}

// NewRedirectResponse writes a redirect response.
func NewRedirectResponse(location string) Response {
	return Response{Type: RedirectResponse, RedirectLocation: location}
}

// NewStatusResponse writes a status response.
func NewStatusResponse(code int, text string) Response {
	return Response{Type: StatusResponse, StatusCode: code, StatusText: text}
}

// NewStatusNotFoundResponse writes a status 404 response.
func NewStatusNotFoundResponse(format string, a ...any) Response {
	return NewStatusResponse(404, fmt.Sprintf(format, a...))
}

// NewStatusInternalServerErrorResponse writes a status 500 response.
func NewStatusInternalServerErrorResponse(format string, a ...any) Response {
	return NewStatusResponse(500, fmt.Sprintf(format, a...))
}

// WithCookie adds a cookie to the HTTP response.
//   - maxAge = 0 means no 'Max-Age' attribute specified.
//   - maxAge < 0 means delete cookie now, equivalently 'Max-Age: 0'
//   - maxAge > 0 means Max-Age attribute present
func (r Response) WithCookie(name, value string, maxAge time.Duration) Response {
	maxAgeSec := int(maxAge / time.Second)
	if maxAge < 0 {
		maxAgeSec = -1
	}
	r.Cookies = append(r.Cookies, &http.Cookie{
		Name:   name,
		Value:  value,
		MaxAge: maxAgeSec,
	})
	return r
}

// WithDeleteCookie is the same as WithCookie(name, "", -1).
func (r Response) WithDeleteCookie(name string) Response {
	return r.WithCookie(name, "", -1)
}

// WithHeader adds a header to the response.
func (r Response) WithHeader(key, value string) Response {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
	return r
}

// A TemplateLoader loads templates.
type TemplateLoader interface {
	Load() (*template.Template, error)
}

// A DefaultTemplateLoader is a TemplateLoader that loads templates from files.
// It uses template.ParseGlob() internally.
type DefaultTemplateLoader struct {
	templatesPattern string
	funcs            template.FuncMap
	cachedTemplate   *template.Template
}

var _ TemplateLoader = (*DefaultTemplateLoader)(nil)

func NewDefaultTemplateLoader(templatesPattern string, funcs template.FuncMap, reload bool) (TemplateLoader, error) {
	loader := &DefaultTemplateLoader{templatesPattern, funcs, nil}
	if !reload {
		templ, err := loader.parse()
		if err != nil {
			return nil, err
		}
		loader.cachedTemplate = templ
	}
	return loader, nil
}

func (l *DefaultTemplateLoader) Load() (*template.Template, error) {
	if l.cachedTemplate != nil {
		return l.cachedTemplate, nil
	}
	return l.parse()
}

func (l *DefaultTemplateLoader) parse() (*template.Template, error) {
	tpl := template.New("")
	tpl.Funcs(l.funcs)
	_, err := tpl.ParseGlob(l.templatesPattern)
	if err != nil {
		return nil, fmt.Errorf("cannot parse templates: %w", err)
	}
	return tpl, nil
}

// A NullTemplateLoader is a TemplateLoader that does nothing.
// Useful for pure REST apps that do not render HTML templates.
type NullTemplateLoader struct {
	err error
}

var _ TemplateLoader = (*NullTemplateLoader)(nil)

func NewNullTemplateLoader() TemplateLoader {
	return &NullTemplateLoader{
		errors.New("NullTemplateLoader cannot Load() anything"),
	}
}

func (l *NullTemplateLoader) Load() (*template.Template, error) {
	return nil, l.err
}

// A ResponseRenderer renders responses.
type ResponseRenderer struct {
	templateLoader TemplateLoader
}

func NewResponseRenderer(templateLoader TemplateLoader) *ResponseRenderer {
	if templateLoader == nil {
		panic("no templateLoader")
	}
	return &ResponseRenderer{templateLoader}
}

// Render renders a response
func (r *ResponseRenderer) Render(w http.ResponseWriter, req *http.Request, response Response) {
	// cookies and headers
	for _, c := range response.Cookies {
		http.SetCookie(w, c)
	}
	for key, value := range response.Headers {
		w.Header().Add(key, value)
	}
	// content
	switch response.Type {
	case TemplateResponse:
		tpl, err := r.templateLoader.Load()
		if err != nil {
			errMsg := fmt.Sprintf("cannot load templates: %s", err)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		err = tpl.ExecuteTemplate(w, response.TemplateName, response.TemplateData)
		if err != nil {
			errMsg := fmt.Sprintf("cannot render %s: %s", response.TemplateName, err)
			io.WriteString(w, errMsg)
		}
	case JsonResponse:
		data, err := json.Marshal(response.JsonData)
		if err != nil {
			errMsg := fmt.Sprintf("cannot marshal json: %s", err)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		w.Write(data)
	case FileResponse:
		if response.FileType != "" {
			w.Header().Set("Content-Type", response.FileType)
		}
		if response.FileDisposition != "" {
			w.Header().Set("Content-Disposition", response.FileDisposition)
		}
		http.ServeFile(w, req, response.FileName)
	case ContentResponse:
		if response.ContentType != "" {
			w.Header().Set("Content-Type", response.ContentType)
		}
		if response.ContentDisposition != "" {
			w.Header().Set("Content-Disposition", response.ContentDisposition)
		}
		w.Write(response.ContentData)
	case RedirectResponse:
		http.Redirect(w, req, response.RedirectLocation, http.StatusSeeOther)
	case StatusResponse:
		w.WriteHeader(response.StatusCode)
		io.WriteString(w, response.StatusText)
	default:
		http.NotFound(w, req)
	}
}

// M holds template data
type M map[string]any

// PageParams are used by templates to carry data from one template
// to another. Can be used when including templates with
// {{template "name"}}.
type PageParams map[string]any

func (p PageParams) Set(key string, value any) string {
	p[key] = value
	return ""
}

func (p PageParams) Is(key string, value any) bool {
	return p[key] == value
}

func (p PageParams) Has(key string) bool {
	_, ok := p[key]
	return ok
}

func (p PageParams) Get(key string) any {
	return p[key]
}

// Session is a user session.
type Session struct {
	id     string
	values map[string]string
}

// NewSession creates a new session with a unique random id.
// Before Go 1.20, you must call rand.Seed() before calling NewSession.
func NewSession() Session {
	const chars = "0123456789abcdef"
	buf := make([]byte, 32)
	for i := range buf {
		n := rand.Intn(16)
		x := chars[n]
		buf[i] = x
	}
	return Session{string(buf), make(map[string]string)}
}

// IsZero returns true if s has an empty id.
func (s Session) IsZero() bool { return s.id == "" }

// Id returns the session id.
func (s Session) Id() string { return s.id }

func (s Session) WithValue(key, value string) Session {
	newValues := make(map[string]string, len(s.values))
	for k, v := range s.values {
		newValues[k] = v
	}
	newValues[key] = value
	s.values = newValues
	return s
}

func (s Session) Get(key, defValue string) string {
	if s.values == nil {
		return defValue
	}
	v, ok := s.values[key]
	if !ok {
		return defValue
	}
	return v
}

func (s Session) Keys() []string {
	var keys []string
	for k := range s.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SessionStore stores session
type SessionStore interface {
	Save(session Session) error
	Delete(id string) error
	Find(id string) Session
	FindAll() []Session
}

// FileSessionStore stores sessions in a json file.
type FileSessionStore struct {
	filename string
	mu       sync.Mutex
	sessions map[string]Session
}

var _ SessionStore = (*FileSessionStore)(nil)

func NewFileSessionStore(filename string) (SessionStore, error) {
	store := &FileSessionStore{
		filename: filename,
		sessions: make(map[string]Session),
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	var valuesMap map[string]map[string]string
	err = json.Unmarshal(data, &valuesMap)
	if err != nil {
		return store, err
	}
	sessions := make(map[string]Session)
	for id, values := range valuesMap {
		sessions[id] = Session{
			id:     id,
			values: values,
		}
	}
	store.sessions = sessions
	return store, nil
}

func (st *FileSessionStore) Save(session Session) error {
	if session.IsZero() {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.sessions[session.id] = session
	return st.save()
}

func (st *FileSessionStore) Delete(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	_, found := st.sessions[id]
	if !found {
		return nil
	}
	delete(st.sessions, id)
	return st.save()
}

func (st *FileSessionStore) Find(id string) Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.sessions[id]
}

func (st *FileSessionStore) FindAll() []Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	var tmp []Session
	for _, session := range st.sessions {
		tmp = append(tmp, session)
	}
	sort.Slice(tmp, func(i, j int) bool {
		a := tmp[i]
		b := tmp[j]
		return a.id < b.id
	})
	return tmp
}

func (st *FileSessionStore) save() error {
	jsessions := make(map[string]map[string]string)
	for id, s := range st.sessions {
		jsessions[id] = s.values
	}
	data, err := json.Marshal(jsessions)
	if err != nil {
		return err
	}
	err = os.WriteFile(st.filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// MemorySessionStore stores sessions in memory.
type MemorySessionStore struct {
	mu       sync.Mutex
	sessions map[string]Session
}

var _ SessionStore = (*MemorySessionStore)(nil)

func NewMemorySessionStore() SessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]Session),
	}
}

func (st *MemorySessionStore) Save(session Session) error {
	if session.IsZero() {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.sessions[session.id] = session
	return nil
}

func (st *MemorySessionStore) Delete(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.sessions, id)
	return nil
}

func (st *MemorySessionStore) Find(id string) Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.sessions[id]
}

func (st *MemorySessionStore) FindAll() []Session {
	st.mu.Lock()
	defer st.mu.Unlock()
	var tmp []Session
	for _, session := range st.sessions {
		tmp = append(tmp, session)
	}
	sort.Slice(tmp, func(i, j int) bool {
		a := tmp[i]
		b := tmp[j]
		return a.id < b.id
	})
	return tmp
}
