package main

import (
	"fmt"
	"testing"
	"webs"
)

func Test(t *testing.T) {
	s := NewServer(webs.NewNullTemplateLoader())
	// test "GET /say?message=hello"
	{
		req := &fakeRequest{
			query: map[string]string{
				"message": "hello",
			},
		}
		res := s.servSay(req)
		assertEq(t, webs.TemplateResponse, res.Type)
		assertEq(t, "say.html", res.TemplateName)
		assertEq(t, 1, len(res.TemplateData))
		assertEq(t, "hello", res.TemplateData["message"])
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

// assertion helper

func assertEq(t *testing.T, exp, act any) {
	t.Helper()
	if act != exp {
		t.Fatalf("expected %v but was %v", exp, act)
	}
}
