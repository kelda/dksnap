# ![dksnap logo](https://kelda.io/img/dksnap/dksnap-logo2.png)
[![Build Status](https://travis-ci.org/kelda/dksnap.svg?branch=master)](https://travis-ci.org/kelda/dksnap)
[![Go Report Card](https://goreportcard.com/badge/github.com/kelda/dksnap)](https://goreportcard.com/report/github.com/kelda/dksnap)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Slack](https://kelda.io/img/dksnap/slack-badge.svg)](http://slack.kelda.io)
[![Made by Kelda](https://kelda.io/img/dksnap/love-badge.svg)](https://kelda.io)

[![Demo Gif](https://kelda.io/img/dksnap/dksnap-demo2.gif)](https://youtu.be/7Aaf5VCldLg)

### Docker Snapshots for Test Databases

`dksnap` creates, inspects, and runs snapshots of docker containers.
Often when testing locally, we use a container like [mongo](https://hub.docker.com/_/mongo) or
[postgres](https://hub.docker.com/_/postgres) to mock out a database.  Setting
up such a container with exactly the data you need for a particular set of
tests can be quite a chore.  Especially if it needs to be done multiple times a
day.

`dksnap` allows you to create and manage snapshots of container images. This
allows you to:

* **Create** a snapshot of your database container in a good state.
* **Replace** a running container with a snapshot you've created.
* **View** a tree of all your snapshots and diffs showing how they've changed.

For a full description of why we built this, check out this
[blogpost](https://kelda.io/todo).

`dksnap` is in **alpha**. It's ready for daily use, but still under heavy
development, so expect the occasional bug.  Please report any
[issues](https://github.com/kelda/dksnap/issues) you may run into.

## Install
Install on MacOS or Linux:

```
curl https://kelda.io/install-dksnap.sh | sh
```

Or download the latest [release](https://github.com/kelda/dksnap/releases) and
copy to your path.

#### Demo
Watch the [demo](https://youtu.be/7Aaf5VCldLg), and recreate it yourself:

```
# Download the demo.
$ git clone https://github.com/kelda/dksnap.git
$ cd dksnap/demo

# Start the example application. You can access it in your browser at localhost:8080.
$ docker-compose up -d

# Use dksnap to create snapshots of the entries in the Mongo database.
$ dksnap
```

We also have [step-by-step instructions](./demo/README.md).

## Key Features

### Create Snapshot
![Create a Snapshot](https://kelda.io/img/dksnap/create-snapshot.gif)

Create a snapshot of **any** running docker container. `dksnap` works with any
Docker container, but has extra features for select databases.
* Snapshots are volume aware.  They will capture data in volumes as well as in
  the container image.
* Snapshots are database aware.  When snapshotting databases that implement the
  [plugin interface](#database-awareness), `dksnap` will politely ask the database process to
  dump its contents before creating a docker image.

### Replace Container
![](https://kelda.io/img/dksnap/swap-snapshot.gif)

Replace a running docker container with a snapshot taken in the past.  `dksnap`
will automatically shut down the running container, boot the snapshot image,
and restart the container using the same docker command arguments.

### View Snapshots
![](https://kelda.io/img/dksnap/view-history.gif)

`dksnap` includes a terminal browser allowing you to view and manipulate the
snapshots you've created.  You can:
* See a tree of all the snapshots and how they relate to each other.
* See the diff between a snapshot and its parent.
* Create/Boot/Replace snapshots from the UI.

### Other Features

#### Database Awareness
`dksnap` is database aware, meaning it knows how to nicely dump and
restore database contents for the following databases:
* Mongo
* Postgres

It has a plugin architecture making it easy to add more databases in the
future.

**Note:** For containers that aren't among the supported databases, it falls back to
capturing the filesystem.

#### Docker Images
`dksnap` images are simply docker images with some additional metadata.  This
means they can be viewed and manipulated using the standard `docker` command
line tools.

#### Share Snapshots
Because `dksnap` stores all of the snapshot information in a `docker` image,
you can share your snapshot by pushing and pulling to Docker registries just
like you would any other Docker container.

#### Volume Awareness
Snapshots are volume aware. The official database images all store their data
in volumes (for good reason) which `docker commit` does not capture.  `dksnap`
saves your volumes as well as the container’s filesystem so that all of the
container’s state is saved.

## FAQ

#### How does it work?

By default, `dksnap` creates a snapshot by:

1. Committing the container's filesystem with `docker commit`.
1. Dumping the contents of all attached volumes.
1. Creating a new Docker image that loads the dumped data at boot.

`dksnap` also has first-class support for select databases, in which case it
runs the database-specific dump command, and creates a Docker image that loads
the dump at boot.

#### How is this different than `docker commit`?
`dksnap` uses docker commit for its generic snapshot approach to capture the
container’s filesystem. However, `docker commit` doesn't capture volumes, so it
can’t be used with database images. It also doesn't track metadata like
snapshot title and version.

#### How is this different than a docker image?
`dksnap` uses docker images as the storage format for its snapshots, which makes
them fully compatible with all of the things you would normally do with an
image (run, push, delete, etc). You could handcraft docker images to mimic
`dksnap` snapshots, but `dksnap` makes it easy to create them from running
containers.

####  Can I use this in production?
`dksnap` is not intended for use in production.  You, of course, may do what
you like.

####  Does `dksnap` capture CPU and RAM?
Not currently -- it's on the roadmap.  Let us know if this would be useful.

## Roadmap

* Automated snapshot creation from production and staging databases.
* A non-graphical CLI interface that's scriptable.
* Native support for additional databases.
* Snapshot of CPU and RAM state as well.

## Contributing

`dksnap` is still in alpha and under heavy development.  Contributions are very
much welcome so please get involved.  Check out the [contribution
guidelines](CONTRIBUTING.md) to get started.

### Build

`dksnap` uses Go Modules which were introduced in `go` version 1.11.

```
git clone https://github.com/kelda/dksnap
cd dksnap
GO111MODULE=on go install .
./dksnap
```
