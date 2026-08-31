package agreement

import "testing"

func TestNewMoney_ExponentPerCurrency(t *testing.T) {
	cases := []struct {
		major     float64
		code      string
		wantMinor int64
	}{
		{50000, "NGN", 5_000_000}, // 2 decimals → ×100
		{12.34, "USD", 1234},      // 2 decimals
		{1000, "JPY", 1000},       // 0 decimals → ×1
		{1.5, "KWD", 1500},        // 3 decimals → ×1000
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			m, err := NewMoney(tc.major, tc.code)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.AmountMinor != tc.wantMinor {
				t.Errorf("%s: AmountMinor = %d, want %d", tc.code, m.AmountMinor, tc.wantMinor)
			}
		})
	}
}

func TestNewMoney_RejectsInvalidCurrency(t *testing.T) {
	if _, err := NewMoney(100, "XYZ"); err == nil {
		t.Error("expected error for invalid currency XYZ, got nil")
	}
}

func TestMoney_MajorRoundTrips(t *testing.T) {
	m, _ := NewMoney(50000, "NGN")
	major, err := m.Major()
	if err != nil {
		t.Fatal(err)
	}
	if major != 50000 {
		t.Errorf("Major() = %v, want 50000", major)
	}
}
