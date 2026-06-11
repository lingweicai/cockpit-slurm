# Cockpit-Slurm architecture and Sample codes recommended by ChatGPT:

```text
React Frontend
      |
      | cockpit.channel()
      |
cockpit-bridge
      |
      | spawn()
      |
cockpit-slurm-channel   (short-lived proxy)
      |
      | Unix Socket
      |
/run/cockpit-slurm.sock
      |
cockpit-slurm-bridge    (persistent daemon)
      |
      +-- Slurm cache
      +-- Slurm events
      +-- sacctmgr
      +-- scontrol
      +-- slurmrestd
```

This keeps:

* Cockpit-specific code very small.
* All Slurm business logic in one persistent Go service.
* Cache shared among all browser sessions.

## 1. Persistent Go Daemon
Notes: we have alread have similar code in bridge/cmd/cockpit-slurm-bridge/main.go like below recommended codes. We may not need to update the code.

File:

```text
bridge/cmd/cockpit-slurm-bridge/main.go
```

Creates:

```text
/run/cockpit-slurm.sock
```

Example:

```go
package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
)

type Request struct {
	Action string `json:"action"`
}

type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		var req Request

		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		var resp Response

		switch req.Action {

		case "partitions":
			resp = Response{
				Status: "ok",
				Data: []string{
					"debug",
					"compute",
				},
			}

		case "nodes":
			resp = Response{
				Status: "ok",
				Data: []string{
					"node01",
					"node02",
				},
			}

		default:
			resp = Response{
				Status: "unknown action",
			}
		}

		b, _ := json.Marshal(resp)

		conn.Write(b)
		conn.Write([]byte("\n"))
	}
}

func main() {

	socket := "/run/cockpit-slurm.sock"

	os.Remove(socket)

	listener, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("listening on %s", socket)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleConn(conn)
	}
}
```

---

## 2. systemd Service
Notes: we have alread have this file, and running in backend as systemd daemon.

File:

```text
/etc/systemd/system/cockpit-slurm-bridge.service
```

```ini
[Unit]
Description=Cockpit Slurm Bridge

[Service]
ExecStart=/usr/libexec/cockpit-slurm/cockpit-slurm-bridge
Restart=always

[Install]
WantedBy=multi-user.target
```

Enable:

```bash
sudo systemctl enable --now cockpit-slurm-bridge
```

---

## 3. Lightweight Channel Helper

Notes: we may do not have this file in channel/cmd/... , please help to generate corresponding codes for this section with our own data type and interface, by referencing below code:  

This program is spawned by Cockpit.

File:

```text
channel/cmd/cockpit-slurm-channel/main.go
```

Purpose:

```text
stdin/stdout
      ↕
Unix Socket
```

Example:

```go
package main

import (
	"io"
	"log"
	"net"
	"os"
)

func main() {

	conn, err := net.Dial(
		"unix",
		"/run/cockpit-slurm.sock",
	)

	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	go io.Copy(conn, os.Stdin)

	io.Copy(os.Stdout, conn)
}
```

This is essentially a relay.

## 4. Frontend Cockpit Channel
Notes: we need to update code on cockpit.channel() for /home/user/cockpit-slurm/src/services/sinfoChannel.ts or other files with below reference:

React:

```javascript
import cockpit from "cockpit";

const channel = cockpit.channel({
    payload: "stream",
    spawn: [
        "/usr/libexec/cockpit-slurm/cockpit-slurm-channel"
    ]
});
```

---

## 5. Send Request
Notes: we do not need to update this with its requests/response. 

Request partitions:

```javascript
channel.send(JSON.stringify({
    action: "partitions"
}) + "\n");
```

---

## 6. Receive Response

```javascript
channel.addEventListener("message", event => {
    const response =
        JSON.parse(event.data);

    console.log(response);
});
```

Output:

```json
{
  "status":"ok",
  "data":["debug","compute"]
}
```

---

## 7. React Hook Example
Notes: We maybe need to update code accordingly.

```javascript
import { useEffect } from "react";
import cockpit from "cockpit";

export function usePartitions() {

    useEffect(() => {

        const channel = cockpit.channel({
            payload: "stream",
            spawn: [
                "/usr/libexec/cockpit-slurm/cockpit-slurm-channel"
            ]
        });

        channel.addEventListener(
            "message",
            event => {
                const data =
                    JSON.parse(event.data);

                console.log(data);
            }
        );

        channel.send(JSON.stringify({
            action: "partitions"
        }) + "\n");

        return () => channel.close();

    }, []);
}
```

## 8. Event Streaming Extension
We will update code for future, not at this time.

The real power comes when your daemon maintains:

```go
type Cache struct {
	Partitions []Partition
	Nodes      []Node
	Jobs       []Job
}
```

and background goroutines:

```go
go watchNodes()
go watchJobs()
go watchPartitions()
```

update the cache.

Then every connected channel receives updates:

```json
{
  "event":"job_added",
  "jobid":"12345"
}
```

without rerunning:

```bash
squeue
sinfo
sacctmgr
```

for every browser refresh.

