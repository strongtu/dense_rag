## ADDED Requirements

### Requirement: YAML Configuration File
The system SHALL read configuration from `~/.dense_rag/config.yaml` at startup.
If the file does not exist, the system SHALL use default values and optionally
create the file with defaults.

The configuration SHALL support the following keys:
- `host`: HTTP listen address (default `127.0.0.1`)
- `port`: HTTP listen port (default `8123`)
- `topk`: number of search results to return (default `5`)
- `watch_dir`: directory to monitor for file changes (default `~/Documents`)
- `model`: embedding model name (default `text-embedding-bge-m3`)
- `model_endpoint`: embedding service URL (default `http://127.0.0.1:11434`)

The configuration file SHALL use YAML format.

#### Scenario: Config file exists with custom values
- **WHEN** the system starts and `~/.dense_rag/config.yaml` contains `port: 9000`
- **THEN** the HTTP server listens on port 9000

#### Scenario: Config file does not exist
- **WHEN** the system starts and `~/.dense_rag/config.yaml` does not exist
- **THEN** the system uses all default values and starts normally

#### Scenario: Invalid config value
- **WHEN** the system starts and config contains `port: -1`
- **THEN** the system logs a validation error and exits with a non-zero code
