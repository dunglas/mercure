require 'json'
require 'net/http'

token = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJmb28iLCJiYXIiXSwicHVibGlzaCI6WyJmb28iXX19.afLx2f2ut3YgNVFStCx95Zm_UND1mZJ69OenXaDuZL8'

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
