const http = require('http');
const querystring = require('querystring');

const demoJwt = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.LRLvirgONK13JgacQ_VbcjySbVhkSmHy3IznH3tA9PM';

const postData = querystring.stringify({
    'topic': 'http://localhost:3000/demo/books/1.jsonld',
    'data': JSON.stringify({ key: 'updated value' }),
});

const req = http.request({
    hostname: 'localhost',
    port: '3000',
    path: '/hub',
    method: 'POST',
    headers: {
        Authorization: `Bearer ${demoJwt}`,
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
