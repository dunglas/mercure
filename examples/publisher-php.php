<?php

define('DEMO_JWT', 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwiaHR0cDovL2xvY2FsaG9zdDozMDAwL2RlbW8vYm9va3Mve2lkfS5qc29ubGQiXSwicGF5bG9hZCI6eyJ1c2VyIjoiaHR0cHM6Ly9leGFtcGxlLmNvbS91c2Vycy9kdW5nbGFzIiwicmVtb3RlX2FkZHIiOiIxMjcuMC4wLjEifX19.bRUavgS2H9GyCHq7eoPUL_rZm2L7fGujtyyzUhiOsnw');

$postData = http_build_query([
    'topic' => 'http://localhost:3000/demo/books/1.jsonld',
    'data' => json_encode(['key' => 'updated value']),
]);

echo file_get_contents('http://localhost:3000/.well-known/mercure', false, stream_context_create(['http' => [
    'method'  => 'POST',
    'header'  => "Content-type: application/x-www-form-urlencoded\r\nAuthorization: Bearer ".DEMO_JWT,
    'content' => $postData,
]]));
