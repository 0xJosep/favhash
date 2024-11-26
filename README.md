# Favhash

A command-line tool to find favicons, calculate their MMH3 hash, and perform Shodan searches based on the hash.

## Features

- 🔍 Advanced favicon detection methods:
  - HTML `<link>` tags parsing
  - Web app manifest checking
  - Common path detection
  - Multiple format support (ICO, PNG, GIF, SVG)
- 🔐 Shodan Integration (requires API key):
  - API key validation
  - Plan status checking
  - Search results with detailed output
- 🛠️ Enhanced Functionality:
  - Custom User-Agent support
  - Proxy support
  - Configurable retries and timeouts
  - Multiple output formats (text, JSON, YAML)
  - Debug mode for detailed logging
  - History tracking
  - Result saving

## Installation

### Prerequisites

- Go 1.16 or higher
  ```bash
  # Install Go on Ubuntu/Debian
  sudo apt-get update
  sudo apt-get install golang-go

  # Install Go on CentOS/RHEL
  sudo yum install golang

  # Install Go on macOS using Homebrew
  brew install go

  # Verify installation
  go version
  ```

### Building from source

```bash
# Clone the repository
git clone https://github.com/0xJosep/favhash.git
cd favhash

# Install dependencies
go mod init favhash
go get github.com/PuerkitoBio/goquery
go get github.com/fatih/color
go get github.com/spaolacci/murmur3
go get gopkg.in/yaml.v3

# Build the binary
go build
```

## Usage

### Basic Usage (No API Key Required)

```bash
# Calculate favicon hash only
favhash -hash example.com

# With debug mode
favhash -hash -debug facebook.com

# With custom User-Agent
favhash -hash -ua "MyCustomUserAgent/1.0" facebook.com

# With retry attempts
favhash -hash -r 5 facebook.com
```

### Advanced Usage (Requires Shodan API Key)

```bash
# Full search with Shodan
favhash -k YOUR_SHODAN_KEY facebook.com

# JSON output
favhash -k YOUR_SHODAN_KEY -o json facebook.com

# Save results to file
favhash -k YOUR_SHODAN_KEY -save facebook.com

# With proxy
favhash -k YOUR_SHODAN_KEY -proxy http://127.0.0.1:8080 facebook.com
```

### Command Line Options

| Flag           | Description                                    | API Key Required |
|----------------|------------------------------------------------|-----------------|
| `-hash`        | Only calculate hash without Shodan search       | No             |
| `-debug`       | Enable debug output                            | No             |
| `-k`           | Shodan API key                                 | Yes            |
| `-o`           | Output format (text, json, yaml)               | Yes            |
| `-t`           | Timeout for requests (e.g., 15s)               | No             |
| `-r`           | Number of retries for failed requests          | No             |
| `-delay`       | Delay between retries                          | No             |
| `-proxy`       | Proxy URL                                      | No             |
| `-ua`          | Custom User-Agent string                       | No             |
| `-save`        | Save results to file                           | Yes            |
| `-no-redirect` | Disable following redirects                    | No             |
| `-no-history`  | Disable search history                         | No             |

## Example Output

### Hash Only Mode
```
[*] Target URL: https://facebook.com
[*] Found favicon at: https://static.xx.fbcdn.net/rsrc.php/yB/r/2sFJRNmJ5OP.ico
[+] Favicon MMH3 hash: 872991029
```

### Full Search Mode (with API key)
```
[*] Target URL: https://facebook.com
[*] Checking Shodan API key status...
[+] API Key Information:
    Plan: dev
    Query Credits: 100
    Scan Credits: 0
[+] Found favicon at: https://static.xx.fbcdn.net/rsrc.php/yB/r/2sFJRNmJ5OP.ico
[+] Favicon MMH3 hash: 872991029
[+] Found X matches
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Shodan](https://www.shodan.io/) for their API
- The Go community for the amazing libraries

## Disclaimer

This tool is for educational purposes only. Make sure you have permission to scan the target systems and comply with Shodan's terms of service.

## Contact

Youssef Boukhriss - [X (Twitter)](https://x.com/0xJosep) - [LinkedIn](https://www.linkedin.com/in/youssefboukhriss/)