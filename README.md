# dksnap

[![Build Status](https://travis-ci.org/kelda/dksnap.svg?branch=master)](https://travis-ci.org/kelda/dksnap)

## Installation

Install on MacOS or Linux using our script:

```
curl https://kelda.io/install-dksnap.sh | sh
```

### Manual Installation

You can also download the binary and it to your PATH yourself:

**MacOS**

```
curl -Lo dksnap https://github.com/kelda/dksnap/releases/download/v0.1.0/dksnap-osx && \
    chmod +x dksnap && \
    sudo mv dksnap /usr/local/bin
```

**Linux**

```
curl -Lo dksnap https://github.com/kelda/dksnap/releases/download/v0.1.0/dksnap-linux && \
    chmod +x dksnap && \
    sudo mv dksnap /usr/local/bin
```

## Contributing

### Build

`dksnap` uses Go Modules for handling dependencies.

In order to build `dksnap`, make sure your version of `go` is 1.11 is higher,
and you have `GO111MODULE=on` enabled in your shell.

```
$ git clone https://github.com/kelda/dksnap
$ cd dksnap
$ go install .
$ dksnap
```
