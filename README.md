# Favhash

A command-line tool to find favicons, calculate their MMH3 hash, and perform Shodan searches based on the hash.

![Favhash Banner](screenshots/banner.png)

## Features

- 🔍 Advanced favicon detection methods:
  - HTML `<link>` tags parsing
  - Web app manifest checking
  - Common path detection
  - Multiple format support (ICO, PNG)
- 🔐 Shodan API integration
  - API key validation
  - Plan status checking
  - Search results with detailed output
- 🔄 Comprehensive error handling

## Installation

### Prerequisites

- Go 1.16 or higher

#### Install Go on Ubuntu/Debian
```bash
sudo apt-get update
sudo apt-get install golang-go
```

#### Install Go on CentOS/RHEL
```bash
sudo yum install golang
```
#### Install Go on macOS using Homebrew
```bash
brew install go
```

#### Verify installation
```bash
go version
```

- Shodan API key (for search functionality)

### Building from source

#### Clone the repository
```bash
git clone https://github.com/yourusername/favhash.git
cd favhash
```

#### Install dependencies
```bash
go mod tidy
```

#### Build the binary
```bash
go build
```

#### Optional: Install globally
```
go install
```

## Usage

#### Show help
```bash
favhash -h
```
#### Calculate favicon hash only
```bash
favhash -hash example.com
```
#### Search Shodan with API key
```bash
favhash -k YOUR_SHODAN_KEY example.com
```

### Command Line Options

| Flag    | Description                           |
|---------|---------------------------------------|
| `-k`    | Shodan API key for searching         |
| `-hash` | Only calculate hash without searching |
| `-h`    | Show help message                    |

## Example Output

```
[*] Target URL: https://example.com

[*] Checking Shodan API key status...

[+] API Key Information:
    Plan: free
    Query Credits: 100
    Scan Credits: 0

[+] Found favicon at: https://example.com/favicon.ico
[+] Favicon MMH3 hash: -123456789
[+] Found 42 matches

[+] Results:
    IP: 192.0.2.1
    Port: 443
    Hostnames: host.example.com
    Location: New York, United States
    ---
```

## Features in Detail

### Favicon Detection Methods

1. HTML parsing
   - Searches for `<link rel="shortcut icon">` tags
   - Searches for `<link rel="icon">` tags
   - Supports multiple icon formats

2. Common Paths
   - Checks standard favicon locations
   - Supports both ICO and PNG formats
   - Validates file existence and type

### Shodan Integration

- Validates API key before searching
- Displays plan information and credits
- Shows warnings for free API limitations
- Provides full search results with IP, hostnames, and location

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Shodan](https://www.shodan.io/) for their excellent API
- The Go community for the amazing libraries

## Disclaimer

This tool is for educational purposes only. Make sure you have permission to scan the target systems and comply with Shodan's terms of service.

## Contact

Youssef Boukhriss - [X (Twitter)](https://x.com/0xJosep) - [LinkedIn](https://www.linkedin.com/in/youssefboukhriss/)