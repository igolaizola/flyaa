# flyaa

CLI utility to query American Airlines availability and pricing, combining cash and award searches into a single JSON response.
The tool calls the AA mobile API, with optional proxy support and configurable search parameters.

## Installation Options

### Go install

```
go install github.com/igolaizola/flyaa/cmd/flyaa@latest
```

### Manual install from make output

1. Run `make build` (or `make app-build`).
2. Copy the desired artifact from `bin/` to a directory on your `PATH`, optionally renaming it to simply `flyaa`.

## Running the CLI

At a minimum you must provide the American Airlines API base URL.
All other parameters have defaults.
A typical invocation looks like:

```
flyaa \
  -base-url https://aa-base-url-here/api/ \
  -origin LAX \
  -destination JFK \
  -date 2025-12-15 \
  -passengers 1 \
  -cabin-class main
```

Successful runs print a JSON payload that includes the search metadata and a list of flight options.
When the tool performs both cash and award searches it enriches the output with cents-per-point calculations.

### Flags

Run `flyaa -help` to see all available flags. Key options include:

- `-base-url` (required): AA API base URL.
- `-origin`: 3-letter origin airport code (default `LAX`).
- `-destination`: 3-letter destination airport code (default `JFK`).
- `-date`: travel date in `YYYY-MM-DD` format (default `2025-12-15`).
- `-passengers`: number of travelers (default `1`).
- `-cabin-class`: one of `economy`, `main`, or `main-plus` (default `main`).
- `-proxy`: optional HTTP proxy URL used for outbound requests.
- `-debug`: enable verbose logging from the underlying HTTP client.

The command also includes a `version` subcommand that reports build metadata.

### Environment variables

Every flag can be supplied through an environment variable prefixed with
`FLYAA_`. Examples:

```
export FLYAA_BASE_URL=https://aa-base-url-here/api
export FLYAA_ORIGIN=SFO
export FLYAA_DESTINATION=LHR
flyaa
```

### Configuration file

You can persist settings in a YAML file and load it with `-config`:

```yaml
# config.yaml
base-url: https://aa-base-url-here/api
origin: SEA
destination: DFW
date: 2025-12-15
passengers: 2
cabin-class: main-plus
proxy: http://user:pass@proxy.example.com:8080
```

```
flyaa -config config.yaml
```

Command-line flags override config values, which override environment variables.

## Docker

A Dockerfile is included for convenience.

To build the Docker image, run:

```
PLATFORM=linux/amd64 BASE_URL="https://aa-base-url-here/api" make docker-build
```

To run the container:

```
docker run --rm flyaa:latest \
  -origin LAX \
  -destination JFK \
  -date 2025-12-15 \
  -passengers 1 \
  -cabin-class main \
  -proxy http://user:pass@proxy.example.com:8080
```