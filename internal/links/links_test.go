package links

import "testing"

func TestExtract(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []Link
	}{
		{
			name: "inline link",
			body: "see [the loader](04-Ship-the-loader.md) for more",
			want: []Link{{Target: "04-Ship-the-loader.md", Line: 1}},
		},
		{
			name: "reference definition",
			body: "text\n\n[loader]: 04-Ship-the-loader.md",
			want: []Link{{Target: "04-Ship-the-loader.md", Line: 3}},
		},
		{
			name: "percent-encoded space",
			body: "[a](01-Sync%20robustness.md)",
			want: []Link{{Target: "01-Sync robustness.md", Line: 1}},
		},
		{
			name: "fragment is stripped",
			body: "[a](02-Other.md#heading)",
			want: []Link{{Target: "02-Other.md", Line: 1}},
		},
		{
			name: "bare fragment and schemes are ignored",
			body: "[a](#here) [b](https://example.com) [c](mailto:x@example.com)",
			want: nil,
		},
		{
			name: "fenced code is skipped",
			body: "```\n[a](01-One.md)\n```\n[b](02-Two.md)",
			want: []Link{{Target: "02-Two.md", Line: 4}},
		},
		{
			name: "tilde fence is skipped",
			body: "~~~md\n[a](01-One.md)\n~~~",
			want: nil,
		},
		{
			name: "inline code is skipped",
			body: "`[a](01-One.md)` but [b](02-Two.md)",
			want: []Link{{Target: "02-Two.md", Line: 1}},
		},
		{
			name: "relative path outside docs",
			body: "[a](../README.md)",
			want: []Link{{Target: "../README.md", Line: 1}},
		},
		{
			name: "title after the target",
			body: `[a](02-Two.md "Two")`,
			want: []Link{{Target: "02-Two.md", Line: 1}},
		},
		{
			name: "two links on one line",
			body: "[a](01-One.md) and [b](02-Two.md)",
			want: []Link{{Target: "01-One.md", Line: 1}, {Target: "02-Two.md", Line: 1}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Extract(testCase.body)
			if len(got) != len(testCase.want) {
				t.Fatalf("got %v, want %v", got, testCase.want)
			}
			for index, want := range testCase.want {
				if got[index] != want {
					t.Errorf("link %d is %v, want %v", index, got[index], want)
				}
			}
		})
	}
}

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		fromPath string
		target   string
		want     string
	}{
		{"docs/001-One.md", "002-Two.md", "docs/002-Two.md"},
		{"docs/001-One.md", "./002-Two.md", "docs/002-Two.md"},
		{"docs/001-One.md", "../README.md", "README.md"},
		{"docs/001-One.md", "sub/notes.md", "docs/sub/notes.md"},
		{"docs/001-One.md", "/docs/002-Two.md", "docs/002-Two.md"},
	}

	for _, testCase := range cases {
		t.Run(testCase.target, func(t *testing.T) {
			if got := ResolveTarget(testCase.fromPath, testCase.target); got != testCase.want {
				t.Errorf("got %q, want %q", got, testCase.want)
			}
		})
	}
}
