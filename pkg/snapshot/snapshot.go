package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// SnapshotGeneric snapshots the container's filesystem with `docker commit`,
// and creates a tarball for each attached volume. The new container's entrypoint
// is then modified to load the volumes at boot.
func SnapshotGeneric(ctx context.Context, dockerClient *client.Client, container types.ContainerJSON, title, imageName string) error {
	buildContext, err := ioutil.TempDir("", "dksnap-context")
	if err != nil {
		return err
	}
	defer os.RemoveAll(buildContext)

	var buildInstructions, bootInstructions []string
	for i, mount := range container.Mounts {
		volumeTarReader, _, err := dockerClient.CopyFromContainer(ctx, container.ID, mount.Destination)
		if err != nil {
			return err
		}

		volumeTarFile, err := ioutil.TempFile(buildContext, "dksnap-volume")
		if err != nil {
			return err
		}

		if _, err := io.Copy(volumeTarFile, volumeTarReader); err != nil {
			return err
		}

		stagePath := fmt.Sprintf("/dksnap/%d", i)
		buildInstructions = append(buildInstructions,
			fmt.Sprintf("ADD %s %s", filepath.Base(volumeTarFile.Name()), stagePath))
		// What effect will wiping db have?
		bootInstructions = append(bootInstructions,
			fmt.Sprintf("rm -rf %s/* && cp -r %s/* %s", mount.Destination, stagePath, filepath.Dir(mount.Destination)))
	}

	// TODO: Shouldn't this be inherited from the parent?
	var args []string
	for _, arg := range container.Args {
		args = append(args, fmt.Sprintf("%q", arg))
	}
	buildInstructions = append(buildInstructions, fmt.Sprintf(`CMD [%s]`, strings.Join(args, ", ")))

	fsCommit, err := dockerClient.ContainerCommit(ctx, container.ID, types.ContainerCommitOptions{
		Pause: true,
	})
	if err != nil {
		return err
	}

	return buildImage(ctx, dockerClient, buildOptions{
		baseImage:         fsCommit.ID,
		oldEntrypoint:     container.Path,
		context:           buildContext,
		buildInstructions: buildInstructions,
		bootInstructions:  bootInstructions,
		title:             title,
		imageNames:        []string{imageName},
	})
}

type buildOptions struct {
	baseImage         string
	oldEntrypoint     string
	context           string
	buildInstructions []string
	bootInstructions  []string
	title             string
	imageNames        []string
	dumpPath          string
}

func buildImage(ctx context.Context, dockerClient *client.Client, opts buildOptions) error {
	if len(opts.bootInstructions) != 0 {
		// TODO: What if Path is undefined? Or parent defines it?
		// TODO: What if copy fails.
		bootScript := fmt.Sprintf(`#!/bin/sh
%s
exec %s $@
`, strings.Join(opts.bootInstructions, " && "), opts.oldEntrypoint)
		if err := ioutil.WriteFile(filepath.Join(opts.context, "entrypoint.sh"), []byte(bootScript), 0755); err != nil {
			return err
		}

		opts.buildInstructions = append(opts.buildInstructions, "COPY entrypoint.sh /dksnap/entrypoint.sh")
		opts.buildInstructions = append(opts.buildInstructions, `ENTRYPOINT ["/dksnap/entrypoint.sh"]`)
	}

	for k, v := range map[string]string{
		TitleLabel:    opts.title,
		DumpPathLabel: opts.dumpPath,
		CreatedLabel:  time.Now().Format(time.RFC3339),
	} {
		opts.buildInstructions = append(opts.buildInstructions, fmt.Sprintf("LABEL %q=%q", k, v))
	}

	dockerfile := fmt.Sprintf(`
FROM %s
%s
`, opts.baseImage, strings.Join(opts.buildInstructions, "\n"))

	if err := ioutil.WriteFile(filepath.Join(opts.context, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return err
	}
	var buildContextTar bytes.Buffer
	if err := makeTar(&buildContextTar, opts.context); err != nil {
		return err
	}

	buildResp, err := dockerClient.ImageBuild(ctx, &buildContextTar, types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       opts.imageNames,
	})
	if err != nil {
		return err
	}
	defer buildResp.Body.Close()
	// TODO: Process failures in Dockerfile.
	io.Copy(ioutil.Discard, buildResp.Body)
	return nil
}

// TODO: Test symlinks.
func makeTar(writer io.Writer, dir string) error {
	tw := tar.NewWriter(writer)
	defer tw.Close()

	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return fmt.Errorf("write header: %s", err)
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		header.Name = relPath
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write header: %s", err)
		}

		fileMode := fi.Mode()
		if !fileMode.IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("write file: %s", err)
		}
		return nil
	})
	return err
}
