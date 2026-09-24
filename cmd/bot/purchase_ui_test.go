package main

import (
	"strings"
	"testing"
	"time"
)

func TestPurchaseFlowRequiresCurrentStepAndUnexpiredState(t *testing.T) {
	a := &botApp{flows: map[int64]conversation{42: {
		Step: "months", PlanID: 7, Vals: map[string]string{}, Expires: time.Now().Add(time.Minute),
	}}}
	if _, ok := a.currentPurchaseFlow(42, "months"); !ok {
		t.Fatal("active months step should be usable")
	}
	if _, ok := a.currentPurchaseFlow(42, "ip"); ok {
		t.Fatal("callback for a different wizard step must be rejected")
	}
	a.flows[42] = conversation{Step: "months", Vals: map[string]string{}, Expires: time.Now().Add(-time.Second)}
	if _, ok := a.purchaseFlow(42); ok {
		t.Fatal("expired wizard should not be restorable by a back button")
	}
}

func TestTopupRetryKeepsTheOriginalAmountAndIdempotencyKey(t *testing.T) {
	app := &botApp{flows: map[int64]conversation{42: {
		Step: "topup-submit", Vals: map[string]string{"amount_toman": "50000", "operation_key": "stable-key"}, Expires: time.Now().Add(time.Minute),
	}}}
	st, ok := app.currentPurchaseFlow(42, "topup-submit")
	if !ok || st.Vals["amount_toman"] != "50000" || st.Vals["operation_key"] != "stable-key" {
		t.Fatalf("retry state = %#v, active=%t", st, ok)
	}
}

func TestPlanCardRendersLegacyPricingDescriptionAndDiscounts(t *testing.T) {
	card := formatPlanDetails(plan{
		Name: "Unlimited", Description: " مناسب برای استفاده روزمره ", BasePrice: 250000,
		BaseIP: 1, MaxIP: 4, PriceExtraIP: 30000,
		DiscountTiers: []discountTier{{Months: 6, BasisPoints: 500}, {Months: 12, BasisPoints: 1250}},
	}, "paid")
	for _, want := range []string{"Unlimited", "250,000 تومان/ماه", "هر IP اضافه: 30,000 تومان/ماه", "مناسب برای استفاده روزمره", "از 6 ماه: 5%", "از 12 ماه: 12.5%"} {
		if !strings.Contains(card, want) {
			t.Errorf("plan details missing %q: %s", want, card)
		}
	}
}

func TestTopupInstructionsIncludeCardOwnerAndMinimum(t *testing.T) {
	text := topupInstructionsText(paymentInstructions{
		CardNumber: "1234-5678", CardOwner: "Account Owner", Instructions: "رسید را ارسال کنید", MinTopupToman: 50000,
	})
	for _, want := range []string{"1234-5678", "Account Owner", "رسید را ارسال کنید", "50,000 تومان"} {
		if !strings.Contains(text, want) {
			t.Errorf("topup instructions missing %q: %s", want, text)
		}
	}
}

func TestPaymentDestinationMustBeCompleteBeforeDirectInvoice(t *testing.T) {
	if validPaymentDestination(paymentInstructions{CardNumber: "123", CardOwner: "Owner"}) != true {
		t.Fatal("configured card and owner should permit direct invoice")
	}
	for _, instructions := range []paymentInstructions{
		{},
		{CardNumber: "123"},
		{CardOwner: "Owner"},
		{CardNumber: "  ", CardOwner: "Owner"},
	} {
		if validPaymentDestination(instructions) {
			t.Errorf("incomplete payment destination accepted: %#v", instructions)
		}
	}
}

func TestTopupMinimumValidation(t *testing.T) {
	for _, tc := range []struct {
		amount, minimum int64
		wantError       bool
	}{
		{amount: 1, minimum: 0},
		{amount: 50000, minimum: 50000},
		{amount: 49999, minimum: 50000, wantError: true},
		{amount: 0, minimum: 0, wantError: true},
		{amount: -1, minimum: 0, wantError: true},
	} {
		err := validateTopupAmount(tc.amount, tc.minimum)
		if (err != nil) != tc.wantError {
			t.Errorf("validateTopupAmount(%d, %d) error=%v, wantError=%t", tc.amount, tc.minimum, err, tc.wantError)
		}
	}
}

func TestAdminCallbackRouteClassificationCoversPrivateAdminScreens(t *testing.T) {
	for _, action := range []string{"admin", "pending", "pendingtopups", "resellers", "approve", "approvetopup", "work-items", "refunds", "config", "cfg", "cfgset", "planedit", "planfield", "feature"} {
		if !adminCallbackAction(action) {
			t.Errorf("admin route %q is not subject to private-chat gate", action)
		}
	}
	for _, action := range []string{"home", "wallet", "services", "support", "select", "confirm-purchase"} {
		if adminCallbackAction(action) {
			t.Errorf("customer route %q should not be classified as admin-only", action)
		}
	}
	for _, step := range []string{"cfgvalue", "cfgtextkey", "planvalue", "plancreate", "paneltoken"} {
		if !adminFlowStep(step) {
			t.Errorf("admin text step %q is not subject to private-chat gate", step)
		}
	}
	for _, step := range []string{"months", "gb", "name", "topup", "cancelid"} {
		if adminFlowStep(step) {
			t.Errorf("customer input step %q should not be classified as admin-only", step)
		}
	}
}
