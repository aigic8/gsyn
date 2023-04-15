# Gsyn
Gsyn is a CLI application to copy files fast and secure.

## Table of contents
- [Installation](#installation)
  - [prebuilt binaries](#prebuilt-binaries)
  - [building from source](#building-from-source)
- [Configuration](#configuration)
  - [Configuration path](#configuration-path)
  - [Writing a Configuration file](#writing-a-configuration-file)
  - [Generating Certificates](#generating-certificates)
- [Usage](#usage)
  - [Commands](#commands)
  - [Path structure](#path-structure)
  - [Examples](#examples)


## Installation
To install the the gsyn, you can [build the project from source](#building-from-source) or [use prebuilt binaries](#prebuilt-binaries)
### Prebuilt binaries
todo
### Building from source
To install gsyn from source, you should have [Go](https://go.dev/doc/install) and [protoc](https://grpc.io/docs/protoc-installation/) installed first.

Then clone this repository:
```bash
git clone https://github.com/aigic8/gsyn
```

After that, generate protobuf files using:
```bash
cd ./gosyn/api
chmod +x ./generate_pb.sh
./generate_pb.sh
```

Then you can build the project using:
```bash
cd ./gsyn/cmd/gsyn
go build
```

## Configuration
### Configuration path
Gsyn searches for config file in these locations:
- `/etc/gsyn/config.toml`
- `$HOME/.config/gsyn/config.toml`

Also you can use `-c` flag in `serve` command to load configuration from a custom location.

### Writing a Configuration file
Gsyn uses [TOML](https://toml.io/en/) as config language. 
```toml
# Client Part
[client]
defaultTimeout = 5000 # optional, default timeout in milliseconds, default is 5000
defaultWorkers = 10 # optional, default golang workers to be used, default is 10

[client.servers.us]
GUID = "6a480a86-eea5-481d-bbae-5c4417519320" # required, client UUID, should match server
address = "https://1.2.3.4:8686" # required, server address can be IP or hostname
certificates = ["/path/to/cert/cert.der"] #optional, valid certificates for specific server. If empty, system default certificates will be used

# Server part
[server]
address = ":8686" # required, server address
certPath = "/path/to/cert.pem" # required, certificate file
privPath = "/path/to/key.pem" # required, certificate private key
users = [
  { 
    GUID = "6a480a86-eea5-481d-bbae-5c4417519320", # required, should match client UUID
    spaces = ["music"] # required, list of spaces user is authorized to access
  }
]

# server spaces, spaceName = "path/to/space"
[server.spaces]
music = "/home/user/spaces/music"
movies = "/home/user/spaces/movies"
```

Config file consists of two parts `client` and `server`. You only need to write the part you are using. `client` is used when you are using gsyn as client (for example with `cp` command) and `server` is used when you are running on server (for example with `serve` command)

### Generating Certificates
If you are using Gsyn with a valid domain, you can use [CertBot](https://certbot.eff.org/) or [acme.sh](acme.sh) to generate a trusted certificate. You **only need to pass that to your server configuration.**

But if you are using your server IP address, you need to generate a self signed certificate for your ip address and **pass that to both client and server.** (You can technically generate a trusted certificate for an IP address, but I could not find any free ways)
You can generate a self signed certificate using openssl:
``` bash
openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes \  
  -keyout key.pem -out cert.pem -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:1.2.3.4"
```
replace `1.2.3.4` with you server IP address.


Then export a `DER` output for client:
```bash
openssl x509 -outform der -in ./cert.pem -out ./cert.der
```
> Server certificates should be in `PEM` format. But for client, `DER` format should be used.

## Usage

### Commands
- `serve` starts a server
- `cp` copies content. Works like a normal copy command.

### Path structure
In Gsyn, a path has structure `server:space/path/to/file` where
- `server` is server name
- `space` is space name
### Examples
```bash
# copying every mp4 file from musics folder to local computer
gsyn cp server:space/musics/*.mp4 .

# copying and replacing if file exists (forced copy)
gsyn cp -f ./truth.mp4 server:/space/musics

# copying multiple files from multiple servers
gsyn cp server:space/musics/truth.mp4 server2:ss/musics/wish-you-where-here.mp4 .

# copying from local to server
gsyn cp ./truth.mp4 server:space/musics

# copying from one server to another
gsyn cp server:space/musics.truth.mp4 server2:ss/musics
```