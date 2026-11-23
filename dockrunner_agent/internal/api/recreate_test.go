package api

import "testing"

func TestStoredPortsAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []SpecPort
	}{
		{
			name: "api shape",
			raw:  `[{"host":8080,"container":80}]`,
			want: []SpecPort{{Host: 8080, Container: 80}},
		},
		{
			name: "deploy shape",
			raw:  `[{"host_port":9090,"container_port":90}]`,
			want: []SpecPort{{Host: 9090, Container: 90}},
		},
		{
			name: "empty",
			raw:  `[]`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := storedPorts(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestSamePortsIgnoresOrder(t *testing.T) {
	a := []SpecPort{{Host: 8080, Container: 80}, {Host: 8443, Container: 443}}
	b := []SpecPort{{Host: 8443, Container: 443}, {Host: 8080, Container: 80}}
	if !samePorts(a, b) {
		t.Fatal("same mappings in another order must be equal")
	}
	if samePorts(a, b[:1]) {
		t.Fatal("different mapping count must differ")
	}
	if samePorts(a, []SpecPort{{Host: 8080, Container: 81}, {Host: 8443, Container: 443}}) {
		t.Fatal("different container port must differ")
	}
}
