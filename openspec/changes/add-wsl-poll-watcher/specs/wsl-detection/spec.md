## ADDED Requirements

### Requirement: WSL environment detection
The system SHALL detect at startup whether it is running inside a WSL2 environment by reading `/proc/version` and checking for the presence of `microsoft` or `WSL` (case-insensitive).

#### Scenario: Running in WSL2
- **WHEN** `/proc/version` contains the string `microsoft` (case-insensitive)
- **THEN** the system SHALL flag the environment as WSL

#### Scenario: Running on native Linux
- **WHEN** `/proc/version` does not contain `microsoft` or `WSL`
- **THEN** the system SHALL flag the environment as non-WSL

#### Scenario: Detection file unreadable
- **WHEN** `/proc/version` cannot be read
- **THEN** the system SHALL default to non-WSL behavior

### Requirement: Per-directory watcher selection
The system SHALL select the watcher implementation for each configured directory based on the combination of WSL detection and path prefix.

#### Scenario: WSL environment with /mnt/ path
- **WHEN** the environment is WSL AND a watch directory path starts with `/mnt/`
- **THEN** the system SHALL use the poll-based watcher for that directory

#### Scenario: WSL environment with native path
- **WHEN** the environment is WSL AND a watch directory path does NOT start with `/mnt/`
- **THEN** the system SHALL use the fsnotify-based watcher for that directory

#### Scenario: Non-WSL environment
- **WHEN** the environment is not WSL
- **THEN** the system SHALL use the fsnotify-based watcher for all directories regardless of path
