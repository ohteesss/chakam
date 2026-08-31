package validator

import "testing"

func TestToE164(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		region string
		want   string
	}{
		// The four formats found together in the users table, all one person.
		{"local with trunk zero", "08031234567", "NG", "+2348031234567"},
		{"wa_id style, no plus", "+2348031234567", "NG", "+2348031234567"},
		{"e164 with spaces", "+234 803 123 4567", "NG", "+2348031234567"},
		{"e164 already canonical", "+2348031234567", "NG", "+2348031234567"},

		{"dashes and parens", "(0803) 123-4567", "NG", "+2348031234567"},
		{"leading/trailing space", "  08031234567  ", "NG", "+2348031234567"},

		// The region argument decides what a bare local number means.
		{"local resolved as GB", "07400123456", "GB", "+447400123456"},
		{"local resolved as KE", "0712123456", "KE", "+254712123456"},

		// An explicit + wins over the default region.
		{"plus overrides region", "+447400123456", "NG", "+447400123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToE164(tt.raw, tt.region)
			if err != nil {
				t.Fatalf("ToE164(%q, %q) returned error: %v", tt.raw, tt.region, err)
			}
			if got != tt.want {
				t.Errorf("ToE164(%q, %q) = %q, want %q", tt.raw, tt.region, got, tt.want)
			}
		})
	}
}

func TestToE164Rejects(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		region string
	}{
		{"empty", "", "NG"},
		{"whitespace only", "   ", "NG"},
		{"too short", "0803", "NG"},
		{"not a number", "not-a-phone", "NG"},
		{"invalid NG prefix", "00001234567", "NG"},
		{"local number without a region", "08031234567", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := ToE164(tt.raw, tt.region); err == nil {
				t.Errorf("ToE164(%q, %q) = %q, want an error", tt.raw, tt.region, got)
			}
		})
	}
}

// The same human must normalise identically whether they message the bot
// (wa_id) or are named as a counterparty in chat (local format). This is the
// collision that created duplicate users.
func TestWaIDAndLocalFormAgree(t *testing.T) {
	fromWebhook, err := FromWaID("2348031234567")
	if err != nil {
		t.Fatalf("FromWaID returned error: %v", err)
	}
	fromChat, err := ToE164("08031234567", "NG")
	if err != nil {
		t.Fatalf("ToE164 returned error: %v", err)
	}
	if fromWebhook != fromChat {
		t.Errorf("same person normalised differently: wa_id -> %q, chat -> %q", fromWebhook, fromChat)
	}
}

func TestFromWaID(t *testing.T) {
	if _, err := FromWaID(""); err == nil {
		t.Error("FromWaID(\"\") = nil error, want an error")
	}
	got, err := FromWaID("+2348031234567")
	if err != nil {
		t.Fatalf("FromWaID with existing plus returned error: %v", err)
	}
	if got != "+2348031234567" {
		t.Errorf("FromWaID = %q, want %q", got, "+2348031234567")
	}
}

func TestRegionFor(t *testing.T) {
	tests := []struct{ e164, want string }{
		{"+2348031234567", "NG"},
		{"+447400123456", "GB"},
		{"+254712123456", "KE"},
		{"garbage", ""},
	}
	for _, tt := range tests {
		if got := RegionFor(tt.e164); got != tt.want {
			t.Errorf("RegionFor(%q) = %q, want %q", tt.e164, got, tt.want)
		}
	}
}

// A "+" in a URL query string decodes to a space, so "?phone=+2348031234567"
// reaches a handler as " 2348031234567". That must still resolve, while a local
// number must still be refused.
func TestFromWaIDHandlesQueryStringDecoding(t *testing.T) {
	for _, raw := range []string{
		"+2348031234567", // %2B encoded, decoded normally
		" 2348031234567", // "+" decoded as a space
		"2348031234567",  // bare wa_id
	} {
		got, err := FromWaID(raw)
		if err != nil {
			t.Fatalf("FromWaID(%q) returned error: %v", raw, err)
		}
		if got != "+2348031234567" {
			t.Errorf("FromWaID(%q) = %q, want %q", raw, got, "+2348031234567")
		}
	}

	// Local forms stay ambiguous and must not be silently adopted.
	for _, raw := range []string{"08031234567", " 08031234567"} {
		if got, err := FromWaID(raw); err == nil {
			t.Errorf("FromWaID(%q) = %q, want an error", raw, got)
		}
	}
}
