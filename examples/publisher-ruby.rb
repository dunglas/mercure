require 'json'
require 'net/http'

token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.LRLvirgONK13JgacQ_VbcjySbVhkSmHy3IznH3tA9PM'

Net::HTTP.start('localhost', 3000) do |http|
  req = Net::HTTP::Post.new('/hub')
  req['Authorization'] = "Bearer #{token}"
  req.form_data = {
    topic: 'http://localhost:3000/demo/books/1.jsonld',
    data: { key: :value }.to_json
  }
  req = http.request(req)
  puts req.body
end
