package uninstaller

import "testing"

func TestParseCommandLine(t *testing.T) {
	cases := []struct {
		in   string
		name string
		narg int
	}{
		{`"C:\Program Files\LocalSend\unins000.exe"`, `C:\Program Files\LocalSend\unins000.exe`, 0},
		{`C:\Windows\System32\MsiExec.exe /X{00809252-FEC6-448E-83B4-E7F55AE7E47D}`, `C:\Windows\System32\MsiExec.exe`, 1},
		{`MsiExec.exe /x{ABC} /qn`, `MsiExec.exe`, 2},
		{`"C:\Program Files (x86)\WXWork\Uninstall.exe"`, `C:\Program Files (x86)\WXWork\Uninstall.exe`, 0},
	}
	for _, c := range cases {
		name, args, err := parseCommandLine(c.in)
		if err != nil {
			t.Errorf("parse %q err: %v", c.in, err)
			continue
		}
		if name != c.name || len(args) != c.narg {
			t.Errorf("parse %q = (%q, %d args), want (%q, %d)", c.in, name, len(args), c.name, c.narg)
		}
	}
}

func TestStripVersionSuffix(t *testing.T) {
	cases := map[string]string{
		"LocalSend 版本 1.17.0": "LocalSend",
		"Foo v2.3":              "Foo",
		"Bar 1.0.0.1":           "Bar",
		"企业微信":                 "企业微信",
		"Google Chrome 129.0.6668.71": "Google Chrome",
	}
	for in, want := range cases {
		if got := stripVersionSuffix(in); got != want {
			t.Errorf("stripVersionSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}
