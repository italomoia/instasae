package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/italomoia/instasae/internal/client"
	"github.com/italomoia/instasae/internal/model"
)

func TestCreateContact_Success(t *testing.T) {
	var gotPath string
	var gotToken string
	var gotReq model.CWCreateContactRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("api_access_token")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.CWCreateContactResponse{
			Payload: model.CWContactPayload{
				Contact: model.CWContact{
					ID:   42,
					Name: "John Doe",
					ContactInboxes: []model.CWContactInbox{
						{SourceID: "src_abc", Inbox: model.CWInbox{ID: 5}},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := client.NewCWClient(srv.Client())
	resp, err := c.CreateContact(context.Background(), srv.URL, 1, "cw_token", model.CWCreateContactRequest{
		InboxID:    5,
		Name:       "John Doe",
		Identifier: "ig_123",
	})
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}

	if resp.Payload.Contact.ID != 42 {
		t.Errorf("ContactID = %d, want 42", resp.Payload.Contact.ID)
	}
	if len(resp.Payload.Contact.ContactInboxes) != 1 {
		t.Fatal("expected 1 contact inbox")
	}
	if resp.Payload.Contact.ContactInboxes[0].SourceID != "src_abc" {
		t.Errorf("SourceID = %q, want %q", resp.Payload.Contact.ContactInboxes[0].SourceID, "src_abc")
	}
	if gotPath != "/api/v1/accounts/1/contacts" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/accounts/1/contacts")
	}
	if gotToken != "cw_token" {
		t.Errorf("api_access_token = %q, want %q", gotToken, "cw_token")
	}
	if gotReq.InboxID != 5 {
		t.Errorf("InboxID = %d, want 5", gotReq.InboxID)
	}
}

func TestCreateConversation_Success(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.CWCreateConversationResponse{ID: 99})
	}))
	defer srv.Close()

	c := client.NewCWClient(srv.Client())
	resp, err := c.CreateConversation(context.Background(), srv.URL, 1, "tok", model.CWCreateConversationRequest{
		SourceID:  "src_abc",
		InboxID:   5,
		ContactID: 42,
		Status:    "open",
	})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	if resp.ID != 99 {
		t.Errorf("ID = %d, want 99", resp.ID)
	}
	if gotPath != "/api/v1/accounts/1/conversations" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/accounts/1/conversations")
	}
}

func TestCreateMessage_Success(t *testing.T) {
	var gotPath string
	var gotReq model.CWCreateMessageRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := client.NewCWClient(srv.Client())
	err := c.CreateMessage(context.Background(), srv.URL, 1, "tok", 99, model.CWCreateMessageRequest{
		Content:     "Hello from agent",
		MessageType: "incoming",
		Private:     false,
	})
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}

	if gotPath != "/api/v1/accounts/1/conversations/99/messages" {
		t.Errorf("path = %q, want %q", gotPath, "/api/v1/accounts/1/conversations/99/messages")
	}
	if gotReq.Content != "Hello from agent" {
		t.Errorf("Content = %q, want %q", gotReq.Content, "Hello from agent")
	}
}

func TestCreateContact_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":"invalid payload"}`))
	}))
	defer srv.Close()

	c := client.NewCWClient(srv.Client())
	_, err := c.CreateContact(context.Background(), srv.URL, 1, "tok", model.CWCreateContactRequest{})
	if err == nil {
		t.Error("should return error on 422")
	}
}
