package launchd

import (
	"strings"
	"testing"
)

func TestRenderPlistEscapesPathsAndDoesNotContainSecrets(t *testing.T) {
	value := renderPlist(Paths{Binary: "/tmp/a&b", Stdout: "/tmp/out", Stderr: "/tmp/err"}, "/usr/bin:/opt/bin")
	for _, want := range []string{Label, "/tmp/a&amp;b", "<string>serve</string>", "<key>KeepAlive</key><true/>"} {
		if !strings.Contains(value, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(strings.ToLower(value), "token") {
		t.Fatal("plist must not contain token material")
	}
}
