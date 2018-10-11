const http = require('http');
const querystring = require('querystring');

const postData = querystring.stringify({
    'topic': 'https://example.com/books/1.jsonld',
    'data': JSON.stringify({ key: 'updated value' }),
});

const req = http.request({
    hostname: 'localhost',
    path: '/publish',
    method: 'POST',
    headers: {
      Authorization: 'Bearer ' + process.env.PUBLISHER_JWT_TOKEN,
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
