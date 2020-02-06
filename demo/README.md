# Try out `dksnap`

This directory contains an example `node` application that stores data in
`mongo`. Shout out to the original author,
[scotch-io](https://github.com/scotch-io/node-todo).

The following instructions will walk you through using `dksnap` to create a
snapshot, and rollback to it. Click the image below to view a video of the demo.

[![Demo](http://i3.ytimg.com/vi/fmYGfs632-g/maxresdefault.jpg)](https://youtu.be/fmYGfs632-g)

## 1. Download the demo

```
$ git clone https://github.com/kelda/dksnap
$ cd dksnap/demo
```

## 2. Boot the app

```
$ docker-compose up -d
```

This will boot the `node` and `mongo` containers. Once they're up, access the
todo UI at [http://localhost:8080](http://localhost:8080).

## 3. Create a snapshot of `mongo`

This will let us rollback to the empty database later.

Start `dksnap` by running `dksnap`, select the `mongo` container, and enter the
snapshot form by hitting `[Enter]`.

Then, title your snapshot "No todos", and hit `[Enter]` on "Create Snapshot".

## 4. Add a todo item at [http://localhost:8080](http://localhost:8080)

## 5. Rollback to the "No todos" snapshot

Switch to the "View snapshots" tab in `dksnap` using `Ctrl-N`, and select the
"No todos" snapshot.

Then, hit `[Enter]`, select "Replace Running Container", and select the `mongo` container.

## 6. Confirm that the database was rolled back

Refresh [http://localhost:8080](http://localhost:8080) and make sure that there
are no todo items.
