require 'json'
require 'net/http'

token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InB1Ymxpc2giOlsiKiJdLCJzdWJzY3JpYmUiOlsiaHR0cHM6Ly9leGFtcGxlLmNvbS9teS1wcml2YXRlLXRvcGljIiwiaHR0cDovL2xvY2FsaG9zdDozMDAwL2RlbW8vYm9va3Mve2lkfS5qc29ubGQiXSwicGF5bG9hZCI6eyJ1c2VyIjoiaHR0cHM6Ly9leGFtcGxlLmNvbS91c2Vycy9kdW5nbGFzIiwicmVtb3RlX2FkZHIiOiIxMjcuMC4wLjEifX19.bRUavgS2H9GyCHq7eoPUL_rZm2L7fGujtyyzUhiOsnw'

Net::HTTP.start('localhost', 3000) do |http|
  req = Net::HTTP::Post.new('/.well-known/mercure')
  req['Authorization'] = "Bearer #{token}"
  req.form_data = {
    topic: 'http://localhost:3000/demo/books/1.jsonld',
    data: { key: :value }.to_json
  }
  req = http.request(req)
  puts req.body
end
