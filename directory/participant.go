package directory

import (
	"sort"

	"github.com/grandcat/srpc/authentication"
)

type Participant struct {
	PeerID authentication.PeerID
	ChanID ChannelID
}

type ParticipantList []Participant

func (p ParticipantList) Len() int           { return len(p) }
func (p ParticipantList) Less(i, j int) bool { return p[i].PeerID < p[j].PeerID }
func (p ParticipantList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func ParticipantsToList(m map[ChannelID]*PeerInfo) ParticipantList {
	list := make(ParticipantList, 0, len(m))
	for cid, p := range m {
		list = append(list, Participant{p.ID, cid})
	}

	return list
}

func SortMapByPeerID(m map[ChannelID]*PeerInfo) ParticipantList {
	// Associate each PeerID with its channel ID.
	list := ParticipantsToList(m)
	// Sort by PeerID.
	sort.Sort(list)

	return list
}
