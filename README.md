GOIKI
=====

Goiki is a git-powered wiki written in Golang. It incorporates Markdown syntax (with extras) and uses Go templates for flexibility in look and feel.

Why? Because I wanted a stand-alone executable that would run on a Raspberry Pi to serve as a documentation server for the home. Gollum was a little too slow and Gitit won't compile (easily).

Building
--------

    ./bundle.sh
    $GOPATH/bin/esc -o static.go static/
    go build

Requirements
------------

* git

TODOs
-----

* Add support for authors within the git log
* Add basic authentication support
* Use authenticated user information for commits
* Compile templates/static content into the final binary
* Add support for uploading files
