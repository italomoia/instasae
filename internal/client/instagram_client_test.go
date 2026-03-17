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

func TestSendTextMessage_Success(t *testing.T) {
	var gotReq model.IGSendMessageRequest
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.IGSendMessageResponse{
			RecipientID: "rcpt_123",
			MessageID:   "m_456",
		})
	}))
	defer srv.Close()

	c := client.NewIGClientWithBaseURL(srv.Client(), "v25.0", srv.URL, true)
	resp, err := c.SendTextMessage(context.Background(), "page_1", "tok_abc", "rcpt_123", "Hello!")
	if err != nil {
		t.Fatalf("SendTextMessage: %v", err)
	}

	if resp.RecipientID != "rcpt_123" {
		t.Errorf("RecipientID = %q, want %q", resp.RecipientID, "rcpt_123")
	}
	if resp.MessageID != "m_456" {
		t.Errorf("MessageID = %q, want %q", resp.MessageID, "m_456")
	}
	if gotAuth != "Bearer tok_abc" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok_abc")
	}
	if gotReq.Message.Text != "Hello!" {
		t.Errorf("text = %q, want %q", gotReq.Message.Text, "Hello!")
	}
	if gotReq.Tag != "HUMAN_AGENT" {
		t.Errorf("tag = %q, want %q", gotReq.Tag, "HUMAN_AGENT")
	}
	if gotReq.Recipient.ID != "rcpt_123" {
		t.Errorf("recipient = %q, want %q", gotReq.Recipient.ID, "rcpt_123")
	}
}

func TestSendTextMessage_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"invalid recipient"}}`))
	}))
	defer srv.Close()

	c := client.NewIGClientWithBaseURL(srv.Client(), "v25.0", srv.URL, false)
	_, err := c.SendTextMessage(context.Background(), "page_1", "tok", "bad", "Hi")
	if err == nil {
		t.Error("should return error on 400")
	}
}

func TestSendAttachment_Success(t *testing.T) {
	var gotReq model.IGSendMessageRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.IGSendMessageResponse{
			RecipientID: "rcpt_123",
			MessageID:   "m_789",
		})
	}))
	defer srv.Close()

	c := client.NewIGClientWithBaseURL(srv.Client(), "v25.0", srv.URL, true)
	resp, err := c.SendAttachment(context.Background(), "page_1", "tok", "rcpt_123", "image", "https://example.com/img.jpg")
	if err != nil {
		t.Fatalf("SendAttachment: %v", err)
	}

	if resp.MessageID != "m_789" {
		t.Errorf("MessageID = %q, want %q", resp.MessageID, "m_789")
	}
	if gotReq.Message.Attachment == nil {
		t.Fatal("attachment should not be nil")
	}
	if gotReq.Message.Attachment.Type != "image" {
		t.Errorf("attachment type = %q, want %q", gotReq.Message.Attachment.Type, "image")
	}
	if gotReq.Message.Attachment.Payload.URL != "https://example.com/img.jpg" {
		t.Errorf("attachment url = %q", gotReq.Message.Attachment.Payload.URL)
	}
	if gotReq.Tag != "HUMAN_AGENT" {
		t.Errorf("tag = %q, want %q", gotReq.Tag, "HUMAN_AGENT")
	}
}

func TestGetUserProfile_Success(t *testing.T) {
	var gotPath string
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.IGUserProfile{
			Name:       "John Doe",
			Username:   "johndoe",
			ProfilePic: "https://pic.example.com/john.jpg",
			ID:         "user_123",
		})
	}))
	defer srv.Close()

	c := client.NewIGClientWithBaseURL(srv.Client(), "v25.0", srv.URL, false)
	profile, err := c.GetUserProfile(context.Background(), "tok_abc", "user_123")
	if err != nil {
		t.Fatalf("GetUserProfile: %v", err)
	}

	if profile.Name != "John Doe" {
		t.Errorf("Name = %q, want %q", profile.Name, "John Doe")
	}
	if profile.Username != "johndoe" {
		t.Errorf("Username = %q, want %q", profile.Username, "johndoe")
	}
	if gotPath != "/v25.0/user_123" {
		t.Errorf("path = %q, want %q", gotPath, "/v25.0/user_123")
	}
	if gotAuth != "Bearer tok_abc" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok_abc")
	}
}

func TestGetUserProfile_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid token"}}`))
	}))
	defer srv.Close()

	c := client.NewIGClientWithBaseURL(srv.Client(), "v25.0", srv.URL, false)
	_, err := c.GetUserProfile(context.Background(), "bad_tok", "user_123")
	if err == nil {
		t.Error("should return error on 401")
	}
}
