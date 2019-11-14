# Contributing

## License and Copyright Attribution

When you open a Pull Request to the project, you agree to license your code under the [GNU AFFERO GENERAL PUBLIC LICENSE](LICENSE)
and to transfer the copyright on the submitted code to [Kévin Dunglas](https://dunglas.fr).

Be sure to you have the right to do that (if you are a professional, ask your company)!

If you include code from another project, please mention it in the Pull Request description and credit the original author.

## Hub

Clone the project:

    $ git clone https://github.com/dunglas/mercure
    
Install Gin for Live Reloading:

    $ go get github.com/codegangsta/gin

Install the dependencies:

    $ cd mercure
    $ go get

Run the server:

    $ gin run main.go

Go to `http://localhost:3000` and enjoy!

To run the test suite:

    $ go test -v -timeout 30s github.com/dunglas/mercure/hub

When you send a PR, just make sure that:

* You add valid test cases.
* Tests are green.
* You make a PR on the related documentation.
* You make the PR on the same branch you based your changes on. If you see commits
  that you did not make in your PR, you're doing it wrong.

## Protocol

The protocol is written in Markdown, compatible with [Mmark](https://mmark.miek.nl/).
It is then converted in the [the "xml2rfc" Version 3 Vocabulary](https://tools.ietf.org/html/rfc7991).

To contribute to the protocol itself:

* Make your changes
* [Download Mmark](https://github.com/mmarkdown/mmark/releases)
* [Download `xml2rfc` using pip](https://pypi.org/project/xml2rfc/): `pip install xml2rfc`
* Format the Markdown file: `mmark -markdown -w spec/mercure.md`
* Generate the XML file: `mmark spec/mercure.md > spec/mercure.xml`
* Validate the generated XML file and generate the text file: `xml2rfc --text --v3 spec/mercure.xml`
* Remove non-ASCII characters from the generated `mercure.txt` file (example: K**é**vin)
* If appropriate, be sure to update the reference implementation accordingly
