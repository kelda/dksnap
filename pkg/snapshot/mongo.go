package snapshot

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// CreateMongo creates a snapshot for Mongo containers. It dumps the database
// using `mongodump`.
func CreateMongo(ctx context.Context, dockerClient *client.Client, container types.ContainerJSON, title, imageName string) error {
	buildContext, err := ioutil.TempDir("", "dksnap-context")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildContext)

	dump, err := exec(ctx, dockerClient, container.ID, []string{"mongodump", "--archive"})
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(buildContext, "dump.archive"), dump, 0644); err != nil {
		return err
	}

	loadScript := []byte("mongorestore --drop --archive=/dksnap/dump.archive")
	if err := ioutil.WriteFile(filepath.Join(buildContext, "load-dump.sh"), loadScript, 0755); err != nil {
		return err
	}

	return buildImage(ctx, dockerClient, buildOptions{
		baseImage: container.Image,
		context:   buildContext,
		buildInstructions: []string{
			"COPY dump.archive /dksnap/dump.archive",
			"COPY load-dump.sh /docker-entrypoint-initdb.d/load-dump.sh",
		},
		title:      title,
		imageNames: []string{imageName},
		dumpPath:   "/dksnap/dump.archive",
	})
}
