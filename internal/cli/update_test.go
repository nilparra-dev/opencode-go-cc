package cli

import "testing"

func TestPlatformAssetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goos    string
		goarch  string
		want    string
		wantErr bool
	}{
		{name: "windows amd64", goos: "windows", goarch: "amd64", want: "occb_windows-amd64.exe"},
		{name: "linux arm64", goos: "linux", goarch: "arm64", want: "occb_linux-arm64"},
		{name: "darwin amd64", goos: "darwin", goarch: "amd64", want: "occb_darwin-amd64"},
		{name: "unsupported os", goos: "freebsd", goarch: "amd64", wantErr: true},
		{name: "unsupported arch", goos: "linux", goarch: "386", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := platformAssetName(tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("unexpected asset name: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestVersionMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "same exact version", current: "v1.2.3", latest: "v1.2.3", want: true},
		{name: "same trimmed version", current: "1.2.3", latest: "v1.2.3", want: true},
		{name: "dev never matches", current: "dev", latest: "v1.2.3", want: false},
		{name: "different version", current: "v1.2.2", latest: "v1.2.3", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := versionMatches(tc.current, tc.latest); got != tc.want {
				t.Fatalf("unexpected result: got %v want %v", got, tc.want)
			}
		})
	}
}