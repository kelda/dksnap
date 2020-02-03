package snapshot

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func SnapshotMongo(ctx context.Context, dockerClient *client.Client, container types.ContainerJSON, title, imageName string) error {
	buildContext, err := ioutil.TempDir("", "docker-snapshot-context")
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

	loadScript := []byte("mongorestore --drop --archive=/docker-snapshot/dump.archive")
	if err := ioutil.WriteFile(filepath.Join(buildContext, "load-dump.sh"), loadScript, 0755); err != nil {
		return err
	}

	return buildImage(ctx, dockerClient, buildOptions{
		baseImage: container.Image,
		context:   buildContext,
		buildInstructions: []string{
			"COPY dump.archive /docker-snapshot/dump.archive",
			"COPY load-dump.sh /docker-entrypoint-initdb.d/load-dump.sh",
		},
		title:      title,
		imageNames: []string{imageName},
		dumpPath:   "/docker-snapshot/dump.archive",
	})
}
