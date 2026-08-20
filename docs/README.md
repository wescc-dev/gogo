![GoGo](gogo-logo-1220x352.png)

An effective **Gopher server** written in Go by Wes C.

## Features
- Supports TLS
- Supports firewall
- Supports file access rules
- Supports configurable extension->item type rules, with optional pictograms
- Supports Lua scripts
- Supports hidden files and directories available only by direct selector
- Supports configurable timeouts and maximum request size
- Supports gophermaps
- Supports dynamic gophermaps from templates
- Supports IPv4 and IPv6
- Supports Docker (optional) linux/amd64 and linux/arm64 images available on Docker Hub

## LICENSE

GoGo is provided under the MIT License. See the [LICENSE](LICENSE) file including in this repository. 

## Build and Run from Source

***Skip to the Docker section below if you don't want to build from source.***

The source code for GoGo is available at:

```
https://github.com/wescc-dev/gogo
```

1. Install the latest Go SDK from https://golang.com
2. Build

```
go build -o gogo ./src
```

3. Run

```
./gogo
```

## Configure

Configure environment variables and firewall.

### Environment Variables

Reasonable default environment variables are in the .env file. You can edit this file or export environment variables in the shell used to run GoGo. EXPORTED environments, including those you set in your IDE or Docker container, will take precedent over the .env file.

```
TITLE="Wes C's Official Gopher Hole"
HOSTNAME=localhost
HOST_BIND_IP=0.0.0.0
PORT=70
TLS_CERT_FILE=
TLS_KEY_FILE=
GOPHER_ROOT=gopher-root
FIREWALL_CONFIG_FILE=firewall-config.json
FILE_ACCESS_CONFIG_FILE=file-access-config.json
ITEM_TYPE_CONFIG_FILE=item-type-config.json
REQUEST_TIMEOUT_SECONDS=30
REQUEST_MAXIMUM_BYTES=1024
TLS_CERT_FILE=
TLS_KEY_FILE=
```

- **TITLE** is the name of your gopher hole. It is only displayed in the gophermap generated from the .gophermap in the GOPHER_ROOT directory.

