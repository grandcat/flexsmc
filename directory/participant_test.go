package directory

import (
	"reflect"
	"testing"

	"github.com/grandcat/srpc/authentication"
)

func TestSortMapByPeerID(t *testing.T) {
	type args struct {
		m map[ChannelID]*PeerInfo
	}
	tests := []struct {
		name string
		args args
		want ParticipantList
	}{
		{
			name: "simple",
			args: args{m: map[ChannelID]*PeerInfo{
				ChannelID(1): &PeerInfo{ID: authentication.PeerID("n2.flexsmc.local")},
				ChannelID(2): &PeerInfo{ID: authentication.PeerID("n1.flexsmc.local")},
				ChannelID(9): &PeerInfo{ID: authentication.PeerID("n5.flexsmc.local")},
				ChannelID(3): &PeerInfo{ID: authentication.PeerID("n3.flexsmc.local")},
			}},
			want: ParticipantList{
				{authentication.PeerID("n1.flexsmc.local"), ChannelID(2)},
				{authentication.PeerID("n2.flexsmc.local"), ChannelID(1)},
				{authentication.PeerID("n3.flexsmc.local"), ChannelID(3)},
				{authentication.PeerID("n5.flexsmc.local"), ChannelID(9)},
			},
		},
	}
	for _, tt := range tests {
		if got := SortMapByPeerID(tt.args.m); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q. SortMapByPeerID() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
