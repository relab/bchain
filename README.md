# bchain
BChain implementation in Go

## To compile

First ensure that you have Go installed, and `$GOPATH` set up, along with `$PATH` set to include `$GOPATH/bin`. (This should all be according to standard Go install instructions.)

Then compile `bchain` as follows:
```sh
go install
```

Then run the `run.sh` script to set up some helpful aliases:
```sh
./run.sh
```

Now open four terminal windows, and in each terminal start a `bchain` replica in the following order:
```sh
b4 (the tail)
b3 (proxy tail)
b2
b1 (the head)
```

The first three replicas will wait until the last replica (the head) is started. The head replica will send 10 messages and the tail will send an ack back to the head, all passing through the other replicas.
