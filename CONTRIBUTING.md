# Contributing to GoPotency

First off, thank you for considering contributing to GoPotency! It's people like you that make GoPotency such a great tool.

## Code of Conduct

By participating in this project, you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md).

## How Can I Contribute?

### Reporting Bugs

- Use the GitHub issue tracker to report bugs.
- Describe the bug in detail and provide a minimal reproducible example if possible.
- Include your Go version and OS.

### Feature Requests

- Open an issue to discuss the feature before starting implementation.
- Explain why the feature is needed and how it would benefit the project.

### Pull Requests

1.  Fork the repository and create your branch from `main`.
2.  If you've added code that should be tested, add tests.
3.  If you've changed APIs, update the documentation.
4.  Ensure the test suite passes (`make test`).
5.  Make sure your code lints (`golangci-lint run`).

## Development Setup

We use a `Makefile` to simplify common tasks:

- `make test`: Runs all tests.
- `make bench`: Runs benchmarks.
- `make build`: Builds all examples.

## Style Guide

- Follow standard Go idiomatic patterns.
- Use `gofmt` to format your code.
- Write clear, documented code (standard Go doc comments).

## License

By contributing, you agree that your contributions will be licensed under its MIT License.
