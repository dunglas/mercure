const http = require('http')
const querystring = require('querystring')

const demoJwt =
  'eyJhbGciOiJIUzI1NiJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwie3NjaGVtZX06Ly97K2hvc3R9L2RlbW8vYm9va3Mve2lkfS5qc29ubGQiLCIvLndlbGwta25vd24vbWVyY3VyZS9zdWJzY3JpcHRpb25zey90b3BpY317L3N1YnNjcmliZXJ9Il0sInBheWxvYWQiOnsidXNlciI6Imh0dHBzOi8vZXhhbXBsZS5jb20vdXNlcnMvZHVuZ2xhcyIsInJlbW90ZUFkZHIiOiIxMjcuMC4wLjEifX19.KKPIikwUzRuB3DTpVw6ajzwSChwFw5omBMmMcWKiDcM'

const postData = querystring.stringify({
  topic: 'https://localhost/demo/books/1.jsonld',
  data: JSON.stringify({ key: 'updated value' })
})

const req = http.request(
  {
    hostname: 'localhost',
    port: '3000',
    path: '/.well-known/mercure',
    method: 'POST',
    headers: {
      Authorization: `Bearer ${demoJwt}`,
      'Content-Type': 'application/x-www-form-urlencoded',
      'Content-Length': Buffer.byteLength(postData)
    }
  },
  (res) => {
    console.log(`Status: ${res.statusCode}`)
    console.log(`Headers: ${JSON.stringify(res.headers)}`)
  }
)

req.on('error', (e) => {
  console.error(`An error occurred: ${e.message}`)
})

req.write(postData)
req.end()
