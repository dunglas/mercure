# Contributing

## License and Copyright Attribution

When you open a Pull Request to the project, you agree to license your code under the [GNU AFFERO GENERAL PUBLIC LICENSE](LICENSE)
and to transfer the copyright on the submitted code to [Kévin Dunglas](https://dunglas.fr).

Be sure to have the right to do that (if you are a professional, ask your company)!

If you include code from another project, please mention it in the Pull Request description and credit the original author.

## Commit Messages

The commit message must follow the [Conventional Commits specification](https://www.conventionalcommits.org/).
The following types are allowed:

* `fix`: bugfix
* `feat`: new feature
* `docs`: change in the documentation
* `spec`: spec change
* `test`: test-related change
* `perf`: performance optimization
* `ci`: CI-related change

Examples:

    fix: Fix something

    feat: Introduce X

    feat!: Introduce Y, BC break

    docs: Add docs for X

    spec: Z disambiguation

## Hub

Clone the project and make your changes:

    git clone https://github.com/dunglas/mercure
    cd mercure

To run the test suite:

    go test -v -timeout 30s github.com/dunglas/mercure

To test the Caddy module:

    cd caddy/mercure
    MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' go run main.go run --config ../../dev.Caddyfile

Go to `https://localhost` and enjoy!

To test the legacy server:

    cd cmd/mercure
    go run main.go

Go to `http://localhost:3000` and enjoy!

When you send a PR, make sure that:

* You add valid test cases.
* Tests are green.
* You make a PR on the related documentation.
* You make the PR on the same branch you based your changes on. If you see commits
  that you did not make in your PR, you're doing it wrong.

### Configuring Visual Studio Code

A configuration for VSCode is provided in the `.vscode/` directory of the repository.
It is automatically loaded by VS Code.

### Finding Deadlocks

To debug potential deadlocks:

1. Install `go-deadlock`: `./tests/use-go-deadlock.sh`
2. Run the tests in race mode: `go test -race ./... -v`
3. To stress-test the app, run the load test (see `docs/load-testing.md`)
4. Be sure to remove `go-deadlock` before committing

## Spec

The spec is written in Markdown, compatible with [Mmark](https://mmark.miek.nl/).
It is then converted in the [the "xml2rfc" Version 3 Vocabulary](https://tools.ietf.org/html/rfc7991).

To contribute to the protocol itself:

* Make your changes
* [Download Mmark](https://github.com/mmarkdown/mmark/releases)
* [Download `xml2rfc` using pip](https://pypi.org/project/xml2rfc/): `pip install xml2rfc`
* Generate the XML file: `mmark spec/mercure.md > spec/mercure.xml`
* Validate the generated XML file and generate the text file: `xml2rfc --text --v3 spec/mercure.xml`
* Remove non-ASCII characters from the generated `mercure.txt` file (example: K**é**vin)
* If appropriate, be sure to update the reference implementation accordingly
