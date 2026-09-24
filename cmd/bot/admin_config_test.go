package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/xui-resell-bot-v2/internal/backend"
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
