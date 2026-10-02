package command

import "testing"

func TestNDMSStatusErrors(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		wantErr bool
	}{
		{
			name: "clean peer add success — no status error",
			resp: `{"interface":{"Wireguard0":{"wireguard":{"peer":[{"key":"K="}]}}}}`,
		},
		{
			name: "nested status array with error",
			resp: `{"interface":{"Wireguard0":{"wireguard":{"peer":{"status":[` +
				`{"status":"message","message":"ok"},` +
				`{"status":"error","message":"allow-ips already in use"}]}}}}}`,
			wantErr: true,
		},
		{
			name:    "scalar status error",
			resp:    `{"status":"error","message":"no such interface"}`,
			wantErr: true,
		},
		{
			name: "only message-level status — not an error",
			resp: `{"status":[{"status":"message","message":"peer added"}]}`,
		},
		{
			name: "unparseable response — treated as no error",
			resp: `not json`,
		},
		{
			name:    "error status without message still flagged",
			resp:    `{"status":[{"status":"error"}]}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgs := ndmsStatusErrors([]byte(tt.resp))
			if got := len(msgs) > 0; got != tt.wantErr {
				t.Errorf("ndmsStatusErrors() error=%v (msgs=%v), want error=%v", got, msgs, tt.wantErr)
			}
		})
	}
}
