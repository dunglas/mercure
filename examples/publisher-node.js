const http = require("http");
const querystring = require("querystring");

const demoJwt =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwiaHR0cDovL2xvY2FsaG9zdDozMDAwL2RlbW8vYm9va3Mve2lkfS5qc29ubGQiXSwicGF5bG9hZCI6eyJ1c2VyIjoiaHR0cHM6Ly9leGFtcGxlLmNvbS91c2Vycy9kdW5nbGFzIiwicmVtb3RlX2FkZHIiOiIxMjcuMC4wLjEifX19.bRUavgS2H9GyCHq7eoPUL_rZm2L7fGujtyyzUhiOsnw";

const postData = querystring.stringify({
  topic: "http://localhost:3000/demo/books/1.jsonld",
  data: JSON.stringify({ key: "updated value" }),
});

const req = http.request(
  {
    hostname: "localhost",
    port: "3000",
    path: "/.well-known/mercure",
    method: "POST",
    headers: {
      Authorization: `Bearer ${demoJwt}`,
      "Content-Type": "application/x-www-form-urlencoded",
      "Content-Length": Buffer.byteLength(postData),
    },
  },
  (res) => {
    console.log(`Status: ${res.statusCode}`);
    console.log(`Headers: ${JSON.stringify(res.headers)}`);
  }
);

req.on("error", (e) => {
  console.error(`An error occurred: ${e.message}`);
});

req.write(postData);
req.end();
