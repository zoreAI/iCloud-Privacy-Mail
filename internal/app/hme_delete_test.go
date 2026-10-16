package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestLocalMailboxDeletionWithoutRemoteSession(t *testing.T) {
	store := newTestStore(t)
	handler := NewServer(Config{}, store, discardLogger())
	cookie, user := registerTestUser(t, handler, "local-delete", "password123")
	otherCookie, _ := registerTestUser(t, handler, "other-user", "password123")
	mailbox, err := store.AddMailboxForOwner(user.ID, "", "removed remotely", "alias@icloud.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cookie *http.Cookie
		want   int
	}{{otherCookie, http.StatusNotFound}, {cookie, http.StatusOK}} {
		req := httptest.NewRequest(http.MethodDelete, "/api/mailboxes/"+mailbox.ID, nil)
		req.AddCookie(tc.cookie)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("status=%d want=%d body=%s", rr.Code, tc.want, rr.Body.String())
		}
		_, exists := store.FindMailboxByID(mailbox.ID)
		if exists != (tc.want != http.StatusOK) {
			t.Fatalf("unexpected local record existence: %v", exists)
		}
	}
}

func TestPublicDeleteMailboxAPIRequiresGlobalKeyAndSupportsLocalCleanup(t *testing.T) {
	store := newTestStore(t)
	handler := NewServer(Config{APIKey: "test-global-key"}, store, discardLogger())
	mailbox, err := store.AddMailboxForOwner("", "", "local cleanup", "alias@icloud.com")
	if err != nil {
		t.Fatal(err)
	}
	request := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/mailboxes/alias%40icloud.com?channel=local", nil)
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}
	if rr := request(""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status=%d body=%s", rr.Code, rr.Body.String())
	}
	if _, exists := store.FindMailboxByID(mailbox.ID); !exists {
		t.Fatal("request without key must not delete mailbox")
	}
	if rr := request("test-global-key"); rr.Code != http.StatusOK {
		t.Fatalf("valid key status=%d body=%s", rr.Code, rr.Body.String())
	}
	if _, exists := store.FindMailboxByID(mailbox.ID); exists {
		t.Fatal("valid local cleanup request must delete local mailbox")
	}
}

func TestPublicDeleteMailboxAPIRejectsAmbiguousEmail(t *testing.T) {
	store := newTestStore(t)
	first, err := store.AddMailboxForOwner("owner-one", "", "first", "alias@icloud.com")
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	duplicate := first
	duplicate.ID = "mb_duplicate"
	duplicate.OwnerID = "owner-two"
	duplicate.Label = "second"
	duplicate.Email = "ALIAS@icloud.com"
	store.state.Mailboxes = append(store.state.Mailboxes, duplicate)
	store.mu.Unlock()
	handler := NewServer(Config{APIKey: "test-global-key"}, store, discardLogger())
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/mailboxes/alias%40icloud.com?channel=local", nil)
	req.Header.Set("Authorization", "Bearer test-global-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if _, exists := store.FindMailboxByID(first.ID); !exists {
		t.Fatal("ambiguous email must not delete any mailbox")
	}
}

func TestRemoteMailboxDeletion(t *testing.T) {
	for _, scenario := range []string{"success", "inactive", "deactivate_fail", "delete_fail", "missing", "unauthorized", "forced_old", "missing_web", "invalid_channel"} {
		t.Run(scenario, func(t *testing.T) {
			var paths []string
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.URL.Path == "/v2/hme/list" {
					if scenario == "missing" {
						fmt.Fprint(w, `{"success":true,"result":{"hmeEmails":[]}}`)
						return
					}
					fmt.Fprintf(w, `{"success":true,"result":{"hmeEmails":[{"anonymousId":"remote-id","hme":"alias@icloud.com","isActive":%t}]}}`, scenario != "inactive")
					return
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" || !reflect.DeepEqual(body, map[string]string{"anonymousId": "remote-id"}) {
					t.Errorf("unexpected mutation: %s %v", r.Method, body)
				}
				if (scenario == "deactivate_fail" && r.URL.Path == "/v1/hme/deactivate") || (scenario == "delete_fail" && r.URL.Path == "/v1/hme/delete") {
					fmt.Fprint(w, `{"success":false,"error":{"errorMessage":"failed"}}`)
					return
				}
				fmt.Fprint(w, `{"success":true,"result":{}}`)
			}))
			defer remote.Close()
			store := newTestStore(t)
			handler := NewServer(Config{}, store, discardLogger())
			cookie, user := registerTestUser(t, handler, "delete-test", "password123")
			err := store.SaveICloudSessionForOwner(user.ID, ICloudSession{AppleID: "owner@example.com", DSID: "123", PremiumMailBaseURL: remote.URL, Cookies: []SessionCookie{{Name: "session", Value: "test"}}})
			if err != nil {
				t.Fatal(err)
			}
			session := store.ICloudSessionsForOwner(user.ID)[0]
			if scenario == "forced_old" || scenario == "missing_web" {
				session.LoginStates = []LoginState{{Kind: LoginStateAppleAccount, Scnt: "test-scnt", APIKey: "test-key"}}
				if err := store.SaveICloudSessionForOwner(user.ID, session); err != nil {
					t.Fatal(err)
				}
				if scenario == "missing_web" {
					// Remove legacy credentials from this isolated test fixture only.
					store.mu.Lock()
					for i := range store.state.ICloudSessions {
						store.state.ICloudSessions[i].Cookies = nil
						store.state.ICloudSessions[i].LoginStates = session.LoginStates
					}
					store.mu.Unlock()
				}
			}
			mailbox, err := store.AddMailboxForOwner(user.ID, session.AccountID, "test", "alias@icloud.com")
			if err != nil {
				t.Fatal(err)
			}
			query := ""
			if scenario == "forced_old" || scenario == "missing_web" {
				query = "?channel=icloud_web"
			}
			if scenario == "invalid_channel" {
				query = "?channel=unknown"
			}
			req := httptest.NewRequest(http.MethodDelete, "/api/mailboxes/"+mailbox.ID+"/remote"+query, nil)
			if scenario != "unauthorized" {
				req.AddCookie(cookie)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			remaining, exists := store.FindMailboxByID(mailbox.ID)
			success := scenario == "success" || scenario == "inactive" || scenario == "forced_old"
			if success != (rr.Code == http.StatusOK) || exists == success {
				t.Fatalf("status=%d exists=%t body=%s", rr.Code, exists, rr.Body.String())
			}
			if scenario == "delete_fail" && (remaining.APIActive || remaining.ICloudActive) {
				t.Fatal("partially deleted mailbox must be disabled")
			}
			want := []string{"/v2/hme/list"}
			switch scenario {
			case "success", "delete_fail", "forced_old":
				want = append(want, "/v1/hme/deactivate", "/v1/hme/delete")
			case "inactive":
				want = append(want, "/v1/hme/delete")
			case "deactivate_fail":
				want = append(want, "/v1/hme/deactivate")
			case "unauthorized", "missing_web", "invalid_channel":
				want = nil
			}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("paths=%v want=%v", paths, want)
			}
		})
	}
}
