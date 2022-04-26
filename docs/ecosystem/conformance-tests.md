# Conformance Tests

We provide conformance tests to check if a hub is compliant with the Mercure specification.
This test suite is based on [Playwright](https://playwright.dev/).

## Install

1. Clone the repository: `git clone https://github.com/dunglas/mercure`
2. Go in the conformance tests directory: `cd conformance-tests`
3. Install the dependencies: `npm ci`
4. Install Playwright: `npx playwright install --with-deps`
5. Run the test suite: `npx playwright test`

## Configuration

The test suite can be configured by setting environment variables:

* `BASE_URL`: the URL of the hub to test
* `CUSTOM_ID`: enable or disable tests related to custom IDs support

## See Also

* [The load test](../hub/load-test.md)
