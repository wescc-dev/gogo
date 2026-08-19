local server = server_info()
print("This is an example of lua script sending dynamic content to the client")
print("TotalConnections")
print(info.TotalConnections)
print(server.title)
print(server.host_name)
print(server.port)
print(server.tls_enabled)
print(info.GopherRoot)
local filename = info.GopherRoot .. "/$textfile.txt"
local content, err = read_file(filename)
assert(content, err)

print(content)