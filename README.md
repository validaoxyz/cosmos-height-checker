# cosmos-height-checker
This is a simple go program that continuously and efficiently checks our local node's height and compares it to publicly reachable peers. It finds the peers automatically.

The server exposes :8080/height, which has either "GOOD" or "BAD". "BAD" if it is behind the current height. This is then used together with Uptime Kuma, to track the status. If it gets "BAD", we are alerted via kuma to Slack webhook

Run it like so.
Build:
```
go build compare_heights.go
```

Run:
```
./compare_heights >> heights.log
```
In a different session:
```
tail -f heights.log
```
This should be run on the same machine where your node is running. It assumes node has port 26657 as rpc port.
