package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The gate covers lookup, stop, remove and persistence of rotated credentials.
func (s *Server) handleDeleteAppleAccountMailbox(w http.ResponseWriter, r *http.Request, mailbox Mailbox, session ICloudSession) {
	if err := s.deleteAppleAccountMailbox(r, mailbox, session); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "remote_deleted": true})
}

func (s *Server) deleteAppleAccountMailbox(r *http.Request, mailbox Mailbox, session ICloudSession) error {
	state, _ := appleAccountLoginState(session)
	release, err := acquireAppleAccountOperationGate(r.Context(), appleAccountOperationKey(session, state))
	if err != nil {
		return err
	}
	defer release()
	// Re-read after acquiring the gate, as another operation may rotate scnt.
	if latest, ok := s.sessionForOwnerAccount(mailbox.OwnerID, mailbox.AccountID); ok {
		session = latest
		state, _ = appleAccountLoginState(session)
	}
	client := NewICloudClient()
	persist := func() error {
		return s.store.SaveICloudSessionForOwner(mailbox.OwnerID, withAppleAccountLoginState(session, state))
	}
	err = client.deleteAppleAccountMailbox(r.Context(), &state, mailbox, func() error {
		if err := persist(); err != nil {
			return err
		}
		inactive := false
		_, err := s.store.SetMailboxStatus(mailbox.ID, &inactive, &inactive, StatusDisabled, "远端已停用，等待删除")
		return err
	})
	saveErr := persist()
	if err != nil {
		return err
	}
	if saveErr != nil {
		return errCode("session_save_failed", "远端已删除，登录态保存失败，本地记录保留", false)
	}
	if err := s.store.DeleteMailbox(mailbox.ID); err != nil {
		return err
	}
	return nil
}

func (c *ICloudClient) deleteAppleAccountMailbox(ctx context.Context, state *LoginState, mailbox Mailbox, stopped func() error) error {
	if appleAccountManageNeedsCreateRefresh(*state, time.Now()) {
		next, err := c.refreshAppleAccountManageStateUnlocked(ctx, *state)
		*state = next
		if err != nil {
			return err
		}
	}
	id := strings.TrimSpace(mailbox.AppleAccountID)
	if id == "" {
		var listing any
		if err := c.callAppleAccount(ctx, state, state.APIKey, http.MethodGet, "/account/manage/email/private", nil, &listing); err != nil {
			return err
		}
		ids := map[string]bool{}
		collectAppleMailboxIDs(listing, mailbox.Email, ids)
		if len(ids) != 1 {
			return errCode("remote_mailbox_unresolved", "新接口列表未唯一匹配邮箱 ID，本地记录保留", false)
		}
		for candidate := range ids {
			id = candidate
		}
	}
	// A fresh detail read confirms identity and active state, including on retry.
	var detail struct {
		ID     string `json:"id"`
		Email  string `json:"emailAddress"`
		Active *bool  `json:"active"`
	}
	base := "/account/manage/email/private/" + url.PathEscape(id)
	if err := c.callAppleAccount(ctx, state, state.APIKey, http.MethodGet, base+".em", nil, &detail); err != nil {
		return err
	}
	if detail.ID != id || !strings.EqualFold(strings.TrimSpace(detail.Email), mailbox.Email) || detail.Active == nil {
		return errCode("remote_mailbox_mismatch", "远端邮箱详情无法确认身份或状态，未执行删除", false)
	}
	if *detail.Active {
		if _, err := c.callAppleAccountRawOnce(ctx, state, state.APIKey, http.MethodDelete, base+"/stop", nil, nil); err != nil {
			return err
		}
	}
	if err := stopped(); err != nil {
		return err
	}
	// Do not retry destructive requests on ambiguous transport failures.
	if _, err := c.callAppleAccountRawOnce(ctx, state, state.APIKey, http.MethodDelete, base+"/remove", nil, nil); err != nil {
		return errCode("remote_delete_failed", "远端已停用，删除未确认；本地记录保留，可重试："+err.Error(), true)
	}
	markAppleAccountManageOK(state)
	return nil
}

// Allow list envelopes while requiring explicit id/emailAddress fields.
func collectAppleMailboxIDs(value any, email string, ids map[string]bool) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			collectAppleMailboxIDs(item, email, ids)
		}
	case map[string]any:
		address, _ := v["emailAddress"].(string)
		id, _ := v["id"].(string)
		if strings.EqualFold(strings.TrimSpace(address), email) && strings.TrimSpace(id) != "" {
			ids[id] = true
		}
		for _, item := range v {
			collectAppleMailboxIDs(item, email, ids)
		}
	}
}
