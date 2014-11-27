GOIKI
=====

Goiki is a Git+Markdown powered wiki. It incorpoates Markdown syntax for writing ease and Git as a backend for revision history. Everything is embedded within the executable for ease of installation, but configuration, templates and static content can be provided for flexibility.

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

Normal use Goiki:

    goiki -c goiki.conf

Where `goiki.conf` is the location of the configuration file.


Building
--------

    ./bundle.sh
    $GOPATH/bin/esc -o static.go static/
    go build


TODOs
-----

* Add support for uploading files
* Better search
