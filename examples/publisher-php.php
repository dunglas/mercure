<?php

define('DEMO_JWT', 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.LRLvirgONK13JgacQ_VbcjySbVhkSmHy3IznH3tA9PM');

$postData = http_build_query([
    'topic' => 'http://localhost:3000/demo/books/1.jsonld',
    'data' => json_encode(['key' => 'updated value']),
]);

echo file_get_contents('http://localhost:3000/hub', false, stream_context_create(['http' => [
    'method'  => 'POST',
    'header'  => "Content-type: application/x-www-form-urlencoded\r\nAuthorization: Bearer ".DEMO_JWT,
    'content' => $postData,
]]));
