package languages

import "testing"

func TestCanonicalAcceptsKnownCodesAndAuto(t *testing.T) {
	got, ok := Canonical("zh-cn")
	if !ok || got != "zh-CN" {
		t.Fatalf("Canonical(zh-cn) = %q, %v", got, ok)
	}
	got, ok = Canonical(" AUTO ")
	if !ok || got != "auto" {
		t.Fatalf("Canonical(AUTO) = %q, %v", got, ok)
	}
	if _, ok := Canonical("not-a-language"); ok {
		t.Fatal("expected unknown language to be rejected")
	}
}

func TestCodesOmitAuto(t *testing.T) {
	for _, code := range Codes() {
		if code == "auto" || code == "" {
			t.Fatalf("unexpected code %q", code)
		}
	}
}
