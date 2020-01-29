package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/pmezard/go-difflib/difflib"
)

func Diff(ctx context.Context, dockerClient *client.Client, x, y *Snapshot) (string, error) {
	if x.DumpPath == "" || y.DumpPath == "" {
		return "", errors.New("can't diff generic snapshots")
	}

	xDump, err := getFile(ctx, dockerClient, x.ImageID, x.DumpPath)
	if err != nil {
		return "", err
	}

	yDump, err := getFile(ctx, dockerClient, y.ImageID, y.DumpPath)
	if err != nil {
		return "", err
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(xDump)),
		B:        difflib.SplitLines(string(yDump)),
		FromFile: x.Title,
		ToFile:   y.Title,
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}

func getFile(ctx context.Context, dockerClient *client.Client, image, path string) ([]byte, error) {
	containerID, err := dockerClient.ContainerCreate(ctx, &container.Config{Image: image}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	defer dockerClient.ContainerRemove(ctx, containerID.ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   true,
		Force:         true,
	})

	tarball, _, err := dockerClient.CopyFromContainer(ctx, containerID.ID, path)
	if err != nil {
		return nil, err
	}
	defer tarball.Close()

	tr := tar.NewReader(tarball)
	for {
		header, err := tr.Next()
		switch {
		case header == nil:
			continue
		case err == io.EOF:
			return nil, errors.New("missing file")
		case err != nil:
			return nil, err
		}

		if header.Name != filepath.Base(path) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg:
			var fileBytes bytes.Buffer
			if _, err := io.Copy(&fileBytes, tr); err != nil {
				return nil, err
			}
			return fileBytes.Bytes(), nil
		default:
			return nil, errors.New("unexpected file type")
		}
	}
}
