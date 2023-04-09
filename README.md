# go-exec-pool

[![MIT
licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/tim-timpani/go-exec-pool/main/LICENSE)

Really, nothing extraordinary. Just a easy way to run 
several external programs without running them all at the 
same time by just having at it with exec.Cmd.Start(). Mainly it 
keeps me from having to duplicate code in so many projects 
that need to run exec.Command in parallel using workers.

