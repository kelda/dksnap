<div align="center">
  <img src="https://kelda.io/img/dksnap/logo.svg" width="400" height="100%">
  <h1>Docker Snapshots for Dev & Test Data</h1>

  [![Build Status](https://travis-ci.org/kelda/dksnap.svg?branch=master)](https://travis-ci.org/kelda/dksnap)
  [![Go Report Card](https://goreportcard.com/badge/github.com/kelda/dksnap)](https://goreportcard.com/report/github.com/kelda/dksnap)
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
  [![Slack](https://kelda.io/img/dksnap/slack-badge.svg)](http://slack.kelda.io)
  [![Made by Kelda](https://kelda.io/img/dksnap/love-badge.svg)](https://kelda.io)

  [Create Snapshots](#create-snapshots)&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;[View Snapshots](#view-snapshots)&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;[Run Snapshots](#replace-running-containers)<br/>
  [Demo](#demo)&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;[FAQ](#faq)&nbsp;&nbsp;&nbsp;|&nbsp;&nbsp;&nbsp;[Roadmap](#roadmap)<br/>

</div>

<br/>

**`dksnap` [creates](#create-snapshots), [inspects](#view-snapshots), and [runs](#replace-running-containers) snapshots of Docker containers**

Often when testing locally, we run containerized versions of databases like
Mongo, Postgres, and MySQL.  Setting up such a container with exactly the data
you need for a particular set of tests can be quite a chore.  Especially if it
needs to be done multiple times a day.

For a full description of why we built this, check out this
[blogpost](https://kelda.io/todo).

<br/>

[<img src="https://kelda.io/img/dksnap/dksnap-demo3.gif" width="100%" height="100%">](https://youtu.be/fmYGfs632-g)

# Install
Install on MacOS or Linux:

```
curl https://kelda.io/install-dksnap.sh | sh
```

Or download the latest [release](https://github.com/kelda/dksnap/releases) and
copy to your path.

## Demo
Watch the [demo](https://youtu.be/7Aaf5VCldLg), or try it yourself with
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
<img src="https://kelda.io/img/dksnap/create-snapshot2.gif" width="450" height="100%">

Create a snapshot of any running Docker container. `dksnap` works with any
container, but has extra features for select databases.
* Snapshots are volume aware.  They will capture data in volumes as well as in
  the container image.
* Snapshots are database aware.  When snapshotting databases that implement the
  [plugin interface](./pkg/snapshot/types.go), `dksnap` will politely ask the database process to
  dump its contents before creating a Docker image.

## Replace Running Containers
<img src="https://kelda.io/img/dksnap/replace-snapshot.gif" width="450" height="100%">

Replace a running Docker container with a snapshot taken in the past.  `dksnap`
will automatically shut down the running container, boot the snapshot image,
and restart the container using the same Docker command arguments.

## View Snapshots
<img src="https://kelda.io/img/dksnap/view-history2.gif" width="450" height="100%">

`dksnap` includes a terminal browser allowing you to view and manipulate the
snapshots you've created.  You can:
* See a **tree** of all the snapshots and how they relate to each other.
* See the **diff** between a snapshot and its parent.

## Other Features

### Works With Any Container

By default, `dksnap` creates snapshots by committing the container's
filesystem with `docker commit`, and dumping the contents of all attached
volumes.

### Database Awareness
`dksnap` is database aware, meaning it knows how to nicely dump and
restore database contents for the following databases:
* Mongo
* Postgres
* MySQL

It has a plugin architecture making it easy to add more databases in the
future.

### Docker Images
`dksnap` images are simply Docker images with some additional metadata.  This
means they can be viewed and manipulated using the standard `docker` command
line tools.

### Share Snapshots
Because `dksnap` stores all of the snapshot information in a `docker` image,
you can share your snapshot by pushing and pulling to Docker registries just
like you would any other Docker container.

### Volume Awareness
Snapshots are volume aware. The official database images all store their data
in volumes (for good reason) which `docker commit` does not capture.  `dksnap`
saves your volumes as well as the container’s filesystem so that all of the
container’s state is saved.

# FAQ

#### How is this different than `docker commit`?
`dksnap` uses Docker commit for its generic snapshot approach to capture the
container’s filesystem. However, `docker commit` doesn't capture volumes, so it
can’t be used with database images. It also doesn't track metadata like
snapshot title and version.

#### How is this different than a Docker image?
`dksnap` uses Docker images as the storage format for its snapshots, which makes
them fully compatible with all of the things you would normally do with an
image (run, push, delete, etc). You could handcraft Docker images to mimic
`dksnap` snapshots, but `dksnap` makes it easy to create them from running
containers.

#### Is it ready to use?

`dksnap` is in **alpha**. It's ready for daily use, but still under heavy
development, so expect the occasional bug.  Please report any
[issues](https://github.com/kelda/dksnap/issues) you may run into.

####  Does `dksnap` capture CPU and RAM?
Not currently -- it's on the roadmap.  Let us know if this would be useful.

# Roadmap

* Automated snapshot creation from production and staging databases in CI.
* A non-graphical CLI interface that's scriptable.
* Native support for additional databases.
* Snapshot of CPU and RAM state as well.

# Contributing

`dksnap` is still in alpha and under heavy development.  Contributions are very
much welcome so please get involved.  Check out the [contribution
guidelines](CONTRIBUTING.md) to get started.

### Build

`dksnap` uses Go Modules which were introduced in `go` version 1.11.

```
git clone https://github.com/kelda/dksnap
cd dksnap
GO111MODULE=on go build .
./dksnap
```
