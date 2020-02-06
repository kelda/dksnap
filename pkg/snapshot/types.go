package snapshot

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
)

// Snapshotter defines the interface for creating snapshots. Implementations
// may make assumptions about the type of container that is being snapshotted.
// For example, the Postgres snapshotter shells out to `pg_dump`.
type Snapshotter interface {
	Create(ctx context.Context, container types.ContainerJSON, title, imageName string) error
}

const (
	// TitleLabel is the label added to Docker images to track the title of
	// snapshots.
	TitleLabel = "dksnap.title"

	// CreatedLabel is the label added to Docker images to track the creation
	// time of snapshots.
	CreatedLabel = "dksnap.created"

	// DumpPathLabel is the label added to Docker images to track the path
	// within the container of a dump representing the state of the database.
	DumpPathLabel = "dksnap.dump-path"
)

// Snapshot represents a snapshot of a container. It can be booted by running
// referenced image.
type Snapshot struct {
	// BaseImage represents whether the image is a regular Docker image, and
	// not created by `dksnap`. Title, DumpPath, and Created are not defined
	// BaseImage is true.
	BaseImage bool

	Title      string
	DumpPath   string
	ImageNames []string
	Created    time.Time
	ImageID    string

	Parent   *Snapshot
	Children []*Snapshot
}
