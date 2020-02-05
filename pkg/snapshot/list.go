package snapshot

import (
	"context"
	"sort"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// List returns all the snapshots on the local machine.
func List(ctx context.Context, dockerClient *client.Client) ([]*Snapshot, error) {
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	// Parse all the snapshots.
	snapshotsByImageID := map[string]*Snapshot{}
	for _, img := range images {
		if len(img.RepoTags) == 0 || (len(img.RepoTags) == 1 && img.RepoTags[0] == "<none>:<none>") {
			continue
		}

		createdStr, ok := img.Labels[CreatedLabel]
		if !ok {
			snapshotsByImageID[img.ID] = &Snapshot{
				BaseImage:  true,
				ImageID:    img.ID,
				ImageNames: img.RepoTags,
			}
			continue
		}

		var snap Snapshot
		created, err := time.Parse(time.RFC3339, createdStr)
		if err != nil {
			return nil, err
		}
		snap.Created = created
		snap.Title = img.Labels[TitleLabel]
		snap.DumpPath = img.Labels[DumpPathLabel]
		snap.ImageID = img.ID
		snap.ImageNames = img.RepoTags

		snapshotsByImageID[snap.ImageID] = &snap
	}

	// Populate parents.
	var snapshots []*Snapshot
	for _, snapshot := range snapshotsByImageID {
		if snapshot.BaseImage {
			continue
		}

		snapshotHistory, err := dockerClient.ImageHistory(ctx, snapshot.ImageID)
		if err != nil {
			return nil, err
		}

		for _, parentImage := range snapshotHistory {
			if parentImage.ID == snapshot.ImageID {
				continue
			}

			// Find the first snapshot parent.
			if parentSnapshot, ok := snapshotsByImageID[parentImage.ID]; ok {
				snapshot.Parent = parentSnapshot
				parentSnapshot.Children = append(parentSnapshot.Children, snapshot)
				break
			}
		}

		snapshots = append(snapshots, snapshot)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Created.After(snapshots[j].Created)
	})
	return snapshots, nil
}
