package snapshot

import (
	"time"
)

const (
	TitleLabel    = "dksnap.title"
	CreatedLabel  = "dksnap.created"
	DumpPathLabel = "dksnap.dump-path"
)

type Snapshot struct {
	Title      string
	DumpPath   string
	ImageNames []string
	BaseImage  bool
	Created    time.Time

	ImageID string

	Parent   *Snapshot
	Children []*Snapshot
}
