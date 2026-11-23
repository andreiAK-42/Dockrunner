package worker

import (
	"encoding/json"
	"testing"
)

func TestPortTaskAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want PortTask
	}{
		{
			name: "deploy shape",
			raw:  `{"host_port":8080,"container_port":80}`,
			want: PortTask{Host: 8080, Container: 80},
		},
		{
			name: "api shape from old recreate records",
			raw:  `{"host":9090,"container":90}`,
			want: PortTask{Host: 9090, Container: 90},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got PortTask
			if err := json.Unmarshal([]byte(tc.raw), &got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestPortTaskMarshalsDeployShape(t *testing.T) {
	raw, err := json.Marshal(PortTask{Host: 8080, Container: 80})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"host_port":8080,"container_port":80}`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}
}
