# dksnap
[![Build Status](https://travis-ci.org/kelda/dksnap.svg?branch=master)](https://travis-ci.org/kelda/dksnap)
[![Go Report Card](https://goreportcard.com/badge/github.com/kelda/dksnap)](https://goreportcard.com/report/github.com/kelda/dksnap)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Slack](https://kelda.io/img/dksnap/slack-badge.svg)](http://slack.kelda.io)
[![Made by Kelda](https://kelda.io/img/dksnap/love-badge.svg)](https://kelda.io)<br/>
[Install](#install)&nbsp;&nbsp;|&nbsp;&nbsp;
[Key Features](#key-features)&nbsp;&nbsp;|&nbsp;&nbsp;
[FAQ](#faq)&nbsp;&nbsp;|&nbsp;&nbsp;
[Roadmap](#roadmap)&nbsp;&nbsp;|&nbsp;&nbsp;
[Contributing](#contributing)<br/>

#### `dksnap` creates, views, and runs snapshots of Docker containers.
For **testing in development**, we often use containers with *test data*. `dksnap` allows
you to snapshot those containers at a good state, and roll back or forward as
needed.

For a full description check out this [blogpost](https://kelda.io/blog/dksnap-docker-snapshots-for-development-and-test-data).

[<img src="https://kelda.io/img/dksnap/dksnap-demo3.gif" width="75%" >](https://youtu.be/fmYGfs632-g)

# Install
Install on MacOS or Linux:

```
curl https://kelda.io/install-dksnap.sh | sh
```

Or download the latest [release](https://github.com/kelda/dksnap/releases) and
copy to your path.

## Demo
Watch the [demo](https://youtu.be/fmYGfs632-g), or try it yourself with
step-by-step [instructions](./demo/README.md):

```
# Download the demo.
git clone https://github.com/kelda/dksnap.git
cd dksnap/demo

# Start the example application. You can access it in your browser at localhost:8080.
docker-compose up -d

# Use dksnap to create snapshots of the entries in the Mongo database.
dksnap
```

# Key Features

## Create Snapshots
<img src="https://kelda.io/img/dksnap/create-snapshot2.png" width="450" height="100%">

Create a snapshot of any running Docker container. `dksnap` works with any
container, but has extra features for select databases.
* Snapshots are volume aware.  They will capture data in volumes as well as in
  the container image.
* Snapshots are database aware.  When snapshotting databases that implement the
  [plugin interface](./pkg/snapshot/types.go), `dksnap` will politely ask the
  database process to dump its contents before creating a Docker image.

## View Snapshots
<img src="https://kelda.io/img/dksnap/diff.png" width="450" height="100%">

`dksnap` includes a terminal browser that provides a tree view of all your
snapshots along with diffs showing how they've changed over time.

## Replace Running Containers
<img src="https://kelda.io/img/dksnap/replace-snapshot.gif" width="450" height="100%">

Replace a running Docker container with a snapshot taken in the past.  `dksnap`
will automatically shut down the running container, boot the snapshot image,
and restart the container using the same Docker command arguments.

## Other Features

### Works With Any Container
By default, `dksnap` creates snapshots by committing the container's
filesystem with `docker commit`, and dumping the contents of all attached
volumes.

### Database Awareness
`dksnap` is database aware, meaning it knows how to politely dump and
restore and diff database contents for the following databases:
* Mongo
* Postgres
* MySQL

It has a plugin architecture making it easy to add more databases in the
future.  Contributions welcome!

### Docker Images
`dksnap` images are simply `docker` images with some additional metadata.  This
means they can be viewed and manipulated using the standard `docker` command
line tools.

### Share Snapshots
`dksnap` stores all of the snapshot information in a `docker` image, so you can
share your snapshot by pushing it to a Docker registry just like you would any
other container image.

### Volume Awareness
Snapshots are volume aware. The official database images all store their data
in volumes  which `docker commit` does not capture.  `dksnap` saves volumes in
addition to the container filesystem.

# FAQ

#### How is this different than `docker commit`?
`dksnap` uses `docker commit` for its generic snapshot approach to capture the
container’s filesystem. However, `docker commit` has distinct limitations:
* It doesn't capture volumes, so it can't be used with most database docker
  images.
* It isn't database aware.  It doesn't politely save/restore database state
  meaning it's prone to creating corrupted database images.

#### How is this different than a Docker image?
`dksnap` uses Docker images as the storage format for its snapshots, which
makes them fully compatible with all of the things you would normally do with
an image (run, push, delete, etc). You could handcraft Docker images to mimic
`dksnap` snapshots, but `dksnap` makes it easy to create them from running
containers.

# Roadmap
* Automated snapshot creation from production and staging databases in CI.
* A non-graphical CLI interface that's scriptable.
* Native support for additional databases.
* Snapshot of CPU and RAM state.

# Contributing
`dksnap` is still in alpha and under heavy development.  Contributions are very
much welcome so please get involved.  Check out the [contribution
guidelines](CONTRIBUTING.md) to get started.

### Build
`dksnap` requires being built with `go` version 1.13 or later.

It uses Go Modules, and [error wrapping](https://blog.golang.org/go1.13-errors).

```
git clone https://github.com/kelda/dksnap
cd dksnap
GO111MODULE=on go build .
./dksnap
```
