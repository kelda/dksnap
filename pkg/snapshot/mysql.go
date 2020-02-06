package snapshot

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// MySQL creates snapshots for MySQL containers. It dumps the database
// using `mysqldump`.
type MySQL struct {
	// TODO: Add username and password.
	client *client.Client
}

// NewMySQL creates a new mongo snapshotter.
func NewMySQL(c *client.Client) Snapshotter {
	return &MySQL{c}
}

// Create creates a new snapshot.
func (c *MySQL) Create(ctx context.Context, container types.ContainerJSON, title, imageName string) error {
	buildContext, err := ioutil.TempDir("", "dksnap-context")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildContext)

	dump, err := exec(ctx, c.client, container.ID, []string{"mysqldump", "--all-databases"})
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(buildContext, "dump.sql"), dump, 0644); err != nil {
		return err
	}

	return buildImage(ctx, c.client, buildOptions{
		baseImage: container.Image,
		context:   buildContext,
		buildInstructions: []string{
			"COPY dump.sql /docker-entrypoint-initdb.d/dump.sql",
		},
		title:      title,
		imageNames: []string{imageName},
		dumpPath:   "/docker-entrypoint-initdb.d/dump.sql",
	})
}
