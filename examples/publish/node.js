const http = require("http");
const querystring = require("querystring");

const demoJwt =
  "eyJhbGciOiJIUzI1NiIsInR5cCI6ImF0K2p3dCJ9.eyJhdWQiOiJodHRwczovL2xvY2FsaG9zdC8ud2VsbC1rbm93bi9tZXJjdXJlIiwiYXV0aG9yaXphdGlvbl9kZXRhaWxzIjpbeyJhY3Rpb25zIjpbInB1Ymxpc2giXSwidG9waWNzIjpbeyJtYXRjaCI6IioifV0sInR5cGUiOiJodHRwczovL21lcmN1cmUucm9ja3MvYXV0aG9yaXphdGlvbi1kZXRhaWwifSx7ImFjdGlvbnMiOlsic3Vic2NyaWJlIl0sInRvcGljcyI6W3sibWF0Y2giOiIqIn1dLCJ0eXBlIjoiaHR0cHM6Ly9tZXJjdXJlLnJvY2tzL2F1dGhvcml6YXRpb24tZGV0YWlsIn1dLCJleHAiOjQxMDI0NDQ4MDAsImlzcyI6Imh0dHBzOi8vbG9jYWxob3N0In0.VO0-PRjJ2MGOrMk2HxlrBv217pB7hyLxLIQUGgSfyXs";

const postData = querystring.stringify({
  topic: "https://localhost/demo/books/1.jsonld",
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
  },
);

req.on("error", (e) => {
  console.error(`An error occurred: ${e.message}`);
});

req.write(postData);
req.end();
