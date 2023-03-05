package main

import (
	"fmt"
	"testing"
	"webs"
)

func TestServSay(t *testing.T) {
	s := NewServer(webs.NewNullTemplateLoader())
	// test "GET /say?message=hello"
	{
		req := &fakeRequest{
			query: map[string]string{
				"message": "hello",
			},
		}
		res := s.servSay(req)
		// response must be TemplateResponse
		if res.Type != webs.TemplateResponse {
			t.Fatalf("wrong Type %v", res.Type)
		}
		// template must be "say.html"
		if res.TemplateName != "say.html" {
			t.Fatalf("wrong TemplateName %v", res.TemplateName)
		}
		// template data must have exactly one entry
		if len(res.TemplateData) != 1 {
			t.Fatalf("wrong TemplateData size in %v size", res.TemplateData)
		}
		// template data must have "message" entry with value "hello"
		if len(res.TemplateData) != 1 {
			t.Fatalf("wrong TemplateData size in %v size", res.TemplateData)
		}
	}
	// test "GET /add"
	{
		req := &fakeRequest{}
		res := s.servAdd(req)
		assertEq(t, webs.TemplateResponse, res.Type)
		assertEq(t, "add.html", res.TemplateName)
		assertEq(t, 3, len(res.TemplateData))
		assertEq(t, 0, res.TemplateData["value1"])
		assertEq(t, 0, res.TemplateData["value2"])
		assertEq(t, 0, res.TemplateData["result"])
	}
	// test "POST /add" with formdata value1=13&value2=29
	{
		req := &fakeRequest{post: true, postForm: map[string]string{
			"value1": "13",
			"value2": "29",
		}}
		res := s.servAdd(req)
		assertEq(t, webs.TemplateResponse, res.Type)
		assertEq(t, "add.html", res.TemplateName)
		assertEq(t, 3, len(res.TemplateData))
		assertEq(t, 13, res.TemplateData["value1"])
		assertEq(t, 29, res.TemplateData["value2"])
		assertEq(t, 42, res.TemplateData["result"])
	}
}

func assertEq(t *testing.T, exp, act any) {
	t.Helper()
	if act != exp {
		t.Fatalf("expected %v but was %v", exp, act)
	}
}

// fake webs.Request

type fakeRequest struct {
	post     bool
	query    map[string]string
	postForm map[string]string
}

func (f *fakeRequest) IsPost() bool {
	return f.post
}
func (f *fakeRequest) Query(name string) string {
	return f.query[name]
}

func (f *fakeRequest) PostForm(name string) string {
	return f.postForm[name]
}

func (f *fakeRequest) FormFile(name string) (webs.FormFile, error) {
	return nil, fmt.Errorf("FormFile() not implemented in fakeRequest")
}

func (f *fakeRequest) CookieValue(name, defValue string) string {
	return defValue
}
