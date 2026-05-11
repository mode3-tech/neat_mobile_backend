package otp

import "testing"

func TestNormalizeNigerianNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "local with leading zero", input: "08012345678", want: "2348012345678"},
		{name: "international with plus", input: "+2348012345678", want: "2348012345678"},
		{name: "international without plus", input: "2348012345678", want: "2348012345678"},
		{name: "ten digit local", input: "8012345678", want: "2348012345678"},
		{name: "with spaces and symbols", input: " 0801-234-5678 ", want: "2348012345678"},
		{name: "invalid short", input: "123", wantErr: true},
		{name: "invalid empty", input: "", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeNigerianNumber(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil with value %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeNigerianNumber(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeDestinationSMS(t *testing.T) {
	got, err := NormalizeDestination("+2348012345678", ChannelSMS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2348012345678" {
		t.Fatalf("NormalizeDestination returned %q, want %q", got, "2348012345678")
	}
}
