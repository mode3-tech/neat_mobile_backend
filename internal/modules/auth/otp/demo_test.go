package otp

import "testing"

func TestDemoConfigMatches(t *testing.T) {
	demo := DemoConfig{Enabled: true, Destination: "+2348000000000", Email: "demo@example.com", Code: "123456"}
	cases := []struct {
		name    string
		cfg     DemoConfig
		channel Channel
		dst     string
		want    bool
	}{
		{"phone", demo, ChannelSMS, "+2348000000000", true},
		{"email", demo, ChannelEmail, "demo@example.com", true},
		{"other phone", demo, ChannelSMS, "+2348111111111", false},
		{"other email", demo, ChannelEmail, "someone@example.com", false},
		{"email on sms channel", demo, ChannelSMS, "demo@example.com", false},
		{"empty dst", demo, ChannelEmail, "", false},
		{"email unset", DemoConfig{Enabled: true, Destination: "+2348000000000", Code: "123456"}, ChannelEmail, "", false},
		{"disabled", DemoConfig{Destination: "+2348000000000", Email: "demo@example.com", Code: "123456"}, ChannelEmail, "demo@example.com", false},
		{"no code", DemoConfig{Enabled: true, Email: "demo@example.com"}, ChannelEmail, "demo@example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.matches(tc.channel, tc.dst); got != tc.want {
				t.Fatalf("matches(%s, %q) = %t, want %t", tc.channel, tc.dst, got, tc.want)
			}
		})
	}
}
