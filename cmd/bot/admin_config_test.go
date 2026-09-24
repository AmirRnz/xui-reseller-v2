package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"example.com/xui-resell-bot-v2/internal/backend"
	"gopkg.in/telebot.v3"
)

func TestPaymentInstructionPatchPreservesOtherFields(t *testing.T) {
	current := paymentInstructions{CardNumber: "1111", CardOwner: "Existing owner", Instructions: "Existing instructions"}
	tests := []struct {
		field, value string
		want         paymentInstructions
	}{
		{"card_number", "2222", paymentInstructions{CardNumber: "2222", CardOwner: "Existing owner", Instructions: "Existing instructions"}},
		{"card_owner", "New owner", paymentInstructions{CardNumber: "1111", CardOwner: "New owner", Instructions: "Existing instructions"}},
		{"instructions", "New instructions", paymentInstructions{CardNumber: "1111", CardOwner: "Existing owner", Instructions: "New instructions"}},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got, err := buildPaymentInstructionPatch(current, tt.field, tt.value)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var actual paymentInstructions
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			if actual != tt.want {
				t.Fatalf("patch = %#v, want %#v", actual, tt.want)
			}
		})
	}
	if _, err := buildPaymentInstructionPatch(current, "unknown", "value"); err == nil {
		t.Fatal("unknown payment fields must be rejected")
	}
}

func TestPaymentInstructionEditReadsAndReplacesFullObject(t *testing.T) {
	var patch paymentInstructions
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("authorization header missing")
		}
		if r.Header.Get("X-Actor-Telegram-ID") != "96937669" {
			t.Errorf("unexpected actor header: %q", r.Header.Get("X-Actor-Telegram-ID"))
		}
		methods = append(methods, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/admin/config":
			_, _ = io.WriteString(w, `{"payment_instructions":{"card_number":"1111","card_owner":"Existing owner","instructions":"Existing instructions"}}`)
		case "PATCH /v1/admin/config/payment-instructions":
			defer r.Body.Close()
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &patch); err != nil {
				t.Errorf("decode patch: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	api, err := backend.New(server.URL, "test-secret", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	app := &botApp{api: api}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := app.updatePaymentInstruction(ctx, adminTelegramID, "card_number", "2222"); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(methods, ","), "GET /v1/admin/config,PATCH /v1/admin/config/payment-instructions"; got != want {
		t.Fatalf("requests = %q, want %q", got, want)
	}
	want := paymentInstructions{CardNumber: "2222", CardOwner: "Existing owner", Instructions: "Existing instructions"}
	if patch != want {
		t.Fatalf("patch = %#v, want %#v", patch, want)
	}
}

func TestResellerApprovalSettingIsDisplayedAsCurrentValue(t *testing.T) {
	var cfg adminConfig
	if err := json.Unmarshal([]byte(`{"settings":{"reseller_approved_required":true}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.Settings.ResellerApprovedRequired {
		t.Fatal("config did not decode the enabled approval requirement")
	}
	if got := resellerApprovalButtonLabel(cfg.Settings.ResellerApprovedRequired); got != "فقط reseller تأییدشده: بله" {
		t.Fatalf("label = %q", got)
	}
	if got := resellerApprovalButtonLabel(false); got != "فقط reseller تأییدشده: خیر" {
		t.Fatalf("disabled label = %q", got)
	}
}

func TestAdminGateRequiresConfiguredIDAndBackendRole(t *testing.T) {
	if !isAdmin(actor{TelegramID: adminTelegramID, Role: "admin"}) {
		t.Fatal("configured backend admin should pass the UI gate")
	}
	if isAdmin(actor{TelegramID: adminTelegramID, Role: "reseller"}) {
		t.Fatal("matching Telegram ID without backend admin role must not pass")
	}
	if isAdmin(actor{TelegramID: adminTelegramID + 1, Role: "admin"}) {
		t.Fatal("other Telegram IDs must not pass")
	}
}

func TestAttemptKeyIsStableForRetriesAndFreshForNewUpdate(t *testing.T) {
	firstDelivery := attemptKey(41, 9, 5001, "trial-7")
	if retry := attemptKey(41, 9, 5001, "trial-7"); retry != firstDelivery {
		t.Fatalf("retry key = %q, want %q", retry, firstDelivery)
	}
	if nextDay := attemptKey(41, 9, 6001, "trial-7"); nextDay == firstDelivery {
		t.Fatal("a later button press must get a fresh attempt key")
	}
}

func TestCallbackDataRejectsStaleMenuVersions(t *testing.T) {
	app := &botApp{menus: map[int64]string{41: "current-token"}}
	parts, valid := app.callbackData(41, "trial|7|vcurrent-token")
	if !valid || len(parts) != 2 || parts[0] != "trial" || parts[1] != "7" {
		t.Fatalf("current callback = %#v, valid=%t", parts, valid)
	}
	if _, valid := app.callbackData(41, "trial|7|vprevious-token"); valid {
		t.Fatal("callback from a previous screen must be rejected")
	}
	if _, valid := app.callbackData(41, "trial|7"); valid {
		t.Fatal("callback without a screen token must be rejected")
	}
	keyboard := &telebot.ReplyMarkup{InlineKeyboard: [][]telebot.InlineButton{{{Data: "trial|7"}}}}
	bindMenuToken(keyboard, "current-token")
	if got := keyboard.InlineKeyboard[0][0].Data; got != "trial|7|vcurrent-token" {
		t.Fatalf("bound callback = %q", got)
	}
}

func TestSubscriptionLinkPagesRespectTelegramTextLimitWithoutSkipping(t *testing.T) {
	links := []string{}
	for i := 0; i < 8; i++ {
		links = append(links, fmt.Sprintf("vless://%d-%s", i, strings.Repeat("x", 1100)))
	}
	s := subscription{DisplayName: "VPN", Links: links}
	start := 0
	seen := 0
	for {
		text, next, _ := renderSubscriptionDetails(s, start)
		if utf8.RuneCountInString(text) > 4096 {
			t.Fatalf("page exceeds Telegram text limit: %d", utf8.RuneCountInString(text))
		}
		for i := start; i < len(links) && i < start+maxSubscriptionLinksPerPage; i++ {
			if strings.Contains(text, links[i]) {
				seen++
			}
		}
		if next < 0 {
			break
		}
		if next <= start {
			t.Fatalf("pagination did not advance from %d: next %d", start, next)
		}
		start = next
	}
	if seen != len(links) {
		t.Fatalf("shown links = %d, want %d", seen, len(links))
	}
}

func TestSubscriptionDetailsRenderOwnerLinksWithPagination(t *testing.T) {
	s := subscription{ID: 9, DisplayName: "My VPN", Email: "owner@example.test", Status: "active", Kind: "paid", IPLimit: 2, TrafficLimitBytes: 2_000_000_000, ExpiryTimeMS: 1_800_000_000_000, Links: []string{"vless://one", "vless://two", "vless://three", "vless://four", "vless://five", "vless://six"}}
	first, next, previous := renderSubscriptionDetails(s, 0)
	if next != 5 || previous != -1 || !strings.Contains(first, "owner@example.test") || !strings.Contains(first, "vless://one") || !strings.Contains(first, "vless://five") || strings.Contains(first, "vless://six") {
		t.Fatalf("first page or metadata is wrong: next=%d previous=%d, text=%q", next, previous, first)
	}
	second, next, previous := renderSubscriptionDetails(s, 5)
	if next != -1 || previous != 0 || !strings.Contains(second, "vless://six") {
		t.Fatalf("second page is wrong: next=%d previous=%d, text=%q", next, previous, second)
	}
}
