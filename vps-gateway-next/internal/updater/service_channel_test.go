package updater

import "testing"

func TestChangelogURLForChannel(t *testing.T) {
	cases := []struct {
		channel string
		want    string
	}{
		{"stable", "http://repo.hoaxisr.ru/CHANGELOG.md"},
		{"develop", "http://repo.hoaxisr.ru/develop/CHANGELOG.md"},
		{"", "http://repo.hoaxisr.ru/CHANGELOG.md"},
	}
	for _, c := range cases {
		if got := changelogURLForChannel(c.channel); got != c.want {
			t.Errorf("changelogURLForChannel(%q) = %q, want %q", c.channel, got, c.want)
		}
	}
}

func TestChangelogURLForChannel_UsesEntwareRepoURL(t *testing.T) {
	old := entwareRepoURL
	entwareRepoURL = "http://example.test"
	t.Cleanup(func() { entwareRepoURL = old })

	if got := changelogURLForChannel("stable"); got != "http://example.test/CHANGELOG.md" {
		t.Errorf("stable = %q, want http://example.test/CHANGELOG.md", got)
	}
	if got := changelogURLForChannel("develop"); got != "http://example.test/develop/CHANGELOG.md" {
		t.Errorf("develop = %q, want http://example.test/develop/CHANGELOG.md", got)
	}
}
