package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestMailboxCreatedRange(t *testing.T) {
	from, until, err := mailboxCreatedRange(url.Values{"created_from": {"2026-10-01"}, "created_to": {"2026-10-01"}})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC)
	if !from.Equal(start) || !until.Equal(start.Add(24*time.Hour)) {
		t.Fatalf("bounds %v %v", from, until)
	}
	rows := []Mailbox{{CreatedAt: start.Add(-time.Nanosecond)}, {CreatedAt: start}, {CreatedAt: until.Add(-time.Nanosecond)}, {CreatedAt: until}, {}}
	if got := filterMailboxCreatedRange(rows, from, until); len(got) != 2 {
		t.Fatalf("inclusive date matches=%d", len(got))
	}
	for _, q := range []url.Values{{"created_from": {"bad"}}, {"created_to": {"2026-02-30"}}, {"created_from": {"2026-10-02"}, "created_to": {"2026-10-01"}}} {
		if _, _, err := mailboxCreatedRange(q); err == nil {
			t.Fatalf("accepted invalid range %v", q)
		}
	}
	if got := filterMailboxCreatedRange(rows, time.Time{}, until); len(got) != 3 {
		t.Fatalf("upper only=%d", len(got))
	}
	if got := filterMailboxCreatedRange(rows, from, time.Time{}); len(got) != 3 {
		t.Fatalf("lower only=%d", len(got))
	}
}

func TestMailboxStatusFilterCombinesWithAccountSearchAndPagination(t *testing.T) {
	store := newTestStore(t)
	handler := NewServer(Config{}, store, discardLogger())
	cookie, user := registerTestUser(t, handler, "status-filter", "password123")
	account, err := store.AddAccountForOwner(user.ID, "Primary", "primary@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	otherAccount, err := store.AddAccountForOwner(user.ID, "Other", "other@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		accountID string
		email     string
		status    string
	}{
		{account.ID, "alpha@icloud.com", StatusUsed},
		{account.ID, "beta@icloud.com", StatusUsed},
		{account.ID, "gamma@icloud.com", StatusAvailable},
		{otherAccount.ID, "delta@icloud.com", StatusUsed},
	} {
		mailbox, err := store.AddMailboxForOwner(user.ID, entry.accountID, "test", entry.email)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.SetMailboxStatus(mailbox.ID, nil, nil, entry.status, ""); err != nil {
			t.Fatal(err)
		}
	}

	query := url.Values{"status": {StatusUsed}, "account_key": {account.ID}, "page": {"1"}, "page_size": {"1"}}
	request := func(values url.Values) (int, struct {
		Mailboxes  []publicMailbox  `json:"mailboxes"`
		Pagination publicPagination `json:"pagination"`
	}) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/mailboxes?"+values.Encode(), nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		var result struct {
			Mailboxes  []publicMailbox  `json:"mailboxes"`
			Pagination publicPagination `json:"pagination"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return rr.Code, result
	}
	status, result := request(query)
	if status != http.StatusOK || result.Pagination.Total != 2 || result.Pagination.TotalAll != 4 || result.Pagination.TotalPages != 2 || len(result.Mailboxes) != 1 || result.Mailboxes[0].Status != StatusUsed {
		t.Fatalf("status=%d result=%+v", status, result)
	}
	query.Set("search", "beta")
	status, result = request(query)
	if status != http.StatusOK || result.Pagination.Total != 1 || len(result.Mailboxes) != 1 || result.Mailboxes[0].Email != "beta@icloud.com" {
		t.Fatalf("combined filter status=%d result=%+v", status, result)
	}
	query.Set("status", "unexpected")
	status, _ = request(query)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid status response=%d", status)
	}
}
