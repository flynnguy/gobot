# gobot
IRC bot written in go

Copy the example_config.yaml to config.yaml and edit to include your relevent information.

Then compile using go build.

I've also successfully used gox to cross compile with:

    gox -arch="amd64" -os="linux"