- **HOSTNAME** is the host name clients use to connect to your server (e.g. *gopher://HOSTNAME.com*). It is used to create the selectors.

- **HOST_BIND_IP** is the IP address that the server listens for client requests.
  It may or may not correspond to HOSTNAME. For example, if running in a Docker container, the HOSTNAME may resolve to the host's IP address while GoGo is listening on a Docker network address. (Inside a container, 0.0.0.0 is the simplest).

- **PORT** is the port the server listens on.

- **TLS_CERT_FILE** and **TLS_KEY_FILE** are optional paths to the server certificate and private key. Set both to enable TLS; leave both empty to use plain TCP.

- **GOPHER_ROOT** is the root directory of the documents you want to provide through Gopher. **Anything in this directory hierarchy is intended to be accessed by the public in clear text.** 
  The one exception is that GoGo will **not** serve any files or directories beginning with a period (.) 
  So, for example, *gopher-root/public/.private.text* will **not** be served.

- **FIREWALL_CONFIG_FILE** is the file containing the firewall rules, which control what IP addresses are allowed to connect to your GoGo server. (See below.)

- **FILE_ACCESS_CONFIG_FILE** is the file containing the rules for allowing or denying access to files in the GOPHER_ROOT directory.
- 
- **ITEM_TYPE_CONFIG_FILE** is the file containing the relationships between file extensions and Gopher item types.

- **REQUEST_TIMEOUT_SECONDS** is the number of seconds to read a request and to write the response (per each direction, not the total time for both read and write). This prevents consuming a connection with a very slow request/ response, intentionally or otherwise.

- **REQUEST_MAXIMUM_BYTES** is the maximum number of bytes for a client request. This prevents clients from sending a request that consumes too much memory.
  The Gopher protocol does not define a maxium request size, but modern servers definitely need one. I used 1k (1024) as the default, based on the more modern Gemini protocol's maximum.

- **TLS_CERT_FILE** and **TLS_KEY_FILE** are optional paths to the server certificate and private key. Set both to enable TLS; leave both empty to use plain TCP.

### Firewall

Gopher is an inherently insecure protocol. GoGo provides an application-level firewall so that operators can at least control what is allowed to connect to the server and request its documents. These rules are configured in the file specified by the FIREWALL_CONFIG_FILE environment variable. By default, this is *`firewall-config.cfg`*

Reasonable settings are in the default file: firewall is **enabled**, in **whitelist** mode, and allows connections only from the **local machine and local network**. 

**GoGo will not immediately be open to the internet without the operator explicitly configuring the firewall to allow it.**

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

GoGo provides a way to generate gophermaps from a template in any directory of the GOPHER_ROOT hierarchy. 
If a file named *.gophermap* is found in the directory, GoGo will 
serve a dynamically generated gophermap if there isn't a file named *gophermap* in the directory.

**If neither *.gophermap* nor *gophermap* is found, the directory's contents are served.**

| Gophermap File      | Content sent to Client                                              |
| ------------------- | ------------------------------------------------------------------- |
| gophermap           | The contents of the file                                            |
| .gophermap          | A dynamically generated gophermap from the template .gophermap file |
| *neither* (default) | The contents of the directory                                       |

GoGo generates the gophermap dynamically by substituting *tokens* with the values of variables dynamically.

| Token                   | Value Source                                                                                      |
|-------------------------|---------------------------------------------------------------------------------------------------|
| {{TITLE}}               | TITLE Environment Variable                                                                        |
| {{HOST}}                | Host name of the server                                                                           |
| {{PORT}                 | The port the host is lisenting on (70 by default)                                                 |
| {{TLS_ENABLED}}         | Whether or not TLS enabled on the server                                                          |
| {{CLIENT_IP_ADDRESS}}   | The IP Adress of the connected client                                                             |
| {{SERVER}}              | Information about the GoGo server running on the system                                       |
| {{START_TIME}}          | The date/time the server started                                                                  |
| {{UPTIME}}              | The duration that the server has been up and running                                              |
| {{CURRENT_CONNECTIONS}} | The number of connections currently being served. (>1 will be very rare)                          |
| {{TOTAL_CONNECTIONS}}   | Total number of connections the server has handled since it started up. (Not a persistent total.) |
| {{OS}}                  | The operating system of the server                                                                |
| {{ARCH}}                | The servers CPU architectire                                                                      |
| {{CPUS}}                | The number of CPUS/Cores on the server                                                            |
| {{ENTRIES}}             | Selectors for the contents of the directory                                                       |
| {{=}}, {{-}}, {{*}}     | A decorative line of =, *, or - characters, primarily for dividers.                               |

There are some examples of .gophermap templates in the *gophermap-templates* directory.

## Special Files and Directories

### Hidden Files and Directories
Any file or directory beginning with $ is considered hidden, and will not have selectors generated for it by .gophermap templates or raw directory lists.

Hidden files and directories are available only by direct selectors.

*Users must know and use the direct selector to access hidden files and directories.*

### Lua Scripts

Lua scripts are supported for dynamic content. Files with a .lua extension are executed when the directory is accessed.
The script is passed a server object with context information. 
Lua's **print** function is used to output the content to the client.

## Docker

GoGo supports running in a Docker container. 

### Docker Pull

The latest image is available with docker pull. 

```
docker pull dbppgpmdtacr/gogo:latest
```

You should then configure the network, ports, volume, and environment variable when you run the image.

See above for the **environment variables**

### Docker Compose

Instead of pulling and configuring the image when you run it, you can run the default image with the *docker-compose.yaml* file in your git working directory of this repository.

This is an example. Be sure to set the network, ports, volume, and environment variables to suit.

```
name: gogo
services:
  gogo:
    image: dbppgpmdtacr/gogo:latest
    container_name: "gogo"
    volumes:
      # Mount the gopher root directory read-only.
      # Consider binding a host path for easier updates.

      - gopher-root:/gopher-root:ro

      # Optional native TLS certificate and private key.
      #- ./certs:/certs:ro

      # Consider binding these config files to host paths for easier updates
      # It is recommended to use read-only mode.

      #- ./firewall-config.json:/firewall-config.json:ro
      #- ./file-access-config.json:/file-access-config.json:ro
    environment:
      - TITLE="Wes C's Gopher Server"
      - HOSTNAME=localhost
      - HOST_BIND_IP=0.0.0.0
      - PORT=70
      - GOPHER_ROOT=/gopher-root
      - FIREWALL_CONFIG_FILE=firewall-config.json
      - FILE_ACCESS_CONFIG_FILE=file-access-config.json
      - ITEM_TYPE_CONFIG_FILE=item-type-config.json
      - REQUEST_TIMEOUT_SECONDS=30
      - REQUEST_MAXIMUM_BYTES=1024
      # Uncomment both settings and the certificate mount to enable TLS.
      #- TLS_CERT_FILE=/certs/server.crt
      #- TLS_KEY_FILE=/certs/server.key
    ports:
      - '70:70'
volumes:
  gopher-root:

```

## Setup

Because GoGo's purpose is to serve documents, it is recommended that operators create a bind mount for the GOPHER_ROOT directory so that documents are more easily managed outside the docker container. It will require Read permissions for directories and files.

Example, change the docker-compose.yaml file to map a host volume to the container volume named in GOPHER_ROOT (/gopher-root)

```
services:
  gogo:
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

The HOST_BIND_ADDRESS may need to change for your Docker environment. One common gotcha is that it's not listening on 127.0.0.1 in most cases. It's bound to an **internal container IP address**, not a host address (unless you configure the container network settings to do so).
