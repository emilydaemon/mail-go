# emily's fork of Mail-Go

This is an effort make Wii Mail less frustrating to use.

Goals:
- Sending a message to anyone who has been registered but who hasn't registered them back.
- Website with a database of friend codes, usernames, etc. Something like Wiimmfi.

# How to develop
The source is entirely here, with each individual cgi component in their own file.
A `Dockerfile` is available to create an image. You can use `docker-compose.yml` to develop on this specific component with its own mysql, or use *something that doesn't yet exist* to develop on RC24 as a whole.
You can use `docker-compose up` to start up both MariaDB and Mail-Go.

# How can I use the patcher for my own usage?
You're welcome to `POST /patch` with a `nwc24msg.cfg` under form key `uploaded_config`.

# What should I do if I'm adding a new dependency?
We use Go's 1.11+ module feature. Make sure you have this enabled. For more information, see [the Go wiki](https://github.com/golang/go/wiki/Modules).

# Credits
This is a fork of RiiConnect24's `mail-go`, which is, in turn, a fork of Disconnect24's `mail-go`.

A list of people who have contributed according to git commit logs follows. May be incomplete, apologies if I accidentally left you out.

In no particular order:
- Gamebuster
- KcrPL
- Artuto
- Spotlight
- Larsen Vallecillo/Larsenv
- Lauren Kelly
- CornierKhan1
