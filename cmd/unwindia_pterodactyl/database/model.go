package database

import (
	"fmt"
	"github.com/GSH-LAN/Unwindia_common/src/go/matchservice"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strconv"
	"time"
)

type MatchInfo struct {
	Id        string                 `bson:"_id" json:"id"`
	MatchInfo matchservice.MatchInfo `bson:"matchInfo,omitempty"`
	MatchId   string                 `bson:"matchId"`
	JobIds    []primitive.ObjectID   `json:"jobIds" bson:"jobIds"`
	CreatedAt time.Time              `bson:"createdAt,omitempty"`
	UpdatedAt time.Time              `bson:"updatedAt,omitempty"`
}

func (*MatchInfo) CollectionName() string {
	return "pterodactyl_matchinfo"
}

type Job struct {
	mgm.DefaultModel `bson:",inline"`
	Action           Action
	Status           JobStatus
	MatchId          string
	Game             string
	Slots            int
	ServerId         int
	ExecAfter        *time.Time `bson:"execAfter,omitempty"`
	MatchInfo        matchservice.MatchInfo
}

func (*Job) CollectionName() string {
	return "pterodactyl_job"
}

type Action int

const (
	ActionCreate Action = iota
	ActionDelete
	_maxAction
)

var ActionName = map[int]string{
	0: "create",
	1: "delete",
}

var ActionValue = map[string]Action{
	JobStatusName[0]: ActionCreate,
	JobStatusName[1]: ActionDelete,
}

func (m Action) String() string {
	s, ok := JobStatusName[int(m)]
	if ok {
		return s
	}
	return strconv.Itoa(int(m))
}

// UnmarshalJSON unmarshals b into MessageTypes.
func (m *Action) UnmarshalJSON(b []byte) error {
	// From json.Unmarshaler: By convention, to approximate the behavior of
	// Unmarshal itself, Unmarshalers implement UnmarshalJSON([]byte("null")) as
	// a no-op.
	if string(b) == "null" {
		return nil
	}
	if m == nil {
		return fmt.Errorf("nil receiver passed to UnmarshalJSON")
	}

	if ci, err := strconv.ParseUint(string(b), 10, 32); err == nil {
		if ci >= uint64(_maxAction) {
			return fmt.Errorf("invalid code: %q", ci)
		}

		*m = Action(ci)
		return nil
	}

	if mv, ok := ActionValue[string(b)]; ok {
		*m = mv
		return nil
	}
	return fmt.Errorf("invalid code: %q", string(b))
}

type JobStatus int

const (
	JobStatusNew JobStatus = iota
	JobStatusInProgress
	JobStatusFinished
	JobStatusError
	_maxJobStatus
)

var JobStatusName = map[int]string{
	0: "new",
	1: "inProgress",
	2: "finished",
	3: "error",
}

var JobStatusValue = map[string]JobStatus{
	JobStatusName[0]: JobStatusNew,
	JobStatusName[1]: JobStatusInProgress,
	JobStatusName[2]: JobStatusFinished,
	JobStatusName[3]: JobStatusError,
}

func (m JobStatus) String() string {
	s, ok := JobStatusName[int(m)]
	if ok {
		return s
	}
	return strconv.Itoa(int(m))
}

// UnmarshalJSON unmarshals b into MessageTypes.
func (m *JobStatus) UnmarshalJSON(b []byte) error {
	// From json.Unmarshaler: By convention, to approximate the behavior of
	// Unmarshal itself, Unmarshalers implement UnmarshalJSON([]byte("null")) as
	// a no-op.
	if string(b) == "null" {
		return nil
	}
	if m == nil {
		return fmt.Errorf("nil receiver passed to UnmarshalJSON")
	}

	if ci, err := strconv.ParseUint(string(b), 10, 32); err == nil {
		if ci >= uint64(_maxJobStatus) {
			return fmt.Errorf("invalid code: %q", ci)
		}

		*m = JobStatus(ci)
		return nil
	}

	if mv, ok := JobStatusValue[string(b)]; ok {
		*m = mv
		return nil
	}
	return fmt.Errorf("invalid code: %q", string(b))
}
