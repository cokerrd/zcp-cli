# Contributing

Thank you for your interest in zcp-cli.

## Reporting Issues

Please use [GitHub Issues](https://github.com/zsoftly/zcp-cli/issues) to report bugs or request features.

When filing a bug report, include:

- The `zcp` version (`zcp version`)
- Your operating system and architecture
- The exact command you ran
- The expected vs. actual output
- Any relevant error messages (use `--debug` for additional detail)

## Pull Requests

We welcome contributions from the community. Before opening a pull request:

1. Open an issue first to discuss the change.
2. Fork the repository and create a feature branch.
3. Follow the existing code style (run `make fmt` before committing).
4. Add or update tests for any changed behavior.
5. Run `make test-race` to confirm all tests pass.
6. Sign off every commit (`git commit -s`) — see [Developer Certificate of Origin](#developer-certificate-of-origin) below.
7. Open a pull request with a clear description of the change.

## Developer Certificate of Origin

All commits must be signed off, certifying the [Developer Certificate of Origin](https://developercertificate.org/): a statement that you wrote the code, or otherwise have the right to submit it under this project's license.

Sign off by adding the `-s` flag when committing:

```sh
git commit -s -m "fix: describe the change"
```

This appends a `Signed-off-by: Your Name <your@email>` line to the commit message. The name and email must match the commit author.

To sign off commits already on your branch, rebase with sign-off and force-push (replace `main` with your pull request's base branch if different):

```sh
git rebase --signoff origin/main
git push --force-with-lease
```

A DCO check runs on every pull request and must pass before merge.

## Development Setup

See [docs/development.md](docs/development.md) for the full development guide.
