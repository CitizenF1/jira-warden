package warden

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssueDiscoversSprintField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/field":
			fmt.Fprint(w, `[
				{
					"id":"customfield_10042",
					"name":"Sprint",
					"custom":true,
					"schema":{"custom":"com.pyxis.greenhopper.jira:gh-sprint"}
				}
			]`)
		case "/rest/api/2/issue/PCS-7696":
			if r.URL.Query().Get("fields") != "assignee,customfield_10042" {
				t.Fatalf("unexpected fields query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{
				"key":"PCS-7696",
				"fields":{
					"assignee":{"name":"SULEYAZA"},
					"customfield_10042":[
						{
							"name":"Sprint 1",
							"state":"active",
							"startDate":"2026-06-01T00:00:00.000+0000",
							"endDate":"2026-06-30T23:59:59.000+0000"
						}
					]
				}
			}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewJiraClient(server.URL, "", "token", "bearer", "auto")
	issue, err := client.Issue(context.Background(), "PCS-7696")
	if err != nil {
		t.Fatalf("fetch issue: %v", err)
	}

	if len(issue.Sprints) != 1 {
		t.Fatalf("expected one sprint, got %d", len(issue.Sprints))
	}
	if issue.Sprints[0].Name != "Sprint 1" {
		t.Fatalf("unexpected sprint: %+v", issue.Sprints[0])
	}
}
