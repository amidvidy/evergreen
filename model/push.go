package model

import (
	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/db"
	"github.com/evergreen-ci/evergreen/db/bsonutil"
	"github.com/evergreen-ci/evergreen/model/version"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"time"
)

const (
	PushlogCollection = "pushes"
	PushLogSuccess    = "success"
	PushLogFailed     = "failed"
)

type PushLog struct {
	Id bson.ObjectId `bson:"_id,omitempty"`

	//the permanent location of the pushed file.
	Location string `bson:"location"`

	//the task id of the push stage
	TaskId string `bson:"task_id"`

	CreateTime time.Time `bson:"create_time"`
	Revision   string    `bson:"githash"`
	Status     string    `bson:"status"`

	//copied from version for the task
	RevisionOrderNumber int `bson:"order"`
}

var (
	// bson fields for the push log struct
	PushLogIdKey         = bsonutil.MustHaveTag(PushLog{}, "Id")
	PushLogLocationKey   = bsonutil.MustHaveTag(PushLog{}, "Location")
	PushLogTaskIdKey     = bsonutil.MustHaveTag(PushLog{}, "TaskId")
	PushLogCreateTimeKey = bsonutil.MustHaveTag(PushLog{}, "CreateTime")
	PushLogRevisionKey   = bsonutil.MustHaveTag(PushLog{}, "Revision")
	PushLogStatusKey     = bsonutil.MustHaveTag(PushLog{}, "Status")
	PushLogRonKey        = bsonutil.MustHaveTag(PushLog{}, "RevisionOrderNumber")
)

func NewPushLog(v *version.Version, task *Task, location string) *PushLog {
	return &PushLog{
		Id:                  bson.NewObjectId(),
		Location:            location,
		TaskId:              task.Id,
		CreateTime:          time.Now(),
		Revision:            v.Revision,
		RevisionOrderNumber: v.RevisionOrderNumber,
		Status:              evergreen.PushLogPushing,
	}
}

func (self *PushLog) Insert() error {
	return db.Insert(PushlogCollection, self)
}

func (self *PushLog) UpdateStatus(newStatus string) error {
	return db.Update(
		PushlogCollection,
		bson.M{
			PushLogIdKey: self.Id,
		},
		bson.M{
			"$set": bson.M{
				PushLogStatusKey: newStatus,
			},
		},
	)
}

func FindOnePushLog(query interface{}, projection interface{},
	sort []string) (*PushLog, error) {
	pushLog := &PushLog{}
	err := db.FindOne(
		PushlogCollection,
		query,
		projection,
		sort,
		pushLog,
	)
	if err == mgo.ErrNotFound {
		return nil, nil
	}
	return pushLog, err
}

// FindNewerPushLog returns a PushLog item if there is a file pushed from
// this version or a newer one, or one already in progress.
func FindPushLogAfter(fileLoc string, revisionOrderNumber int) (*PushLog, error) {
	query := bson.M{
		PushLogStatusKey: bson.M{
			"$in": []string{
				evergreen.PushLogPushing, evergreen.PushLogSuccess,
			},
		},
		PushLogLocationKey: fileLoc,
		PushLogRonKey: bson.M{
			"$gte": revisionOrderNumber,
		},
	}
	existingPushLog, err := FindOnePushLog(
		query,
		db.NoProjection,
		[]string{"-" + PushLogRonKey},
	)
	if err != nil {
		return nil, err
	}
	return existingPushLog, nil
}
