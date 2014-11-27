GOIKI
=====

Goiki is a Git+Markdown powered wiki in a single executable. It incorpoates Markdown syntax for ease of writing and Git as a backend for content storage and revision history. Everything is embedded within the executable for ease of installation. Flexibility is provided with the use of custom templates and static content.

What's the point? I wanted a wiki easily run on a Raspberry Pi with no external dependencies (well, other than Git).


Getting Started
---------------

    git init data
    goiki

Browse to `localhost:4567` and you will be presented with a login for editing the _FrontPage_ page. `goiki:goiki` is the default username/password.


Configuring
-----------

The `-d` flag sends the default configuration to STDOUT. You can use this as a basis for your own configuration:

    goiki -d > goiki.conf

Read the default configuration for pointers on configurable options.


Running
-------

Normally:

    goiki -c goiki.conf

Where `goiki.conf` is the location of the configuration file. Everything configurable is specified in the configuration file.


Building
--------

    go build

If the default configuration or templates are altered, you will need to run the bundler to update that content for the build:

    ./bundle.sh

And if any of the static content changes, you will need the [esc file embedder](https://github.com/mjibson/esc):

    go get github.com/mjibson/esc
    $GOPATH/bin/esc -o static.go static/


TODOs
-----

* Add support for uploading files
* Better search
* More tests
* Cleaner code
