package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		tag     string
		want    Version
		wantErr bool
	}{
		{tag: "v1.2.3", want: Version{"1", "2", "3", "", ""}},
		{tag: "v0.0.0", want: Version{"0", "0", "0", "", ""}},
		{tag: "v10.20.30", want: Version{"10", "20", "30", "", ""}},
		{tag: "v18446744073709551616.2.3", want: Version{"18446744073709551616", "2", "3", "", ""}},
		{tag: "v1.2.3-rc.1", want: Version{"1", "2", "3", "rc.1", ""}},
		{tag: "v1.2.3-alpha", want: Version{"1", "2", "3", "alpha", ""}},
		{tag: "v1.2.3-0.3.7", want: Version{"1", "2", "3", "0.3.7", ""}},
		{tag: "v1.2.3-x.7.z.92", want: Version{"1", "2", "3", "x.7.z.92", ""}},
		{tag: "v1.2.3+build.5", want: Version{"1", "2", "3", "", "build.5"}},
		{tag: "v1.2.3-rc.1+build.5", want: Version{"1", "2", "3", "rc.1", "build.5"}},
		{tag: "v1.2.3+20130313144700", want: Version{"1", "2", "3", "", "20130313144700"}},
		{tag: "v1.2.3+0.0.1", want: Version{"1", "2", "3", "", "0.0.1"}}, // padded numeric ids allowed in build

		{tag: "1.2.3", wantErr: true},
		{tag: "V1.2.3", wantErr: true},
		{tag: "v1.2", wantErr: true},
		{tag: "v1.2.3.4", wantErr: true},
		{tag: "v", wantErr: true},
		{tag: "", wantErr: true},
		{tag: "v01.2.3", wantErr: true},
		{tag: "v1.02.3", wantErr: true},
		{tag: "v1.2.03", wantErr: true},
		{tag: "v1.a.3", wantErr: true},
		{tag: "v1.-.3", wantErr: true},
		{tag: "v1.二.3", wantErr: true},
		{tag: "v+1.2.3", wantErr: true},
		{tag: "v1.2.3-", wantErr: true},
		{tag: "v1.2.3-rc..1", wantErr: true},
		{tag: "v1.2.3-rc.01", wantErr: true}, // padded numeric prerelease id
		{tag: "v1.2.3-rc_1", wantErr: true},
		{tag: "v1.2.3+", wantErr: true},
		{tag: "v1.2.3+bui ld", wantErr: true},
		{tag: "v1.2.3-1.2+", wantErr: true},
		{tag: "latest", wantErr: true},
		{tag: "main", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, err := ParseVersion(tt.tag)
			if tt.wantErr {
				require.Error(t, err, "tag %q", tt.tag)
				return
			}
			require.NoError(t, err, "tag %q", tt.tag)
			assert.Equal(t, tt.want, got, "tag %q", tt.tag)
			assert.Equal(t, tt.tag, got.String(), "round trip")
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.3+build", "v1.2.3", 0}, // build metadata ignored
		{"v2.0.0", "v1.99.99", 1},
		{"v18446744073709551616.0.0", "v18446744073709551615.999.999", 1},
		{"v1.2.3", "v1.2.4", -1},
		{"v1.2.3", "v1.3.0", -1},
		{"v1.2.3-rc.1", "v1.2.3", -1},          // prerelease < release
		{"v1.2.3-rc.1", "v1.2.3-rc.2", -1},     // numeric ids
		{"v1.2.3-rc.2", "v1.2.3-rc.10", -1},    // numeric, not lexical
		{"v1.2.3-rc.1", "v1.2.3-alpha", 1},     // numeric sorts before alphanumeric
		{"v1.2.3-alpha", "v1.2.3-alpha.1", -1}, // fewer ids sorts first
		{"v1.2.3-alpha.1", "v1.2.3-alpha.beta", -1},
		{"v1.2.3-rc.1", "v1.2.4-rc.1", -1},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
		{"v1.0.0-alpha.beta", "v1.0.0-beta", -1},
		{"v1.0.0-beta.2", "v1.0.0-beta.11", -1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			a, err := ParseVersion(tt.a)
			require.NoError(t, err)
			b, err := ParseVersion(tt.b)
			require.NoError(t, err)
			assert.Equal(t, tt.want, a.Compare(b), "%s vs %s", tt.a, tt.b)
			assert.Equal(t, -tt.want, b.Compare(a), "%s vs %s (reversed)", tt.b, tt.a)
		})
	}
}

func TestDecideUpdate(t *testing.T) {
	tests := []struct {
		name          string
		current       string
		target        string
		exact         bool
		allowPre      bool
		wantInstall   bool
		wantReasonSub string
		wantErr       bool
	}{
		{name: "newer stable", current: "v1.2.3", target: "v1.4.0", wantInstall: true, wantReasonSub: "newer"},
		{name: "already latest", current: "v1.2.3", target: "v1.2.3", wantReasonSub: "already on the latest"},
		{name: "current newer than published", current: "v1.3.0", target: "v1.2.9", wantReasonSub: "newer than latest published"},
		{name: "dev build updates", current: "dev", target: "v1.0.0", wantInstall: true, wantReasonSub: "development"},
		{name: "empty current fails closed", current: "", target: "v1.0.0", wantErr: true},
		{name: "unparsable current fails closed", current: "1.2.3", target: "v1.0.0", wantErr: true},
		{name: "unparsable target fails closed", current: "v1.0.0", target: "latest", wantErr: true},
		{
			name: "prerelease target needs opt in", current: "v1.2.3", target: "v1.3.0-rc.1",
			wantErr: true,
		},
		{
			name: "prerelease target with opt in", current: "v1.2.3", target: "v1.3.0-rc.1", allowPre: true,
			wantInstall: true, wantReasonSub: "newer",
		},
		{
			name: "older prerelease with opt in is still not a downgrade", current: "v1.3.0", target: "v1.3.0-rc.1", allowPre: true,
			wantReasonSub: "newer than latest published",
		},
		{
			name: "exact selection permits downgrade", current: "v1.5.0", target: "v1.2.3", exact: true,
			wantInstall: true, wantReasonSub: "explicit",
		},
		{
			name: "exact selection permits reinstall", current: "v1.2.3", target: "v1.2.3", exact: true,
			wantInstall: true, wantReasonSub: "explicit",
		},
		{
			name: "exact selection permits dev current", current: "dev", target: "v1.2.3", exact: true,
			wantInstall: true, wantReasonSub: "explicit",
		},
		{
			name: "exact prerelease still needs opt in", current: "v1.2.3", target: "v1.3.0-rc.1", exact: true,
			wantErr: true,
		},
		{
			name: "exact prerelease with opt in", current: "v1.2.3", target: "v1.3.0-rc.1", exact: true, allowPre: true,
			wantInstall: true, wantReasonSub: "explicit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			install, reason, err := decideUpdate(tt.current, tt.target, tt.exact, tt.allowPre)
			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, install)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantInstall, install)
			assert.Contains(t, reason, tt.wantReasonSub)
		})
	}
}
