const http = require('http');
const querystring = require('querystring');

const postData = querystring.stringify({
  'topic': 'http://localhost:3000/demo/books/1.jsonld',
  'data': JSON.stringify({ key: 'updated value' }),
});

const token = process.env.PUBLISHER_JWT_TOKEN || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.HB0k08BaV8KlLZ3EafCRlTDGbkd9qdznCzJQ_l8ELTU'

const req = http.request({
    hostname: 'localhost',
    path: '/publish',
    method: 'POST',
    headers: {
      Authorization: 'Bearer ' + token,
        'Content-Type': 'application/x-www-form-urlencoded',
        'Content-Length': Buffer.byteLength(postData),
    }
}, (res) => {
    console.log(`Status: ${res.statusCode}`);
    console.log(`Headers: ${JSON.stringify(res.headers)}`);
});

req.on('error', (e) => {
    console.error(`An error occured: ${e.message}`);
});

req.write(postData);
req.end();
