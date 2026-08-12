# GoGopher

An effective Gopher server written in Go by Wes C.

## Build and Run

1. Install the latest Go SDK from https://golang.com
2. Build

```
go build .
```

3. Run

```
./gogopher
```

## Configure

Configure environment variables and firewall.

### Environment Variables

Reasonable default environment variables are in the .env file. You can edit this file, or export environment variables in the shell used to run GoGopher. EXPORTED environments, including those you set in your IDE or Docker container, will take precedent over the .env file.

```
TITLE="Wes C's Official Gopher Hole"
HOSTNAME=localhost
HOST_BIND_IP=0.0.0.0
PORT=70
GOPHER_ROOT=gopher-root
FIREWALL_CONFIG_FILE=firewall-config.json
IDLE_TIMEOUT_SECONDS=10
READWRITE_TIMEOUT_SECONDS=30
```

- **TITLE** is the name of your gopher hole. It is only displayed in the gophermap generated from the .gophermap in the GOPHER_ROOT directory.

- **HOSTNAME** is host name clients use to connect to your server (eg. *gopher://<HOSTNAME>.com*). It is used to create the selectors.

- **HOST_BIND_IP** is the IP address that the server listens for client requests.
  It may or may not correspnd to HOSTNAME. For example, if running in a Docker container, the HOSTNAME may resolve to the host's IP address while GoGopher is llistening on a Docker network address. (Inside a container, 0.0.0.0 is the simplest).

- **GOPHER_ROOT** is the root directory of the documents you want to provide through Gopher. **Anything in this directory hierarchy is intended to be accessed by the public in clear text.** The one exception is that GoGopher will not serve any files or directories beginning with a periond (.) So, for example *gopher-root/public/.private.text* will **not** be served.

- **FIREWALL_CONFIG_FILE** is the the file containing the firewall rules, which control what IP addresses are allowed to connect to your GoGopher server. (See below)

- **IDLE_TIMEOUT_SECONDS** is the number of seconds to wait for client request to start. If no request is received before this time, the connection is closed. This prevents malicious users from opening thousands of connections and idling indefinitely and exhausitng your connections.

- **READWRITE_TIMEOUT_SECONDS** is the number of seconds to read a request and to write the response (per each direction, not the total time for both read and write). This prevents consuming a connection with a very slow request/repsonse, intentionally or otherwise.

### Firewall

Gopher is an inherently insecure protocol. GoGopher provides an application-level protocol so that operators can at least control what is allowed to connect to the server and request its documents. These rules are confiured in the file specified by the FIREWALL_CONFIG_FILE environment variable. By default, this is *`firewall-config.cfg`*

Reasonable settings are in the default file: firewall is **enabled**, in **whitelist** mode, and allows connections only from the **local machine and local network**. 

GoGopher will not immediately be open to the internet without the operator explicitly configruing the firewall to allow it.

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

For production, the operatoe will probably want to set it to blacklist mode and add blockedIps as needed. Setting **enabled** to *false* will disable the firewall completely, allowing connections from any client anywhere in the universe.

IP Addresses can be individual addresses, a range in CIDR notation, or wildcard (*) placeholders.

*(Note: An entry with a single * indicates all IP addresses)*

## Gophermap Template




