package server

import "testing"

func TestParseSSHRow(t *testing.T) {
	// Typical Linux ps output with procps-ng formatting — header suppressed
	// because we used `=` suffixes on -o fields. Columns are space-separated
	// with leading whitespace on numeric fields.
	tests := []struct {
		name    string
		output  string
		tpgid   string
		wantNil bool
		want    RemotePane
	}{
		{
			name:   "ssh with user@host",
			output: "12345 12345 ssh ssh sachin@seaking-local",
			tpgid:  "12345",
			want:   RemotePane{User: "sachin", Host: "seaking-local"},
		},
		{
			name:   "ssh with bare host",
			output: "12345 12345 ssh ssh seaking-local",
			tpgid:  "12345",
			want:   RemotePane{Host: "seaking-local"},
		},
		{
			name:   "ssh with -i key and -p port",
			output: " 100 100 ssh ssh -i /userdata/system/.ssh/id_ed25519 -p 2222 sachin@192.168.1.186",
			tpgid:  "100",
			want:   RemotePane{User: "sachin", Host: "192.168.1.186", Port: "2222", Key: "/userdata/system/.ssh/id_ed25519"},
		},
		{
			name:   "ssh with -oStrictHostKeyChecking=no then host",
			output: "1 1 ssh ssh -oStrictHostKeyChecking=no -l sachin seaking-local",
			tpgid:  "1",
			want:   RemotePane{User: "sachin", Host: "seaking-local"},
		},
		{
			name: "bash foreground, ssh elsewhere — pick foreground",
			output: `500 500 bash -bash
 600 600 ssh ssh sachin@otherhost`,
			tpgid:   "500",
			wantNil: true, // foreground pgid=500 is bash, not ssh
		},
		{
			name:    "foreground is not ssh",
			output:  "200 200 vim vim /tmp/foo",
			tpgid:   "200",
			wantNil: true,
		},
		{
			name:   "multiple rows, only one matches pgid",
			output: "100 100 tmux tmux new\n200 200 ssh ssh user@host\n300 200 sshd sshd",
			tpgid:  "200",
			want:   RemotePane{User: "user", Host: "host"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSSHRow(tt.output, tt.tpgid)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want %+v, got nil", tt.want)
			}
			if *got != tt.want {
				t.Fatalf("want %+v, got %+v", tt.want, *got)
			}
		})
	}
}

func TestSplitUserHost(t *testing.T) {
	tests := []struct {
		in, user, host string
	}{
		{"host", "", "host"},
		{"user@host", "user", "host"},
		{"user@192.168.1.1", "user", "192.168.1.1"},
		{"weird@name@host", "weird@name", "host"}, // last @ wins
	}
	for _, tt := range tests {
		u, h := splitUserHost(tt.in)
		if u != tt.user || h != tt.host {
			t.Errorf("splitUserHost(%q) = (%q,%q), want (%q,%q)", tt.in, u, h, tt.user, tt.host)
		}
	}
}

func TestRemotePaneAddr(t *testing.T) {
	if (RemotePane{Host: "h"}).Addr() != "h" {
		t.Fatal("bare host")
	}
	if (RemotePane{User: "u", Host: "h"}).Addr() != "u@h" {
		t.Fatal("user@host")
	}
}
