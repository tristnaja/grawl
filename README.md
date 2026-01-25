# grawl 🤖

[![Go Report Card](https://goreportcard.com/badge/github.com/tristnaja/grawl)](https://goreportcard.com/report/github.com/tristnaja/grawl)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Latest Release](https://img.shields.io/github/v/release/tristnaja/grawl)](https://github.com/tristnaja/grawl/releases/latest)

`grawl` is a lightweight, concurrent, and polite web crawler written in Go. It's designed to be efficient while respecting the rules and load of the websites it crawls. It can be run via command-line flags or a simple interactive prompt.

## ✨ Features

- **Concurrent Crawling**: Utilizes a worker pool model to fetch multiple pages concurrently.
- **Polite Crawling**:
    - Obeys `robots.txt` rules, including `Allow`, `Disallow`, and `Crawl-Delay` directives.
    - Uses a sensible default delay and a custom User-Agent (`Grawl/1.0`).
- **Configurable**: Easily configure the number of workers, crawl depth, and buffer capacity.
- **Interactive CLI**: Can be run with flags or through an interactive prompt to guide you through setup.
- **Duplicate Prevention**: Tracks visited URLs to avoid redundant work and getting stuck in loops.
- **Graceful Shutdown**: Employs Go's context package for clean and graceful shutdowns.
- **Robust & Performant**: Built with a custom HTTP client with fine-tuned timeouts and connection pooling.

## 🏗️ Architecture

`grawl` uses a modular, concurrent architecture where components communicate through channels.

- **`main`**: The entry point, handles CLI flags and interactive user configuration.
- **`Orchestrator`**: Initializes and wires together all the different components.
- **`Manager`**: The central coordinator. It manages the queue of URLs to be crawled, dispatches jobs to workers, and processes their results.
- **`Worker`**: The workhorse. It receives a URL, fetches the page, and extracts new links.
- **`Scheduler`**: The crawler's memory. It keeps track of every URL visited to prevent duplicates.
- **`Robots`**: The crawler's conscience. It fetches, parses, and caches `robots.txt` files to ensure all crawling is compliant and polite.
- **`Client`**: A fine-tuned HTTP client responsible for all network requests.

## 🚀 Installation

There are multiple ways to install `grawl`.

### Homebrew (macOS & Linux)

This is the recommended method for macOS and Linux users.

```sh
brew tap tristnaja/tap/
```
```sh
brew install grawl
```

### From Release

You can download the pre-compiled binary for your operating system from the [latest GitHub release](https://github.com/tristnaja/grawl/releases/latest). Unpack the archive and place the `grawl` binary in your `PATH`.

### From Source

If you have Go installed (version 1.22 or newer is recommended), you can install `grawl` directly from the source.

```sh
go install github.com/tristnaja/grawl@latest
```

## Usage

You can run `grawl` in several ways.

#### Provide URL via Flag

```sh
grawl --url https://example.com
# or with the shorthand
grawl -u https://example.com
```

#### Provide URL as an Argument

```sh
grawl https://example.com
```

#### Interactive Mode

Simply run `grawl` without any arguments to enter the interactive setup.

```sh
grawl
```

The application will then prompt you to enter the starting URL and guide you through configuring the crawl parameters.

```
Enter your starting URL: https://example.com

This is the default configuration:
Capacity: 100
Number of Worker(s): 10
Crawl Depth: 2
Do you want the default config for your worker (y/n)? y

Starting Crawl with:
Capacity: 100
Number of Worker(s): 10
Crawl Depth: 2
```

### Options

- `-u, --url`: Specify the starting URL to crawl.
- `-v, --version`: Print the current version of `grawl`.

## ❤️ Contributing

Contributions are welcome! If you'd like to help improve `grawl`, please feel free to open a pull request or issue.

## 📄 License

`grawl` is licensed under the **MIT License**. See the [LICENSE](./LICENSE) file for details.
