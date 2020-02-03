package snapshot

import (
	"time"
)

const (
	TitleLabel    = "docker-snapshot.title"
	CreatedLabel  = "docker-snapshot.created"
	DumpPathLabel = "docker-snapshot.dump-path"
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
