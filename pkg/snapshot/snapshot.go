package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	mountTypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
)

// Generic creates snapshots by saving the container's filesystem with `docker
// commit`, and creating a tarball for each attached volume. The new
// container's entrypoint is then modified to load the volumes at boot.
type Generic struct {
	client *client.Client
}

// NewGeneric creates a new generic snapshotter.
func NewGeneric(c *client.Client) Snapshotter {
	return &Generic{c}
}

// Create creates a new snapshot.
func (c *Generic) Create(ctx context.Context, container types.ContainerJSON, title, imageName string) error {
	buildContext, err := ioutil.TempDir("", "dksnap-context")
	if err != nil {
		return fmt.Errorf("make build context dir: %w", err)
	}
	defer os.RemoveAll(buildContext)

	var buildInstructions, bootCommands []string
	for i, mount := range container.Mounts {
		// Skip host volumes so that we don't affect the files in them when we
		// load the snapshot later.
		if mount.Type == mountTypes.TypeBind {
			continue
		}

		volumeTarReader, _, err := c.client.CopyFromContainer(ctx, container.ID, mount.Destination)
		if err != nil {
			return fmt.Errorf("dump volume %s: %w", mount.Destination, err)
		}

		volumeTarFile, err := ioutil.TempFile(buildContext, "dksnap-volume")
		if err != nil {
			return fmt.Errorf("create volume dump %s: %w", mount.Destination, err)
		}

		if _, err := io.Copy(volumeTarFile, volumeTarReader); err != nil {
			return fmt.Errorf("write volume dump %s: %w", mount.Destination, err)
		}

		stagePath := fmt.Sprintf("/dksnap/%d", i)
		buildInstructions = append(buildInstructions,
			fmt.Sprintf("ADD %s %s", filepath.Base(volumeTarFile.Name()), stagePath))

		bootCommand := fmt.Sprintf(`
# Load %[1]s.
snapshotPath="%[2]s"
volumePath="%[1]s"
if [ ! -d "${volumePath}" ]; then
  mkdir -p "${volumePath}"
fi

rm -rf ${volumePath}/*
cp -R "${snapshotPath}/." "${volumePath}/.."
`, mount.Destination, stagePath)
		bootCommands = append(bootCommands, bootCommand)
	}

	fsCommit, err := c.client.ContainerCommit(ctx, container.ID, types.ContainerCommitOptions{
		Pause: true,
	})
	if err != nil {
		return fmt.Errorf("commit container: %w", err)
	}

	err = buildImage(ctx, c.client, buildOptions{
		baseImage:         fsCommit.ID,
		context:           buildContext,
		buildInstructions: buildInstructions,
		bootCommands:      bootCommands,
		title:             title,
		imageNames:        []string{imageName},
	})
	if err != nil {
		return fmt.Errorf("build image: %w", err)
	}
	return nil
}

type buildOptions struct {
	baseImage         string
	context           string
	buildInstructions []string
	bootCommands      []string
	title             string
	imageNames        []string
	dumpPath          string
}

func buildImage(ctx context.Context, dockerClient *client.Client, opts buildOptions) error {
	baseImageInfo, _, err := dockerClient.ImageInspectWithRaw(ctx, opts.baseImage)
	if err != nil {
		return fmt.Errorf("get base image info: %w", err)
	}

	baseEntrypoint := baseImageInfo.Config.Entrypoint
	if baseEntrypointJSON, ok := baseImageInfo.Config.Labels[BaseEntrypointLabel]; ok {
		if err := json.Unmarshal([]byte(baseEntrypointJSON), &baseEntrypoint); err != nil {
			return fmt.Errorf("malformed entrypoint value %s: %w", baseEntrypointJSON, err)
		}
	}

	// Add a script that first runs the boot commands, then runs the
	// original entrypoint.
	if len(opts.bootCommands) != 0 {
		bootScript := fmt.Sprintf(`#!/bin/sh
%s

exec %s "$@"
`,
			strings.Join(opts.bootCommands, "\n\n"),
			strings.Join(quoteStrings(baseEntrypoint), " "))

		if err := ioutil.WriteFile(filepath.Join(opts.context, "entrypoint.sh"), []byte(bootScript), 0755); err != nil {
			return fmt.Errorf("write entrypoint: %w", err)
		}

		opts.buildInstructions = append(opts.buildInstructions,
			"COPY entrypoint.sh /dksnap/entrypoint.sh",
			`ENTRYPOINT ["/dksnap/entrypoint.sh"]`)

		// Docker discards the original CMD when the entrypoint is changed, so
		// we need to copy it over explicitly.
		quotedCmds := quoteStrings(baseImageInfo.Config.Cmd)
		opts.buildInstructions = append(opts.buildInstructions,
			fmt.Sprintf(`CMD [%s]`, strings.Join(quotedCmds, ", ")))
	}

	baseEntrypointJSON, err := json.Marshal(baseEntrypoint)
	if err != nil {
		return fmt.Errorf("marshal entrypoint: %w", err)
	}

	for k, v := range map[string]string{
		TitleLabel:          opts.title,
		DumpPathLabel:       opts.dumpPath,
		CreatedLabel:        time.Now().Format(time.RFC3339),
		BaseEntrypointLabel: string(baseEntrypointJSON),
	} {
		opts.buildInstructions = append(opts.buildInstructions, fmt.Sprintf("LABEL %q=%q", k, v))
	}

	dockerfile := fmt.Sprintf(`
FROM %s
%s
`, opts.baseImage, strings.Join(opts.buildInstructions, "\n"))

	if err := ioutil.WriteFile(filepath.Join(opts.context, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}
	var buildContextTar bytes.Buffer
	if err := makeTar(&buildContextTar, opts.context); err != nil {
		return fmt.Errorf("tar build context: %w", err)
	}

	buildResp, err := dockerClient.ImageBuild(ctx, &buildContextTar, types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       opts.imageNames,
	})
	if err != nil {
		return fmt.Errorf("start build: %w", err)
	}
	defer buildResp.Body.Close()

	// Block until the build completes, and return any errors that happen
	// during the build.
	streamErr := jsonmessage.DisplayJSONMessagesStream(buildResp.Body, ioutil.Discard, 0, false, nil)
	if streamErr != nil {
		return fmt.Errorf("build image: %w", streamErr)
	}
	return nil
}

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
			return fmt.Errorf("get normalized path %q: %w", path, err)
		}

		header.Name = relPath
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write header %q: %w", header.Name, err)
		}

		fileMode := fi.Mode()
		if !fileMode.IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file %q: %w", header.Name, err)
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("write file %q: %w", header.Name, err)
		}
		return nil
	})
	return err
}

func quoteStrings(strs []string) (quoted []string) {
	for _, str := range strs {
		quoted = append(quoted, fmt.Sprintf("%q", str))
	}
	return quoted
}
