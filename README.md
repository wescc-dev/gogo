# GoGopher

An effective Gopher server written in Go by Wes C.

## LICENSE

GoGopher is provided under the MIT License. See the [LICENSE](LICENSE) file including in this repository. 

## Build and Run from Source

***You can skip to the Docker section below if you don't want to build from source.***

The source code for GoGOpher is available at:

```
https://github.com/wescc-dev/gogopher
```

1. Install the latest Go SDK from https://golang.com
2. Build

```
go build -o gogopher ./src
```

3. Run

```
./gogopher
```

## Configure

Configure environment variables and firewall.

### Environment Variables

Reasonable default environment variables are in the .env file. You can edit this file or export environment variables in the shell used to run GoGopher. EXPORTED environments, including those you set in your IDE or Docker container, will take precedent over the .env file.

```
TITLE="Wes C's Official Gopher Hole"
HOSTNAME=localhost
HOST_BIND_IP=0.0.0.0
PORT=70
GOPHER_ROOT=gopher-root
FIREWALL_CONFIG_FILE=firewall-config.json
REQUEST_TIMEOUT_SECONDS=30
```

- **TITLE** is the name of your gopher hole. It is only displayed in the gophermap generated from the .gophermap in the GOPHER_ROOT directory.

- **HOSTNAME** is the host name clients use to connect to your server (e.g. *gopher://HOSTNAME.com*). It is used to create the selectors.

- **HOST_BIND_IP** is the IP address that the server listens for client requests.
  It may or may not correspond to HOSTNAME. For example, if running in a Docker container, the HOSTNAME may resolve to the host's IP address while GoGopher is listening on a Docker network address. (Inside a container, 0.0.0.0 is the simplest).

- **GOPHER_ROOT** is the root directory of the documents you want to provide through Gopher. **Anything in this directory hierarchy is intended to be accessed by the public in clear text.** 
  The one exception is that GoGopher will **not** serve any files or directories beginning with a period (.) 
  So, for example, *gopher-root/public/.private.text* will **not** be served.

- **FIREWALL_CONFIG_FILE** is the file containing the firewall rules, which control what IP addresses are allowed to connect to your GoGopher server. (See below.)

- **REQUEST_TIMEOUT_SECONDS** is the number of seconds to read a request and to write the response (per each direction, not the total time for both read and write). This prevents consuming a connection with a very slow request/ response, intentionally or otherwise.

### Firewall

Gopher is an inherently insecure protocol. GoGopher provides an application-level firewall so that operators can at least control what is allowed to connect to the server and request its documents. These rules are configured in the file specified by the FIREWALL_CONFIG_FILE environment variable. By default, this is *`firewall-config.cfg`*

Reasonable settings are in the default file: firewall is **enabled**, in **whitelist** mode, and allows connections only from the **local machine and local network**. 

**GoGopher will not immediately be open to the internet without the operator explicitly configuring the firewall to allow it.**

```firewall-config.json
{
  "enabled": true,
  "mode": "whitelist",
  "allowedIps": [
    "127.0.0.1",
    "192.168.1.*",
    "10.0.0.0/8",
    "::1",
    "2001:db8::/32"
  ],
  "blockedIps": []
}
```

For production, the operator will probably want to set it to *blacklist* mode and add blockedIps as needed. Setting **enabled** to *false* will disable the firewall completely, allowing connections from any client anywhere in the universe.

Mode can be set to either 

- **whitelist** - only IP address in the "allowedIps" list can connect. Others will be dropped.

- **blacklist** - IP addresses in the "blockedIps" list will be dropped. All others can connect.

IP Addresses can be individual addresses, a range in CIDR notation, or wildcard (*) placeholders.

*(Note: An entry with a single * indicates all IP addresses)*

## Gophermap Template

Gopher's default behavior for directory selectors is to present a menu of the directory's contents. This includes subdirectories so the user can navigate the server's document library.

Gophermaps allow operators to customize directory menus. If any directory contains a file named *gophermap*, it is sent instead of the directory's contents.

GoGopher provides a way to generate the top-level gophermap from a template (not subdirectories). If a file named *.gophermap* is found in the GOPHER_ROOT directory, GoGopher will generate a gophermap file if one does not already exist.

At startup, GoGopher generates the gophermap by substituting *tokens* with the values of variables dynamically. The server must be restarted to regenerate the top-level gophermap. *(Dynamic generation, including subdirectory support, is planned.)*

| Token               | Value Source                                                        |
| ------------------- | ------------------------------------------------------------------- |
| {{TITLE}}           | TITLE Environment Variable                                          |
| {{ENTRIES}}         | Selectors for the contents of the directory                         |
| {{=}}, {{-}}, {{*}} | A decorative line of =, *, or - characters, primarily for dividers. |

# Docker

GoGopher supports running in a Docker container. 

### Docker Pull

The latest image is available with docker pull. 

```
docker pull dbppgpmdtacr/gogopher:latest
```

You should then configure the network, ports, volume, and environment variable when you run the image.

See above for the **environment variables**

### Docker Compose

Instead of pulling and configuring the image when you run it, you can run the default image with the *docker-compose.yaml* file in your git working directory of this repository.

Be sure to set the network, ports, volume, and environment variables to suit.

```
name: gogopher  
services:  
  gogopher:  
    image: dbppgpmdtacr/gogopher:latest  
    container_name: "gogopher"  
    volumes:  
      - gopher-root:/gopher-root  
    environment:  
      - TITLE="Wes C's Gopher Server"  
      - HOSTNAME=localhost  
      - HOST_BIND_IP=0.0.0.0  
      - PORT=70  
      - GOPHER_ROOT=/gopher-root  
      - FIREWALL_CONFIG_FILE=firewall-config.json  
      - REQUEST_TIMEOUT_SECONDS=30  
    ports:  
      - '70:70'  
volumes:  
  gopher-root:
```

#### Setup

Because GoGopher's purpose is to serve documents, it is recommended that operators create a bind mount for the GOPHER_ROOT directory so that documents are more easily managed outside the docker container. It will require Read permissions for directories and files.

Example, change the docker-compose.yaml file to map a host volume to the container volume named in GOPHER_ROOT (/gopher-root)

```
services:
  gogopher:
    volumes:
      - /home/yourusername/gopher-root:/gopher-root
```

Ensure that the HOST_BIND_ADDRESS and PORT environment variables match those used by the container. 

The gopher protocol's official standard port is 70.

Standard URL. *This assumes the container has mapped host port 70 to container port 70.

```
    ports:  
      - '70:70'
```

```
gopher://localhost
```

Operators can modify either the host or container ports.

```
    ports:  
      - '7070:70'
```

```
gopher://localhost:7070
```

*Note that it was not necessary to change the container port.*

The HOST_BIND_ADDRESS may need to change for your Docker environment. One command gotcha is that it's not listening on 127.0.0.1 in most cases. It's bound to an **internal container IP address**, not a host address (unless you configure the container network settings to do so).
