package registerv2

import (
	"net/url"
	"strings"
	"testing"
)

func TestWemaFaceURL(t *testing.T) {
	s := &Service{}
	s.SetWemaFaceConfig("https://face.example.net/", "key-123", "https://api.example.com/api/v1/alat/facial-verification")

	got, err := s.WemaFaceURL(" BVN ", "12345678901")
	if err != nil {
		t.Fatalf("WemaFaceURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse %q: %v", got, err)
	}
	if u.Scheme+"://"+u.Host != "https://face.example.net" || u.Path != "/" {
		t.Fatalf("unexpected base in %q", got)
	}
	q := u.Query()
	if q.Get("bvn") != "12345678901" || q.Get("x_tk") != "key-123" ||
		q.Get("cb_uri") != "https://api.example.com/api/v1/alat/facial-verification" || q.Get("rd_uri") != "" {
		t.Fatalf("unexpected query in %q", got)
	}

	nin, err := s.WemaFaceURL("nin", "10987654321")
	if err != nil || !strings.Contains(nin, "nin=10987654321") || strings.Contains(nin, "bvn=") {
		t.Fatalf("nin url = %q, err = %v", nin, err)
	}
}

func TestWemaFaceURL_Rejects(t *testing.T) {
	configured := &Service{}
	configured.SetWemaFaceConfig("https://face.example.net", "key", "")

	if _, err := configured.WemaFaceURL("passport", "123"); err == nil {
		t.Fatal("unknown id_type should be rejected")
	}
	if _, err := configured.WemaFaceURL("bvn", " "); err == nil {
		t.Fatal("empty id should be rejected")
	}
	if _, err := (&Service{}).WemaFaceURL("bvn", "12345678901"); err == nil {
		t.Fatal("an unconfigured service should refuse to build a URL")
	}
	// No callback configured: cb_uri is simply omitted.
	if got, err := configured.WemaFaceURL("bvn", "12345678901"); err != nil || strings.Contains(got, "cb_uri") {
		t.Fatalf("url = %q, err = %v", got, err)
	}
}
