# Contributing

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

## Protocol

The protocol is written in Markdown, compatible with [Mmark](https://mmark.nl/).
It is then converted in the [the "xml2rfc" Version 3 Vocabulary](https://tools.ietf.org/html/rfc7991).

To contribute to the protocol itself:

* Make your changes
* [Download Mmark](https://github.com/mmarkdown/mmark/releases)
* [Download `xml2rfc` using pip](https://pypi.org/project/xml2rfc/): `pip install xml2rfc`
* Format the Markdown file: `mmark -markdown -w spec/mercure.md`
* Generate the XML file: `mmark spec/mercure.md > spec/mercure.xml`
* Validate the generated XML file and generate the text file: `xml2rfc --text --v3 spec/mercure.xml`
* If appropriate, be sure to update the reference implementation accordingly
