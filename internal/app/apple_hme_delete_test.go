package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAppleAccountRemoteDeletion(t *testing.T) {
	for _, scenario := range []string{"success", "legacy", "inactive", "mismatch", "stop_fail", "remove_fail", "explicit_new", "explicit_auto"} {
		t.Run(scenario, func(t *testing.T) {
			var paths []string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if strings.HasSuffix(r.URL.Path, "/private") {
					fmt.Fprint(w, `[{"id":"remote-id","emailAddress":"alias@icloud.com"}]`)
					return
				}
				if strings.HasSuffix(r.URL.Path, ".em") {
					email := "alias@icloud.com"
					if scenario == "mismatch" {
						email = "other@icloud.com"
					}
					fmt.Fprintf(w, `{"id":"remote-id","emailAddress":%q,"active":%t}`, email, scenario != "inactive")
					return
				}
				if r.Method != http.MethodDelete || r.ContentLength > 0 {
					t.Errorf("unexpected mutation %s length=%d", r.Method, r.ContentLength)
				}
				if strings.HasSuffix(r.URL.Path, "/stop") {
					if scenario == "stop_fail" {
						w.WriteHeader(500)
						return
					}
					w.Header().Set("scnt", "rotated")
					http.SetCookie(w, &http.Cookie{Name: "rotation", Value: "updated", Path: "/"})
				} else {
					if scenario != "inactive" && (r.Header.Get("scnt") != "rotated" || !strings.Contains(r.Header.Get("Cookie"), "rotation=updated")) {
						t.Error("rotated credentials not used")
					}
					if scenario == "remove_fail" {
						w.WriteHeader(500)
						return
					}
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer upstream.Close()
			oldBase := appleAccountManageBaseURL
			appleAccountManageBaseURL = upstream.URL
			defer func() { appleAccountManageBaseURL = oldBase }()
			store := newTestStore(t)
			handler := NewServer(Config{}, store, discardLogger())
			cookie, user := registerTestUser(t, handler, "apple-delete", "password123")
			err := store.SaveICloudSessionForOwner(user.ID, ICloudSession{AppleID: "owner@example.com", LoginStates: []LoginState{{Kind: LoginStateAppleAccount, Scnt: "original", APIKey: "test-key", LastCheckedAt: time.Now(), ManageExpiresAt: time.Now().Add(time.Hour)}}})
			if err != nil {
				t.Fatal(err)
			}
			session := store.ICloudSessionsForOwner(user.ID)[0]
			mailbox, err := store.AddMailboxForOwner(user.ID, session.AccountID, "test", "alias@icloud.com")
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "legacy" {
				if _, err := store.SetMailboxAppleAccountID(mailbox.ID, "remote-id"); err != nil {
					t.Fatal(err)
				}
			}
			query := ""
			if scenario == "explicit_new" {
				query = "?channel=apple_account"
			}
			if scenario == "explicit_auto" {
				query = "?channel=auto"
			}
			req := httptest.NewRequest(http.MethodDelete, "/api/mailboxes/"+mailbox.ID+"/remote"+query, nil)
			req.AddCookie(cookie)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			remaining, exists := store.FindMailboxByID(mailbox.ID)
			success := scenario == "success" || scenario == "legacy" || scenario == "inactive" || scenario == "explicit_new" || scenario == "explicit_auto"
			if (rr.Code == 200) != success || exists == success {
				t.Fatalf("status=%d exists=%t body=%s", rr.Code, exists, rr.Body.String())
			}
			if scenario == "remove_fail" && (remaining.APIActive || remaining.ICloudActive) {
				t.Fatal("partial failure not disabled")
			}
			want := []string{}
			base := "/account/manage/email/private/remote-id"
			if scenario == "legacy" {
				want = append(want, "/account/manage/email/private")
			}
			want = append(want, base+".em")
			if scenario != "mismatch" {
				if scenario != "inactive" {
					want = append(want, base+"/stop")
				}
				if scenario != "stop_fail" {
					want = append(want, base+"/remove")
				}
			}
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("paths=%v want=%v", paths, want)
			}
			if scenario == "remove_fail" {
				saved, _ := appleAccountLoginState(store.ICloudSessionsForOwner(user.ID)[0])
				if saved.Scnt != "rotated" {
					t.Fatal("rotated scnt not persisted")
				}
			}
		})
	}
}
