## ADDED Requirements

### Requirement: Multi-directory watch configuration
The system SHALL accept a `watch_dirs` configuration field as an ordered list of directory paths to monitor.

#### Scenario: New array format
- **WHEN** the config file contains `watch_dirs: ["~/Documents", "/mnt/c/Users/strongtu/Documents"]`
- **THEN** the system SHALL monitor both directories independently

#### Scenario: Legacy single-directory format
- **WHEN** the config file contains `watch_dir: "~/Documents"` (string) and `watch_dirs` is absent
- **THEN** the system SHALL treat it as a single-element list `["~/Documents"]`

#### Scenario: Both fields present
- **WHEN** the config file contains both `watch_dir` and `watch_dirs`
- **THEN** the system SHALL use `watch_dirs` and ignore `watch_dir`

### Requirement: Directory overlap detection
The system SHALL reject configurations where one watch directory is an ancestor (parent) of another.

#### Scenario: Nested directory rejected
- **WHEN** the config contains `watch_dirs: ["~/Documents", "~/Documents/notes"]`
- **THEN** the system SHALL return a validation error at startup indicating the overlap

#### Scenario: Independent directories accepted
- **WHEN** the config contains `watch_dirs: ["/mnt/c/docs", "~/Documents"]`
- **THEN** the system SHALL accept the configuration

### Requirement: Tilde and path normalization
The system SHALL expand `~` to the user's home directory and normalize all watch directory paths (resolve `.`, `..`, trailing slashes) before overlap detection and watcher assignment.

#### Scenario: Tilde expansion in array
- **WHEN** the config contains `watch_dirs: ["~/Documents", "~/Notes"]`
- **THEN** both paths SHALL be expanded to absolute paths using the current user's home directory
