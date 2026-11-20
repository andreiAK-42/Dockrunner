package agents

import "testing"

func TestResolveAgentIdentity(t *testing.T) {
	cases := []struct {
		name     string
		in       HeartbeatInput
		wantID   string
		wantName string
	}{
		{
			name:     "agent sends only AGENT_NAME",
			in:       HeartbeatInput{AgentName: "Сатурн"},
			wantID:   "Сатурн",
			wantName: "Сатурн",
		},
		{
			name:     "old agent sends only AGENT_ID",
			in:       HeartbeatInput{AgentID: "worker-01"},
			wantID:   "worker-01",
			wantName: "worker-01",
		},
		{
			name:     "both set, agent_id wins as id",
			in:       HeartbeatInput{AgentID: "worker-01", AgentName: "Сатурн"},
			wantID:   "worker-01",
			wantName: "Сатурн",
		},
		{
			name:     "whitespace is ignored",
			in:       HeartbeatInput{AgentID: "  ", AgentName: "  Сатурн  "},
			wantID:   "Сатурн",
			wantName: "Сатурн",
		},
		{
			name:     "nothing set",
			in:       HeartbeatInput{},
			wantID:   "",
			wantName: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveAgentID(c.in); got != c.wantID {
				t.Errorf("resolveAgentID() = %q, want %q", got, c.wantID)
			}
			if got := resolveAgentName(c.in); got != c.wantName {
				t.Errorf("resolveAgentName() = %q, want %q", got, c.wantName)
			}
		})
	}
}
