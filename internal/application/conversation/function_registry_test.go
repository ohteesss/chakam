package conversation

import (
	"strings"
	"testing"
)

const (
	initiatorPhone    = "+2348031234567"
	counterpartyPhone = "+2349035177909"
)

func TestWithPartyPhones_SubstitutesPlaceholders(t *testing.T) {
	in := "Peter ({counterparty}) agrees to deliver the initiator's ({initiator}) logo by Wednesday, 26 August 2026, at no cost."
	want := "Peter (" + counterpartyPhone + ") agrees to deliver the initiator's (" + initiatorPhone + ") logo by Wednesday, 26 August 2026, at no cost."

	if got := withPartyPhones(in, initiatorPhone, counterpartyPhone); got != want {
		t.Errorf("withPartyPhones()\n got: %q\nwant: %q", got, want)
	}
}

func TestWithPartyPhones_SubstitutesEveryMention(t *testing.T) {
	in := "{initiator} pays {counterparty}; {counterparty} then ships the goods to {initiator}."
	want := initiatorPhone + " pays " + counterpartyPhone + "; " + counterpartyPhone + " then ships the goods to " + initiatorPhone + "."

	if got := withPartyPhones(in, initiatorPhone, counterpartyPhone); got != want {
		t.Errorf("withPartyPhones()\n got: %q\nwant: %q", got, want)
	}
}

// The model may ignore the instruction. The record must still name both parties.
func TestWithPartyPhones_AppendsWhenPlaceholdersMissing(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "both missing",
			in:   "Peter agrees to deliver a logo by Wednesday, at no cost.",
			want: "Peter agrees to deliver a logo by Wednesday, at no cost. (Parties: initiator " + initiatorPhone + "; counterparty " + counterpartyPhone + ".)",
		},
		{
			name: "only counterparty present",
			in:   "Peter ({counterparty}) agrees to deliver a logo.",
			want: "Peter (" + counterpartyPhone + ") agrees to deliver a logo. (Parties: initiator " + initiatorPhone + ".)",
		},
		{
			name: "adds the missing full stop",
			in:   "Peter agrees to deliver a logo",
			want: "Peter agrees to deliver a logo. (Parties: initiator " + initiatorPhone + "; counterparty " + counterpartyPhone + ".)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withPartyPhones(tt.in, initiatorPhone, counterpartyPhone); got != tt.want {
				t.Errorf("withPartyPhones()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// WhatsApp template parameters reject newlines and tabs, and the description is
// passed straight into one.
func TestWithPartyPhones_StaysSingleLine(t *testing.T) {
	for _, in := range []string{
		"  Peter ({counterparty}) agrees to deliver a logo for {initiator}.  ",
		"Peter agrees to deliver a logo.",
		"",
	} {
		got := withPartyPhones(in, initiatorPhone, counterpartyPhone)
		for _, r := range got {
			if r == '\n' || r == '\t' {
				t.Errorf("withPartyPhones(%q) produced a newline or tab: %q", in, got)
				break
			}
		}
	}
}

// The model sometimes echoes the placeholders into its chat reply, most often
// when reading a draft description back for confirmation. The user must never
// see raw braces.
func TestStripPartyTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "confirmation preview leaking both tokens",
			in:   "Description: Peter ({counterparty}) is selling a laptop to the initiator ({initiator}) for ₦500,000.",
			want: "Description: Peter (the other party) is selling a laptop to the initiator (you) for ₦500,000.",
		},
		{
			name: "repeated tokens",
			in:   "{initiator} pays {counterparty}, then {counterparty} ships to {initiator}.",
			want: "you pays the other party, then the other party ships to you.",
		},
		{
			name: "clean text is untouched",
			in:   "Peter (+2348130748051) is selling a laptop to you for ₦500,000.",
			want: "Peter (+2348130748051) is selling a laptop to you for ₦500,000.",
		},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripPartyTokens(tt.in); got != tt.want {
				t.Errorf("stripPartyTokens()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// Whatever reaches the user must never contain a raw placeholder.
func TestStripPartyTokens_LeavesNoBraces(t *testing.T) {
	in := "Draft: {counterparty} agrees to pay {initiator} today."
	got := stripPartyTokens(in)
	if strings.Contains(got, initiatorToken) || strings.Contains(got, counterpartyToken) {
		t.Errorf("stripPartyTokens(%q) left a placeholder: %q", in, got)
	}
}
