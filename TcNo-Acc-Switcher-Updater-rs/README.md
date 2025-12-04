# TcNo Account Switcher Updater (Rust)

This is a complete rewrite of the TcNo Account Switcher Updater in Rust.

## Features

- Update downloading and installation
- 7z archive extraction
- VCDiff patch application
- File verification with MD5 hashes
- GUI using egui
- Windows-specific features (admin checks, registry access, process management)
- Command-line modes for automation

## Building

```bash
cd TcNo-Acc-Switcher-Updater-rs
cargo build --release
```

## Usage

### GUI Mode
Simply run the executable to launch the GUI updater.

### Command-Line Modes

- `--downloadcef`: Download CEF files
- `--verify`: Verify files and exit
- `--hashlist`: Generate hash list
- `--createupdate`: Create update package

## Dependencies

- **eframe/egui**: GUI framework
- **reqwest**: HTTP client
- **sevenz-rust**: 7z archive extraction
- **serde/serde_json/serde_yaml**: Serialization
- **md5**: Hashing
- **windows**: Windows API bindings
- **clap**: Command-line parsing

## Notes

- VCDiff implementation is currently a placeholder and needs to be replaced with actual VCDiff encoding/decoding
- Some Windows-specific features require running as administrator
- The updater checks for single instance to prevent multiple instances





